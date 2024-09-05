package service_test

import (
	"context"
	"testing"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/stretchr/testify/assert"
)

func setupRateLimiterService() (*service.RateLimiterService, repository.AppTokenRepository, *shared.RedisClient) {
	// 设置真实的数据库和 Redis 连接
	db := shared.SetupRealDB()
	redisClient := shared.SetupRealRedis()
	appTokenRepo := repository.NewAppTokenRepository(db, redisClient)
	return service.NewRateLimiterService(appTokenRepo, redisClient), appTokenRepo, redisClient
}

func TestRateLimiterService_Allow_ValidToken(t *testing.T) {
	service, appTokenRepo, _ := setupRateLimiterService()

	// 先添加一个 AppToken 到数据库中
	appToken := &schema.AppToken{
		Name:  "TestApp",
		Token: "valid_token",
		Rate:  10,
	}
	err := appTokenRepo.AddAppToken(appToken)
	assert.NoError(t, err)

	// 测试有效 token 请求允许
	allowed, err := service.Allow(context.Background(), appToken.Token)
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiterService_Allow_ExceededLimit(t *testing.T) {
	service, appTokenRepo, _ := setupRateLimiterService()

	// 添加一个 AppToken 到数据库中
	appToken := &schema.AppToken{
		Token: "limited_token",
		Rate:  1,
	}
	err := appTokenRepo.AddAppToken(appToken)
	assert.NoError(t, err)

	// 首次请求应被允许
	allowed, err := service.Allow(context.Background(), appToken.Token)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// 超出限额的请求不应被允许
	allowed, err = service.Allow(context.Background(), appToken.Token)
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestRateLimiterService_Allow_DefaultToken(t *testing.T) {
	service, _, _ := setupRateLimiterService()

	// 使用空 token，应该使用默认限额
	allowed, err := service.Allow(context.Background(), "")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestRateLimiterService_Allow_RedisError(t *testing.T) {
	service, _, redisClient := setupRateLimiterService()

	// 关闭 Redis 以模拟 Redis 错误
	redisClient.Client.Close()

	// 使用有效 token 测试 Redis 错误情况下的请求
	allowed, err := service.Allow(context.Background(), "some_token")
	assert.Error(t, err)
	assert.False(t, allowed)

	// 重新连接 Redis
	// redisClient = shared.SetupRealRedis()
}

func TestRateLimiterService_Allow_RepoError(t *testing.T) {
	service, _, _ := setupRateLimiterService()

	// 测试一个不存在的 token，应该返回错误
	allowed, err := service.Allow(context.Background(), "non_existent_token")
	assert.Error(t, err)
	assert.False(t, allowed)
}

func TestRateLimiterService_Allow_ConcurrentRequests(t *testing.T) {
	service, appTokenRepo, _ := setupRateLimiterService()

	// 添加一个 AppToken 到数据库中
	appToken := &schema.AppToken{
		Token: "concurrent_token",
		Rate:  5,
	}
	err := appTokenRepo.AddAppToken(appToken)
	assert.NoError(t, err)

	// 模拟多次并发请求
	var allowedCount int
	for i := 0; i < 10; i++ {
		allowed, err := service.Allow(context.Background(), appToken.Token)
		assert.NoError(t, err)
		if allowed {
			allowedCount++
		}
	}

	assert.Greater(t, allowedCount, 5)
}
