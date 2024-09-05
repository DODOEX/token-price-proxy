package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database"
	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/redis/go-redis/v9"
)

type AppTokenRepository interface {
	GetAppTokenByToken(token string) (*schema.AppToken, error)
	GetAllAppTokens() ([]schema.AppToken, error)
	AddAppToken(appToken *schema.AppToken) error
	UpdateAppToken(appToken *schema.AppToken) error
	DeleteAppToken(token string) error
}

type appTokenRepository struct {
	db          *database.Database
	redisClient *shared.RedisClient
}

func NewAppTokenRepository(db *database.Database, redisClient *shared.RedisClient) AppTokenRepository {
	return &appTokenRepository{
		db:          db,
		redisClient: redisClient,
	}
}

func (r *appTokenRepository) GetAppTokenByToken(token string) (*schema.AppToken, error) {
	ctx := context.Background()
	cacheKey := "app_token:" + token

	// 尝试从 Redis 缓存中获取
	val, err := r.redisClient.Client.Get(ctx, cacheKey).Result()
	if err == redis.Nil {
		// 缓存未命中，从数据库中获取
		var appToken schema.AppToken
		if err := r.db.DB.Where("token = ?", token).First(&appToken).Error; err != nil {
			return nil, err
		}
		// 将结果存入缓存
		data, _ := json.Marshal(appToken)
		r.redisClient.Client.Set(ctx, cacheKey, data, 30*time.Minute)

		return &appToken, nil
	} else if err != nil {
		return nil, err
	}

	// 缓存命中，反序列化结果
	var appToken schema.AppToken
	if err := json.Unmarshal([]byte(val), &appToken); err != nil {
		return nil, err
	}

	return &appToken, nil
}

func (r *appTokenRepository) GetAllAppTokens() ([]schema.AppToken, error) {
	var appTokens []schema.AppToken
	if err := r.db.DB.Where("deleted_at IS NULL").Find(&appTokens).Error; err != nil {
		return nil, err
	}
	return appTokens, nil
}

func (r *appTokenRepository) AddAppToken(appToken *schema.AppToken) error {
	if err := r.db.DB.Create(appToken).Error; err != nil {
		return err
	}

	cacheKey := "app_token:" + appToken.Token
	data, _ := json.Marshal(appToken)
	r.redisClient.Client.Set(context.Background(), cacheKey, data, 10*time.Minute)

	return nil
}

func (r *appTokenRepository) UpdateAppToken(appToken *schema.AppToken) error {
	// 删除旧的缓存
	oldAppToken, err := r.GetAppTokenByToken(appToken.Token)
	if err != nil {
		return err
	}
	oldCacheKey := "app_token:" + oldAppToken.Token
	r.redisClient.Client.Del(context.Background(), oldCacheKey)
	r.redisClient.Client.Del(context.Background(), "rate_limit:"+oldAppToken.Token)

	if err := r.db.DB.Save(appToken).Error; err != nil {
		return err
	}

	// 设置新的缓存
	newCacheKey := "app_token:" + appToken.Token
	data, _ := json.Marshal(appToken)
	r.redisClient.Client.Set(context.Background(), newCacheKey, data, 10*time.Minute)
	r.redisClient.Client.Del(context.Background(), "rate_limit:"+appToken.Token)

	return nil
}

func (r *appTokenRepository) DeleteAppToken(token string) error {
	cacheKey := "app_token:" + token
	r.redisClient.Client.Del(context.Background(), cacheKey)
	r.redisClient.Client.Del(context.Background(), "rate_limit:"+token)

	if err := r.db.DB.Where("token = ?", token).Delete(&schema.AppToken{}).Error; err != nil {
		return err
	}

	return nil
}
