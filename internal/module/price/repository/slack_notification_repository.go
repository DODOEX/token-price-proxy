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
	"gorm.io/gorm/clause"
)

const (
	slackQueueKey          = "slack_notifications:queue"
	slackMaxRetries        = 3
	slackRetryDelay        = 2 * time.Second
	slackLockTTL           = 15 * time.Second
	slackLockRetryInterval = 3 * time.Second
	slackLockRetryCount    = 3
	exeBatchSize           = 1000
)

type SlackNotificationRepository interface {
	InsertNotification(ctx context.Context, notification schema.SlackNotifications) error
	ProcessQueue() error
	ProcessTopNotifications() error
	Stop()
	DeleteOldData() error
}

type slackNotificationRepository struct {
	db          *database.Database
	redisClient *shared.RedisClient
	logger      zerolog.Logger
	taskQueue   chan func() error
	quit        chan bool
}

func NewSlackNotificationRepository(lc fx.Lifecycle, db *database.Database, redisClient *shared.RedisClient, logger zerolog.Logger) SlackNotificationRepository {
	repo := &slackNotificationRepository{
		db:          db,
		redisClient: redisClient,
		logger:      logger,
		taskQueue:   make(chan func() error, 1000),
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

func (r *slackNotificationRepository) worker() {
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

func (r *slackNotificationRepository) Stop() {
	close(r.quit)
}

func (r *slackNotificationRepository) InsertNotification(ctx context.Context, notification schema.SlackNotifications) error {
	task := func() error {
		data, err := json.Marshal(notification)
		if err != nil {
			return err
		}

		if err := r.redisClient.Client.RPush(ctx, slackQueueKey, data).Err(); err != nil {
			return err
		}

		return nil
	}

	select {
	case r.taskQueue <- task:
		return nil
	default:
		r.logger.Debug().Msgf("InsertNotification task queue is full")
		return nil
	}
}

func (r *slackNotificationRepository) ProcessQueue() error {
	lockKey := "lock:slack_notifications_queue"

	for attempt := 1; attempt <= slackLockRetryCount; attempt++ {
		ok := r.redisClient.AcquireLock(lockKey, slackLockTTL)
		if ok {
			defer r.redisClient.ReleaseLock(lockKey)
			return r.processQueueWithTransaction()
		}
		time.Sleep(slackLockRetryInterval)
	}

	return nil
}

func (r *slackNotificationRepository) processQueueWithTransaction() error {
	notificationsData, err := r.redisClient.Client.LRange(context.Background(), slackQueueKey, 0, -1).Result()
	if err != nil {
		return err
	}
	r.logger.Debug().Msgf("slack_notifications processQueueWithTransaction len: %d", len(notificationsData))

	if len(notificationsData) == 0 {
		return nil
	}
	defer r.redisClient.Client.Del(context.Background(), slackQueueKey).Err()

	uniqueNotifications := make(map[string]schema.SlackNotifications)
	for _, notificationData := range notificationsData {
		var notification schema.SlackNotifications
		if err := json.Unmarshal([]byte(notificationData), &notification); err != nil {
			return err
		}
		key := fmt.Sprintf("%s_%s", notification.CoinID, notification.DayDate)
		if existing, found := uniqueNotifications[key]; found {
			notification.Counter += existing.Counter
		}
		uniqueNotifications[key] = notification
	}

	var notifications []schema.SlackNotifications
	for _, notification := range uniqueNotifications {
		notifications = append(notifications, notification)
	}

	for attempt := 1; attempt <= slackMaxRetries; attempt++ {
		err = r.insertNotificationsTransaction(notifications)
		if err == nil {
			break
		}
		if err != nil && err.Error() == "ERROR: deadlock detected (SQLSTATE 40P01)" {
			r.logger.Error().Err(err).Msgf("Deadlock detected, retrying (%d/%d)", attempt, slackMaxRetries)
			time.Sleep(slackRetryDelay)
			continue
		}
		r.logger.Error().Err(err).Msg("Failed to process queue")
		return err
	}

	return nil
}

func (r *slackNotificationRepository) insertNotificationsTransaction(notifications []schema.SlackNotifications) error {
	batchSize := 1000
	r.logger.Debug().Msgf("slack_notifications insertNotificationsTransaction len: %d", len(notifications))

	for i := 0; i < len(notifications); i += batchSize {
		end := i + batchSize
		if end > len(notifications) {
			end = len(notifications)
		}
		batch := notifications[i:end]

		tx := r.db.DB.Begin()
		err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "coin_id"}, {Name: "day_date"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"counter":    gorm.Expr("CASE WHEN slack_notifications.deleted_at IS NOT NULL THEN EXCLUDED.counter ELSE slack_notifications.counter + EXCLUDED.counter END"),
				"deleted_at": gorm.Expr("NULL"),
				"date":       gorm.Expr(`EXCLUDED."date"`),
			}),
		}).Create(&batch).Error

		if err != nil {
			tx.Rollback()
			r.logger.Error().Err(err).Msg("Failed to insert or update slack notifications")
			return err
		}
		if err := tx.Commit().Error; err != nil {
			r.logger.Error().Err(err).Msg("Failed to commit transaction")
			return err
		}
	}
	return nil
}

