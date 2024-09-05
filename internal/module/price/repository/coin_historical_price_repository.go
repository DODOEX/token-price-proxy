package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database"
	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gorm.io/gorm/clause"
)

const (
	historicalQueueKey     = "historical_prices:queue"
	historicalSetKeyPrefix = "historical_prices:set:"
	historicalQueueRunSize = 1000
	maxRetries             = 3
	retryDelay             = 2 * time.Second
	lockTTL                = 15 * time.Second
	lockRetryInterval      = 1 * time.Second
	lockRetryCount         = 3
)

type CoinHistoricalPriceRepository interface {
	SaveHistoricalPrices(prices []schema.CoinHistoricalPrice) error
	GetHistoricalPrices(coinIDs []string, dates []int64) (map[string]string, error)
	ProcessQueue() error
}

type coinHistoricalPriceRepository struct {
	coinRepository CoinRepository
	db             *database.Database
	redisClient    *shared.RedisClient
	logger         zerolog.Logger
}

func NewCoinHistoricalPriceRepository(db *database.Database, logger zerolog.Logger, redisClient *shared.RedisClient, coinRepository CoinRepository) CoinHistoricalPriceRepository {
	return &coinHistoricalPriceRepository{
		db:             db,
		logger:         logger,
		redisClient:    redisClient,
		coinRepository: coinRepository,
	}
}

// SaveHistoricalPrices 添加历史价格到队列中，如果队列长度达到限制则处理队列
func (r *coinHistoricalPriceRepository) SaveHistoricalPrices(prices []schema.CoinHistoricalPrice) error {
	if len(prices) == 0 {
		return nil
	}

	// 使用map进行去重
	uniquePrices := make(map[string]schema.CoinHistoricalPrice)
	for _, price := range prices {
		key := fmt.Sprintf("%s%s_%s", historicalSetKeyPrefix, price.CoinID, price.DayDate)
		uniquePrices[key] = price
	}

	ctx := context.Background()
	pipe := r.redisClient.Client.Pipeline()

	// 批量检查成员是否存在
	for key, price := range uniquePrices {
		pipe.SIsMember(ctx, key, price.CoinID)
	}
	results, err := pipe.Exec(ctx)
	if err != nil {
		return err
	}

	existsMap := make(map[string]bool)
	i := 0
	for key := range uniquePrices {
		existsMap[key] = results[i].(*redis.BoolCmd).Val()
		i++
	}

	pipe = r.redisClient.Client.Pipeline()
	for key, price := range uniquePrices {
		exists := existsMap[key]

		// 添加价格缓存，当天10分钟，其他时间3天
		r.redisClient.SetHistoricalPriceCache(price.CoinID, price.DayDate, price.Price)
		if exists {
			// 从队列中删除旧的记录并替换为新的记录
			queueItemsCmd := pipe.LRange(ctx, historicalQueueKey, 0, -1)
			queueItems, err := queueItemsCmd.Result()
			if err != nil {
				return err
			}

			for i, item := range queueItems {
				var oldPrice schema.CoinHistoricalPrice
				if err := json.Unmarshal([]byte(item), &oldPrice); err != nil {
					return err
				}
				if oldPrice.CoinID == price.CoinID && oldPrice.DayDate == price.DayDate {
					// 替换旧的记录
					data, err := json.Marshal(price)
					if err != nil {
						return err
					}
					pipe.LSet(ctx, historicalQueueKey, int64(i), data)
					break
				}
			}
		} else {
			// 添加记录到集合中
			pipe.SAdd(ctx, key, price.CoinID)

			// 将记录添加到队列
			data, err := json.Marshal(price)
			if err != nil {
				return err
			}
			pipe.RPush(ctx, historicalQueueKey, data)
		}
	}

	// 执行管道中的所有命令
	_, err = pipe.Exec(ctx)
	if err != nil {
		return err
	}

	// 检查队列长度，如果达到队列大小则处理队列
	queueLen, err := r.redisClient.Client.LLen(ctx, historicalQueueKey).Result()
	if err != nil {
		return err
	}

	if queueLen >= int64(historicalQueueRunSize) {
		err = r.ProcessQueue()
		if err != nil {
			return err
		}
	}

	return nil
}

// ProcessQueue 处理队列中的历史价格
func (r *coinHistoricalPriceRepository) ProcessQueue() error {
	lockKey := "lock:historical_prices_queue"

	// 尝试获取锁，确保多个 Pod 之间对队列的操作是互斥的
	for attempt := 1; attempt <= lockRetryCount; attempt++ {
		ok := r.redisClient.AcquireLock(lockKey, lockTTL)
		if ok {
			defer r.redisClient.ReleaseLock(lockKey)
			return r.processQueueWithTransaction()
		}
		time.Sleep(lockRetryInterval)
	}
	return nil

}

