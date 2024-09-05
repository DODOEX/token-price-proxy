package shared

import (
	"context"
	"time"

	"github.com/knadh/koanf/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type RedisClient struct {
	Client           *redis.Client
	url              string
	options          *redis.Options
	retryCount       int
	keepliveInterval time.Duration
	logger           zerolog.Logger
}

const (
	redisCurrentPricePrefix             = "price:current:"
	redisHistoricalPricePrefix          = "price:historical:"
	redisHistoricalPriceExistencePrefix = "price:historical:exists:"
	batchSize                           = 1000
	maxRetries                          = 3
)

func NewRedisClient(cfg *koanf.Koanf, logger zerolog.Logger) *RedisClient {
	url := cfg.String("redis.url")
	opts, err := redis.ParseURL(url)
	if err != nil {
		logger.Panic().Err(err)
	}

	return &RedisClient{
		Client:           nil,
		options:          opts,
		logger:           logger,
		url:              url,
		retryCount:       cfg.Int("redis.retry-count"),
		keepliveInterval: cfg.Duration("redis.keeplive-interval"),
	}
}

func (r *RedisClient) keeplive() {
	for {
		if r.Client == nil {
			for i := 1; i <= r.retryCount; i++ {
				_, err := r.Client.Ping(context.Background()).Result()
				if err == nil || err == redis.Nil {
					r.logger.Info().Msgf("Reconnected to Redis succesfully!")
					break
				} else {
					if i == r.retryCount {
						r.Close()
						r.logger.Panic().Msgf("Failed to connect to Redis: %v. Retrying in %v...\n", err, i)
						return
					}

					r.logger.Warn().Msgf("Failed to connect to Redis: %v. Retrying in %v...\n", err, i)
					r.Client = redis.NewClient(r.options)
				}
			}
		} else {
			r.Client = redis.NewClient(r.options)
		}

		time.Sleep(r.keepliveInterval)
	}
}

func (r *RedisClient) Connect() {
	r.Client = redis.NewClient(r.options)
	go r.keeplive()
}

func (r *RedisClient) Close() error {
	return r.Client.Close()
}

func (r *RedisClient) DeleteKeysByPrefix(prefix string) error {
	ctx := context.Background()
	iter := r.Client.Scan(ctx, 0, prefix+"*", 0).Iterator()
	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
		if len(keys) >= batchSize {
			if err := r.DeleteKeyBatch(keys); err != nil {
				return err
			}
			keys = keys[:0]
		}
	}

	// Delete remaining keys if any
	if len(keys) > 0 {
		if err := r.DeleteKeyBatch(keys); err != nil {
			return err
		}
	}

	if err := iter.Err(); err != nil {
		return err
	}
	return nil
}
func (r *RedisClient) DeleteKeyBatch(keys []string) error {
	ctx := context.Background()

	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]

		for retries := 0; retries < maxRetries; retries++ {
			pipe := r.Client.Pipeline()
			for _, key := range batch {
				pipe.Del(ctx, key)
			}
			_, err := pipe.Exec(ctx)
			if err != nil {
				if retries == maxRetries-1 {
					return err
				}
				continue
			}
			break
		}
	}

	return nil
}
func (r *RedisClient) DeleteCoinBatch(keys []string) {
	ctx := context.Background()
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}
		batch := keys[i:end]

		for retries := 0; retries < 3; retries++ {
			pipe := r.Client.Pipeline()
			success := true

			for _, key := range batch {
				if err := pipe.Del(ctx, key).Err(); err != nil {
					r.logger.Error().Err(err).Msgf("删除单个键失败: %s", key)
					success = false
					break
				}
			}

			if !success {
				break
			}

			_, err := pipe.Exec(ctx)
			if err != nil {
				r.logger.Error().Err(err).Msgf("执行管道删除命令失败，重试次数: %d", retries+1)
			} else {
				break
			}
		}
	}
}
func (r *RedisClient) AcquireLock(lockKey string, ttl time.Duration) bool {
	ctx := context.Background()
	ok, err := r.Client.SetNX(ctx, lockKey, "locked", ttl).Result()
	if err != nil {
		r.logger.Debug().Msg("获取锁失败 key" + lockKey)
		return false
	}
	if !ok {
		r.logger.Debug().Msg("任务已经被其他实例锁定 key:" + lockKey)
		return false
	}
	return true
}

func (r *RedisClient) ReleaseLock(lockKey string) {
	ctx := context.Background()
	r.Client.Del(ctx, lockKey)
}

func (r *RedisClient) SetCurrentPriceCache(coinID string, price string) error {
	cacheKey := redisCurrentPricePrefix + coinID
	return r.Client.Set(context.Background(), cacheKey, price, 10*time.Minute).Err()
}

func (r *RedisClient) GetCurrentPricesCache(coinIDs []string) (map[string]string, error) {
	priceMap := make(map[string]string)
	for _, coinID := range coinIDs {
		cacheKey := redisCurrentPricePrefix + coinID
		price, err := r.Client.Get(context.Background(), cacheKey).Result()
		if err == nil && price != "" {
			priceMap[coinID] = price
		}
	}
	return priceMap, nil
}
func (r *RedisClient) GetCurrentPriceCache(coinID string) (string, error) {
	cacheKey := redisCurrentPricePrefix + coinID
	return r.Client.Get(context.Background(), cacheKey).Result()
}

func (r *RedisClient) SetHistoricalPriceCache(coinID string, dayDate string, price string) error {
	cacheKey := redisHistoricalPricePrefix + coinID + "_" + dayDate
	cacheDuration := 72 * time.Hour
	if dayDate == time.Now().Format("02-01-2006") {
		cacheDuration = 24 * time.Hour
	}
	//是否存在历史的缓存标记
	r.Client.Set(context.Background(), redisHistoricalPriceExistencePrefix+coinID, "1", 24*7*time.Hour)
	return r.Client.Set(context.Background(), cacheKey, price, cacheDuration).Err()
}

func (r *RedisClient) GetHistoricalPriceCache(coinID string, dayDate string) (string, error) {
	cacheKey := redisHistoricalPricePrefix + coinID + "_" + dayDate
	return r.Client.Get(context.Background(), cacheKey).Result()
}
func (r *RedisClient) HasHistoricalPriceCache(coinID string) (bool, error) {
	cacheKey := redisHistoricalPriceExistencePrefix + coinID
	have, err := r.Client.Get(context.Background(), cacheKey).Result()
	if err != nil && err != redis.Nil {
		return false, err
	}
	return have == "1", nil
}
