package shared

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
)

type SlackPayload struct {
	Channel   string `json:"channel"`
	Username  string `json:"username"`
	Text      string `json:"text"`
	IconEmoji string `json:"icon_emoji"`
}

// 常量配置
const (
	RedisErrorCountPrefix          = "error_count:"
	RedisErrorCountDuration        = 10 * time.Minute // 计数过期时间
	RedisErrorThreshold            = 5                // 阈值
	SlackWebhookURL                = "https://hooks.slack.com/services/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"
	SlackNotificationResetDuration = 60 * 24 * time.Minute // 通知重置时间
)

func SendSlackAlert(alertedKey string, message string, logger zerolog.Logger, redisClient *RedisClient) error {
	ctx := context.Background()
	// 获取计数器值

	// 检查是否已经发送过告警
	if counterValue, err := redisClient.Client.Get(ctx, alertedKey).Result(); err == nil && counterValue != "1" {
		return nil
	}
	payload := SlackPayload{
		Channel:  "#price-api-v2-alert",
		Username: "price-bot",
		Text:     message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal Slack payload")
		return err
	}

	req, err := http.NewRequest("POST", SlackWebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		logger.Error().Err(err).Msg("Failed to create Slack request")
		return err
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to send Slack request")
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Error().Msgf("Slack request failed with status code: %d", resp.StatusCode)
		return err
	}

	logger.Info().Msg("Slack notification sent successfully")
	return nil
}

// HandleErrorWithThrottling 处理错误统计和节流
func HandleErrorWithThrottling(redisClient *RedisClient, logger zerolog.Logger, key string, errorMsg string) {
	ctx := context.Background()
	errorCountKey := RedisErrorCountPrefix + key
	alertedKey := errorCountKey + ":alerted"
	lockKey := errorCountKey + ":lock"
	// 尝试获取锁
	lockAcquired, err := redisClient.Client.SetNX(ctx, lockKey, "1", time.Second*10).Result()
	if err != nil {
		return
	}
	if !lockAcquired {
		// 如果无法获取锁，直接返回
		return
	}
	defer redisClient.Client.Del(ctx, lockKey) // 确保锁最终被释放
	// 检查是否已经发送过告警
	if _, err := redisClient.Client.Get(ctx, alertedKey).Result(); err == nil {
		return
	}
	// 增加错误计数
	count, err := redisClient.Client.Incr(ctx, errorCountKey).Result()
	if err != nil {
		logger.Error().Err(err).Msg("增加错误计数 异常！")
		return
	}

	if count == 1 {
		redisClient.Client.Expire(ctx, errorCountKey, RedisErrorCountDuration)
	}

	// 达到阈值时发送通知并重置计数
	if count >= RedisErrorThreshold {
		redisClient.Client.Set(ctx, alertedKey, "1", RedisErrorCountDuration)
		SendSlackAlert(key, fmt.Sprintf("错误请求已达到阈值 %s: %s", key, errorMsg), logger, redisClient)
		redisClient.Client.Del(ctx, errorCountKey)
	}
}
