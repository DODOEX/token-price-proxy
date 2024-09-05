package service

import (
	"context"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
)

type SlackNotificationService interface {
	SaveLog(ctx context.Context, source, chainID, address, dateDay string, timestamp int64) error
}

type slackNotificationService struct {
	slackRepo   repository.SlackNotificationRepository
	redisClient *shared.RedisClient
	logger      zerolog.Logger
}

func NewSlackNotificationService(slackRepo repository.SlackNotificationRepository, redisClient *shared.RedisClient, logger zerolog.Logger) SlackNotificationService {
	return &slackNotificationService{
		slackRepo:   slackRepo,
		redisClient: redisClient,
		logger:      logger,
	}
}

func (s *slackNotificationService) SaveLog(ctx context.Context, source, chainID, address, dateDay string, timestamp int64) error {
	if _, exists := shared.RefuseChainIdMap[chainID]; exists {
		return nil
	}
	// 插入数据库记录
	notification := schema.SlackNotifications{
		Source:  source,
		CoinID:  chainID + "_" + address,
		DayDate: dateDay,
		Date:    timestamp,
		Counter: 1,
	}
	s.slackRepo.InsertNotification(ctx, notification)
	return nil
}