// ProcessTopNotifications 处理当天计数器最多的前十条记录并解除节流
func (r *slackNotificationRepository) ProcessTopNotifications() error {
	var topNotifications []schema.SlackNotifications
	// 获取当前时间
	now := time.Now()
	// 设置小时、分钟、秒和纳秒为 0
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// 获取时间戳
	timestamp := midnight.Unix()
	err := r.db.DB.Where("date > ?", timestamp).Order("counter DESC").Limit(10).Find(&topNotifications).Error
	if err != nil {
		r.logger.Error().Err(err).Msg("Failed to query top notifications")
		return err
	}

	r.logger.Debug().Msgf("ProcessTopNotifications timestamp: %d len: %d", timestamp, len(topNotifications))

	var keysToDelete []string
	for _, notification := range topNotifications {
		throttleKey := shared.CoinsThrottlePrefix + notification.CoinID
		throttleCountKey := shared.CoinsThrottleCountPrefix + notification.CoinID
		t, err := time.Parse("2006-01-02", notification.DayDate)
		if err != nil {
			r.logger.Error().Err(err).Msgf("Failed to parse date: %s", notification.DayDate)
			continue
		}
		output := t.Format("02-01-2006")
		historicalThrottleKey := throttleKey + "_" + output
		historicalThrottleCountKey := shared.CoinsThrottleCountPrefix + notification.CoinID + "_" + output

		keysToDelete = append(keysToDelete, throttleKey, throttleCountKey, historicalThrottleKey, historicalThrottleCountKey)
	}

	if err := r.redisClient.DeleteKeyBatch(keysToDelete); err != nil {
		r.logger.Error().Err(err).Msg("Failed to delete keys in batch")
	}

	// 使用 CoinID 和 DayDate 删除记录
	for _, notification := range topNotifications {
		if err := r.db.DB.Where("coin_id = ? AND day_date = ?", notification.CoinID, notification.DayDate).Delete(&schema.SlackNotifications{}).Error; err != nil {
			r.logger.Error().Err(err).Msgf("Failed to delete notification record: %s", notification.CoinID)
		}
	}

	return nil
}

func (r *slackNotificationRepository) DeleteOldData() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// 计算三天前的时间
	threeDaysAgo := time.Now().AddDate(0, 0, -3)

	// 物理删除 RequestLog 表中超过三天的数据
	if err := r.db.DB.WithContext(ctx).Where("created_at < ?", threeDaysAgo).Unscoped().Delete(&schema.RequestLog{}).Error; err != nil {
		return fmt.Errorf("删除 RequestLog 表数据时出错: %v", err)
	}

	// 物理删除 SlackNotifications 表中超过三天的数据
	if err := r.db.DB.WithContext(ctx).Where("created_at < ?", threeDaysAgo).Unscoped().Delete(&schema.SlackNotifications{}).Error; err != nil {
		return fmt.Errorf("删除 SlackNotifications 表数据时出错: %v", err)
	}

	return nil
}
