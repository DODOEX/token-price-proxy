package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// // MockLifecycle 模拟的 fx.Lifecycle
// type MockLifecycle struct {
// 	hooks []fx.Hook
// }

// func NewMockLifecycle() *MockLifecycle {
// 	return &MockLifecycle{}
// }

// func (m *MockLifecycle) Append(hook fx.Hook) {
// 	m.hooks = append(m.hooks, hook)
// }

// func (m *MockLifecycle) Start(ctx context.Context) error {
// 	for _, hook := range m.hooks {
// 		if hook.OnStart != nil {
// 			if err := hook.OnStart(ctx); err != nil {
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }

// func (m *MockLifecycle) Stop(ctx context.Context) error {
// 	for _, hook := range m.hooks {
// 		if hook.OnStop != nil {
// 			if err := hook.OnStop(ctx); err != nil {
// 				return err
// 			}
// 		}
// 	}
// 	return nil
// }

// 设置生命周期
func setupSlackNotificationService() (service.SlackNotificationService, repository.SlackNotificationRepository, *shared.RedisClient) {
	db := shared.SetupRealDB()
	redisClient := shared.SetupRealRedis()
	lc := NewMockLifecycle() // 创建一个新的生命周期对象
	slackRepo := repository.NewSlackNotificationRepository(lc, db, redisClient, zerolog.New(nil))
	return service.NewSlackNotificationService(slackRepo, redisClient, zerolog.New(nil)), slackRepo, redisClient
}

func TestSlackNotificationService_SaveLog_Valid(t *testing.T) {
	service, _, _ := setupSlackNotificationService()

	ctx := context.Background()
	source := "TestSource"
	chainID := "1"
	address := "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"
	dateDay := time.Now().Format("02-01-2006")
	timestamp := time.Now().Unix()

	err := service.SaveLog(ctx, source, chainID, address, dateDay, timestamp)
	assert.NoError(t, err)

}

func TestSlackNotificationService_SaveLog_RefusedChainID(t *testing.T) {
	service, _, _ := setupSlackNotificationService()

	ctx := context.Background()
	source := "TestSource"
	chainID := "9999" // 假设 9999 在 shared.RefuseChainIdMap 中
	address := "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"
	dateDay := time.Now().Format("02-01-2006")
	timestamp := time.Now().Unix()

	err := service.SaveLog(ctx, source, chainID, address, dateDay, timestamp)
	assert.NoError(t, err)

}

func TestSlackNotificationService_SaveLog_RedisError(t *testing.T) {
	service, _, redisClient := setupSlackNotificationService()

	// 模拟 Redis 错误
	redisClient.Close()

	ctx := context.Background()
	source := "TestSource"
	chainID := "1"
	address := "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"
	dateDay := time.Now().Format("02-01-2006")
	timestamp := time.Now().Unix()

	// 验证在 Redis 错误的情况下，SaveLog 是否能够处理错误
	err := service.SaveLog(ctx, source, chainID, address, dateDay, timestamp)
	assert.NoError(t, err)
}
