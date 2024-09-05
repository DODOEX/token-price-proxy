package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database"
	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

const (
	logQueueKey          = "logs:queue"
	logQueueRunSize      = 5000
	logMaxRetries        = 3
	logRetryDelay        = 2 * time.Second
	logLockTTL           = 15 * time.Second
	logLockRetryInterval = 1 * time.Second
	logLockRetryCount    = 3
)

type RequestLogRepository interface {
	InsertLog(ctx context.Context, log schema.RequestLog) error
	InsertLogs(tx *gorm.DB, logs []schema.RequestLog) error
	ProcessQueue() error
	Stop()
}

type requestLogRepository struct {
	db          *database.Database
	redisClient *shared.RedisClient
	logger      zerolog.Logger
	taskQueue   chan func() error
	quit        chan bool
}

func NewRequestLogRepository(lc fx.Lifecycle, db *database.Database, redisClient *shared.RedisClient, logger zerolog.Logger) RequestLogRepository {
	repo := &requestLogRepository{
		db:          db,
		redisClient: redisClient,
		logger:      logger,
		taskQueue:   make(chan func() error, 1000), // 任务队列
		quit:        make(chan bool),
	}
	workerCount := 8
	lc.Append(fx.Hook{
		OnStart: func(context.Context) error {
			for i := 0; i < workerCount; i++ {
				go repo.worker()
			}
			return nil
		},
		OnStop: func(context.Context) error {
			close(repo.quit)
			return nil
		},
	})

	return repo
}
func (r *requestLogRepository) Stop() {
	close(r.quit)
}
func (r *requestLogRepository) worker() {
	for {
		select {
		case task := <-r.taskQueue:
			if err := task(); err != nil {
				r.logger.Error().Err(err).Msg("Failed to execute task")
			}
		case <-r.quit:
			return
		}
	}
}

func (r *requestLogRepository) InsertLog(ctx context.Context, log schema.RequestLog) error {
	task := func() error {
		data, err := json.Marshal(log)
		if err != nil {
			return err
		}

		if err := r.redisClient.Client.RPush(ctx, logQueueKey, data).Err(); err != nil {
			return err
		}

		// queueLen, err := r.redisClient.Client.LLen(ctx, logQueueKey).Result()
		// if err != nil {
		// 	return err
		// }

		// if queueLen >= int64(logQueueRunSize) {
		// 	return r.ProcessQueue()
		// }

		return nil
	}

	select {
	case r.taskQueue <- task:
		return nil
	default:
		r.logger.Debug().Msg("InsertLog task queue is full")
		return nil
	}
}

func (r *requestLogRepository) InsertLogs(tx *gorm.DB, logs []schema.RequestLog) error {
	return tx.Create(logs).Error
}

func (r *requestLogRepository) ProcessQueue() error {
	lockKey := "lock:logs_queue"

	for attempt := 1; attempt <= logLockRetryCount; attempt++ {
		ok := r.redisClient.AcquireLock(lockKey, logLockTTL)
		if ok {
			defer r.redisClient.ReleaseLock(lockKey)
			return r.processQueueWithTransaction()
		}
		time.Sleep(logLockRetryInterval)
	}

	return fmt.Errorf("failed to acquire lock after %d attempts", logLockRetryCount)
}

func (r *requestLogRepository) processQueueWithTransaction() error {
	logsData, err := r.redisClient.Client.LRange(context.Background(), logQueueKey, 0, -1).Result()
	if err != nil {
		return err
	}
	r.logger.Debug().Msgf("request_logs processQueueWithTransaction len: %d", len(logsData))

	if len(logsData) == 0 {
		return nil
	}
	defer func() {
		r.redisClient.Client.Del(context.Background(), logQueueKey).Err()
	}()
	var logs []schema.RequestLog
	for _, logData := range logsData {
		var log schema.RequestLog
		if err := json.Unmarshal([]byte(logData), &log); err != nil {
			return err
		}
		logs = append(logs, log)
	}

	for attempt := 1; attempt <= logMaxRetries; attempt++ {
		err = r.insertLogsTransaction(logs)
		if err == nil {
			break
		}
		if err != nil && err.Error() == "ERROR: deadlock detected (SQLSTATE 40P01)" {
			r.logger.Error().Err(err).Msgf("Deadlock detected, retrying (%d/%d)", attempt, logMaxRetries)
			time.Sleep(logRetryDelay)
			continue
		}
		r.logger.Error().Err(err).Msg("Failed to process queue")
		return err
	}

	return nil
}

func (r *requestLogRepository) insertLogsTransaction(logs []schema.RequestLog) error {
	batchSize := 1000
	r.logger.Debug().Msgf("request_logs insertLogsTransaction len: %d", len(logs))

	for i := 0; i < len(logs); i += batchSize {
		end := i + batchSize
		if end > len(logs) {
			end = len(logs)
		}
		batch := logs[i:end]

		tx := r.db.DB.Begin()
		err := tx.Create(&batch).Error
		if err != nil {
			tx.Rollback()
			r.logger.Error().Err(err).Msg("Failed to insert logs")
			return err
		}
		if err := tx.Commit().Error; err != nil {
			r.logger.Error().Err(err).Msg("Failed to commit transaction")
			return err
		}
	}
	return nil
}
