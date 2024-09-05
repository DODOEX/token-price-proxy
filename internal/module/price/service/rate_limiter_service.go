package service

import (
	"context"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
)

type RateLimiterService struct {
	appTokenRepo repository.AppTokenRepository
	redisClient  *shared.RedisClient
}

func NewRateLimiterService(appTokenRepo repository.AppTokenRepository, redisClient *shared.RedisClient) *RateLimiterService {
	return &RateLimiterService{
		appTokenRepo: appTokenRepo,
		redisClient:  redisClient,
	}
}

func (s *RateLimiterService) Allow(ctx context.Context, token string) (bool, error) {
	var appToken *schema.AppToken
	var err error

	if token == "" {
		appToken = &schema.AppToken{Token: "DEFAULT_TOKEN", Rate: float64(shared.AllowApiKeyNilRateLimiter)}
	} else {
		appToken, err = s.appTokenRepo.GetAppTokenByToken(token)
		if err != nil {
			return false, err
		}
	}
	key := "rate_limit:" + token
	interval := time.Second
	limit := appToken.Rate

	allowed, err := s.redisClient.Client.Eval(ctx, `
		local key = KEYS[1]
		local limit = tonumber(ARGV[1])
		local interval = tonumber(ARGV[2])
		local current = redis.call("GET", key)
		if current and tonumber(current) >= limit then
			return 0
		else
			redis.call("INCR", key)
			redis.call("EXPIRE", key, interval)
			return 1
		end
	`, []string{key}, limit, int64(interval.Seconds())).Int()

	if err != nil {
		return false, err
	}

	return allowed == 1, nil
}
