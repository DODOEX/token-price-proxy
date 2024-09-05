package scheduler

import (
	"context"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
)

// Scheduler struct to hold repositories and logger
type Scheduler struct {
	CoinHistoricalPriceRepo     repository.CoinHistoricalPriceRepository
	CoinRepo                    repository.CoinRepository
	CoinGeckoService            service.CoinGeckoService
	SlackNotificationRepository repository.SlackNotificationRepository
	RequestLogRepository        repository.RequestLogRepository
	redisClient                 *shared.RedisClient
	Logger                      zerolog.Logger
}

// NewScheduler creates a new Scheduler
func NewScheduler(coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository, coinRepo repository.CoinRepository, slackNotificationRepository repository.SlackNotificationRepository, requestLogsRepository repository.RequestLogRepository, redisClient *shared.RedisClient, logger zerolog.Logger, coinGeckoService service.CoinGeckoService) *Scheduler {
	return &Scheduler{
		CoinHistoricalPriceRepo:     coinHistoricalPriceRepo,
		CoinRepo:                    coinRepo,
		CoinGeckoService:            coinGeckoService,
		SlackNotificationRepository: slackNotificationRepository,
		RequestLogRepository:        requestLogsRepository,
		redisClient:                 redisClient,
		Logger:                      logger,
	}
}

// StartProcessQueue 定时处理队列数据
func (s *Scheduler) StartCoinsProcessQueue() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		redisLockKey := "coins_process_queue_lock"
		if s.redisClient.AcquireLock(redisLockKey, 1*time.Minute) {
			if err := s.CoinRepo.ProcessQueue(); err != nil {
				s.Logger.Error().Err(err).Msg("处理 Coin 队列失败")
			} else {
				s.Logger.Info().Msg("处理 Coin 队列成功")
			}
			s.redisClient.ReleaseLock(redisLockKey)
		}

	}
}
func (s *Scheduler) StartCoinHistoricalPriceProcessQueue() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		redisLockKey := "coin_historical_price_process_queue_lock"
		if s.redisClient.AcquireLock(redisLockKey, 1*time.Minute) {
			if err := s.CoinHistoricalPriceRepo.ProcessQueue(); err != nil {
				s.Logger.Error().Err(err).Msg("处理 CoinHistoricalPrice 队列失败")
			} else {
				s.Logger.Info().Msg("处理 CoinHistoricalPrice 队列成功")
			}
			s.redisClient.ReleaseLock(redisLockKey)
		}
	}
}

func (s *Scheduler) StartSyncCoins() {
	ticker := time.NewTicker(72 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		redisLockKey := "sync_coins_lock"
		if s.redisClient.AcquireLock(redisLockKey, 5*time.Minute) {
			if err := s.CoinGeckoService.SyncCoins(); err != nil {
				s.Logger.Error().Err(err).Msg("处理 SyncCoins 失败")
			} else {
				s.Logger.Info().Msg("处理 SyncCoins 成功")
			}
			s.redisClient.ReleaseLock(redisLockKey)
		}

	}
}
func (s *Scheduler) StartSyncCoinsCache() {
	ticker := time.NewTicker(4 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		redisLockKey := "sync_coins_cache_lock"
		if s.redisClient.AcquireLock(redisLockKey, 5*time.Minute) {
			if err := s.CoinRepo.RefreshAllCoinsCache(); err != nil {
				s.Logger.Error().Err(err).Msg("处理 SyncCoinsCache 失败")
			} else {
				s.Logger.Info().Msg("处理 SyncCoinsCache 成功")
			}
			s.redisClient.ReleaseLock(redisLockKey)
		}

	}
}

// 每分钟释放top10的token
func (s *Scheduler) StartProcessTopNotifications() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		redisLockKey := "sync_process_top_notifications_lock"
		if s.redisClient.AcquireLock(redisLockKey, 1*time.Minute) {
			if err := s.SlackNotificationRepository.ProcessTopNotifications(); err != nil {
				s.Logger.Error().Err(err).Msg("处理 ProcessTopNotifications 失败")
			} else {
				s.Logger.Info().Msg("处理 ProcessTopNotifications 成功")
			}
			s.redisClient.ReleaseLock(redisLockKey)
		}

	}
}

func (s *Scheduler) StartProcessSlackNotifications() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		redisLockKey := "sync_process_slack_notifications_lock"
		if s.redisClient.AcquireLock(redisLockKey, 1*time.Minute) {
			if err := s.SlackNotificationRepository.ProcessQueue(); err != nil {
				s.Logger.Error().Err(err).Msg("处理 ProcessSlackNotifications 失败")
			} else {
				s.Logger.Info().Msg("处理 ProcessSlackNotifications 成功")
			}
			s.redisClient.Client.Del(context.Background(), "slack_notifications:queue").Err()
			s.redisClient.ReleaseLock(redisLockKey)
		}

	}
}

func (s *Scheduler) StartProcessRequestLogs() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		redisLockKey := "sync_process_request_logs_lock"
		if s.redisClient.AcquireLock(redisLockKey, 1*time.Minute) {
			if err := s.RequestLogRepository.ProcessQueue(); err != nil {
				s.Logger.Error().Err(err).Msg("处理 ProcessProcessRequestLogs 失败")
			} else {
				s.Logger.Info().Msg("处理 ProcessProcessRequestLogs 成功")
			}
			s.redisClient.Client.Del(context.Background(), "logs:queue").Err()
			s.redisClient.ReleaseLock(redisLockKey)
		}
	}
}
func (s *Scheduler) StartProcessDeleteOldData() {
	ticker := time.NewTicker(8 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		redisLockKey := "sync_delete_old_data_lock"
		if s.redisClient.AcquireLock(redisLockKey, 1*time.Minute) {
			if err := s.SlackNotificationRepository.DeleteOldData(); err != nil {
				s.Logger.Error().Err(err).Msg("处理 DeleteOldData 失败")
			} else {
				s.Logger.Info().Msg("处理 DeleteOldData 成功")
			}
			s.redisClient.ReleaseLock(redisLockKey)
		}

	}
}