// processQueueWithTransaction 处理队列中的历史价格，使用事务确保操作的原子性
func (r *coinHistoricalPriceRepository) processQueueWithTransaction() error {
	historicalPricesData, err := r.redisClient.Client.LRange(context.Background(), historicalQueueKey, 0, -1).Result()
	if err != nil {
		return err
	}

	if len(historicalPricesData) == 0 {
		return nil
	}
	defer func() {
		// 清空 Redis 队列
		r.redisClient.Client.Del(context.Background(), historicalQueueKey).Err()
		r.redisClient.DeleteKeysByPrefix(historicalSetKeyPrefix)
	}()
	var historicalPrices []schema.CoinHistoricalPrice
	for _, priceData := range historicalPricesData {
		var price schema.CoinHistoricalPrice
		if err := json.Unmarshal([]byte(priceData), &price); err != nil {
			return err
		}
		historicalPrices = append(historicalPrices, price)
	}

	// 重试机制，处理可能的死锁和其他临时性错误
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = r.processQueueTransaction(historicalPrices)
		if err == nil {
			break
		}
		if err != nil && err.Error() == "ERROR: deadlock detected (SQLSTATE 40P01)" {
			r.logger.Error().Err(err).Msgf("Deadlock detected, retrying (%d/%d)", attempt, maxRetries)
			time.Sleep(retryDelay)
			continue
		}
		r.logger.Error().Err(err).Msg("Failed to process queue")
		return err
	}

	return nil
}
func (r *coinHistoricalPriceRepository) processQueueTransaction(historicalPrices []schema.CoinHistoricalPrice) error {
	batchSize := 1000
	r.logger.Debug().Msgf("coinHistoricalPriceRepository processQueueTransaction len: %d", len(historicalPrices))
	var coinIDs []string
	coinIDMap := make(map[string]string)

	for i := 0; i < len(historicalPrices); i += batchSize {
		end := i + batchSize
		if end > len(historicalPrices) {
			end = len(historicalPrices)
		}
		batch := historicalPrices[i:end]

		tx := r.db.DB.Begin()
		for _, price := range batch {
			err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "coin_id"}, {Name: "day_date"}},
				DoUpdates: clause.AssignmentColumns([]string{"price", "source", "updated_at"}),
			}).Create(&price).Error
			if err != nil {
				tx.Rollback()
				r.logger.Error().Err(err).Msg("批量插入或更新历史价格失败")
				return err
			}
			if price.DayDate == time.Now().Format("02-01-2006") {
				coinIDs = append(coinIDs, price.CoinID)
				coinIDMap[price.CoinID] = price.Source
			}
		}
		if err := tx.Commit().Error; err != nil {
			r.logger.Error().Err(err).Msg("提交事务失败")
			return err
		}
	}

	coins, err := r.coinRepository.GetCoinsByID(coinIDs)
	if err != nil {
		for _, coin := range coins {
			if source, found := coinIDMap[coin.ID]; coin.Address != "" && coin.ChainID != "" && found && source != "" {
				coin.LastPriceSource = &source
				coin.UpdatedAt = time.Now()
			}
		}
	}

	if len(coins) > 0 {
		err := r.coinRepository.AddToQueue(coins)
		if err != nil {
			r.logger.Error().Err(err).Msg("批量添加到队列失败")
			return err
		}
	}
	r.logger.Debug().Msg("coinHistoricalPriceRepository 处理队列成功")
	return nil
}
func (r *coinHistoricalPriceRepository) GetHistoricalPrices(coinIDs []string, dates []int64) (map[string]string, error) {
	dayDates := make([]string, len(dates))
	currentDay := time.Now().Format("02-01-2006")
	for i, date := range dates {
		dayDates[i] = time.Unix(date, 0).Format("02-01-2006")
	}

	priceMap := make(map[string]string)
	missingPrices := make(map[string][]string) // 用于存储每个 coinID 对应的缺失日期

	for i, coinID := range coinIDs {
		price, err := r.redisClient.GetHistoricalPriceCache(coinID, dayDates[i])
		if err == nil && price != "" {
			priceMap[coinID+"_"+dayDates[i]] = price
		} else {
			if dayDates[i] != currentDay { // 跳过当天日期的数据库查询
				missingPrices[coinID] = append(missingPrices[coinID], dayDates[i])
			}
		}
	}

	if len(missingPrices) > 0 {
		for coinID, days := range missingPrices {
			for _, day := range days {
				// 执行数据库查询
				var existingPrices []schema.CoinHistoricalPrice
				err := r.db.DB.Where("coin_id = ? AND day_date = ?", coinID, day).Find(&existingPrices).Error
				if err != nil {
					r.logger.Error().Err(err).Msgf("批量查询历史价格失败: coinID=%s, day=%s", coinID, day)
					continue
				}

				for _, price := range existingPrices {
					priceMap[price.CoinID+"_"+price.DayDate] = price.Price
					r.redisClient.SetHistoricalPriceCache(price.CoinID, price.DayDate, price.Price)

				}
			}
		}
	}
	return priceMap, nil
}
