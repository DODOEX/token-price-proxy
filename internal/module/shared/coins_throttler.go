package shared

import (
	"context"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// CoinsThrottler 结构体，用于持有 Redis 客户端和其他配置
type CoinsThrottler struct {
	redisClient *RedisClient
	logger      zerolog.Logger
	coinChecker CoinChecker
}

type CoinChecker interface {
	CheckCoinExists(coinID string) (bool, error)
}

// Throttler 配置常量
const (
	// CoinsThrottlePrefix 节流键的前缀
	CoinsThrottlePrefix = "coins_throttle:"

	// CoinsThrottleCountPrefix 节流计数键的前缀
	CoinsThrottleCountPrefix = "coins_throttle_count:"

	// CoinsThrottleDuration 节流键的有效期
	CoinsThrottleDuration = 60 * 1 * time.Second

	// CoinsThrottleCountResetDuration 节流计数键的重置时间
	CoinsThrottleCountResetDuration = 30 * time.Minute

	// CoinsMaxThrottleCount 最大节流计数
	CoinsMaxThrottleCount = 3

	// SlackNotificationResetDuration Slack 通知重置时间
	CoinsSlackNotificationResetDuration = 30 * time.Minute
)

// NewCoinsThrottler 创建一个新的 CoinsThrottler 实例
func NewCoinsThrottler(redisClient *RedisClient, logger zerolog.Logger, coinChecker CoinChecker) *CoinsThrottler {
	return &CoinsThrottler{
		redisClient: redisClient,
		logger:      logger,
		coinChecker: coinChecker,
	}
}

// IsCoinsThrottled 检查请求是否被节流
func (t *CoinsThrottler) IsCoinsThrottled(coinsId string) bool {
	chainId := strings.Split(coinsId, "_")[0]
	if _, exists := RefuseChainIdMap[chainId]; exists {
		return true
	}

	ctx := context.Background()

	// 节流主键
	throttleKey := CoinsThrottlePrefix + coinsId

	// 检查请求是否已被节流
	if _, err := t.redisClient.Client.Get(ctx, throttleKey).Result(); err == nil {
		return true
	}

	return false
}

func (t *CoinsThrottler) GetAlertedKey(coinsId string) string {
	if coinsId == "" {
		return ""
	}
	return CoinsThrottleCountPrefix + coinsId + ":alerted"
}

// CoinsThrottle 增加节流计数并检查是否超过限制和允许发送Slack，如果超过限制则返回 true 并且重置
func (t *CoinsThrottler) CoinsThrottle(coinsId string, reqeusetStatus string) bool {
	ctx := context.Background()

	if reqeusetStatus == "429" {
		// 设置节流键
		if err := t.redisClient.Client.Set(ctx, CoinsThrottlePrefix+coinsId, "1", 3*time.Minute).Err(); err != nil {
			t.logger.Error().Err(err).Msgf("设置节流键失败: %s", coinsId)
		}
		return false
	}

	// 节流计数键
	throttleCountKey := CoinsThrottleCountPrefix + coinsId

	// Slack 发送标志
	alertedKey := t.GetAlertedKey(coinsId)

	// 增加节流计数
	count, err := t.redisClient.Client.Incr(ctx, throttleCountKey).Result()
	if err != nil {
		t.logger.Error().Err(err).Msgf("增加节流计数失败: %s", coinsId)
		return false
	}
	if count == 1 {
		if err := t.redisClient.Client.Expire(ctx, throttleCountKey, CoinsThrottleCountResetDuration).Err(); err != nil {
			t.logger.Error().Err(err).Msgf("设置节流计数键过期时间失败: %s", coinsId)
		}
	}

	// 检查是否超过限制
	if count >= CoinsMaxThrottleCount {
		// 检查 coins 表中是否存在这个 coin
		coinExists, err := t.coinChecker.CheckCoinExists(coinsId)
		if err != nil {
			t.logger.Error().Err(err).Msgf("检查 Coin 存在性失败: %s", coinsId)
			return false
		}

		var duration time.Duration
		if has, err := t.redisClient.HasHistoricalPriceCache(coinsId); err == nil && has {
			// 如果存在历史记录，限流1分钟
			duration = 1 * time.Minute
		} else if !coinExists {
			// 如果 coin 不存在，限流 24 小时
			duration = 24 * time.Hour
		} else {
			// 如果 coin 存在，限流 30 分钟
			duration = 30 * time.Minute
		}

		if err := t.redisClient.Client.Set(ctx, CoinsThrottlePrefix+coinsId, "1", duration).Err(); err != nil {
			t.logger.Error().Err(err).Msgf("设置节流键失败: %s", coinsId)
			return false
		}

		// 新增 Slack 发送标志
		alertCount, _ := t.redisClient.Client.Incr(ctx, alertedKey).Result()
		if alertCount == 1 {
			if err := t.redisClient.Client.Expire(ctx, alertedKey, SlackNotificationResetDuration).Err(); err != nil {
				t.logger.Error().Err(err).Msgf("设置 Slack 发送标志失败: %s", coinsId)
			}
		}

		// 删除计数器
		if err := t.redisClient.Client.Del(ctx, throttleCountKey).Err(); err != nil {
			t.logger.Error().Err(err).Msgf("删除节流计数键失败: %s", coinsId)
		}
		return true
	}

	// 设置节流键
	if err := t.redisClient.Client.Set(ctx, CoinsThrottlePrefix+coinsId, "1", CoinsThrottleDuration).Err(); err != nil {
		t.logger.Error().Err(err).Msgf("设置节流键失败: %s", coinsId)
	}
	return false
}
