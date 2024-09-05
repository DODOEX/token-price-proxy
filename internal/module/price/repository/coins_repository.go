package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database"
	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type CoinRepository interface {
	UpsertCoins(coins []schema.Coins) error
	GetCoinsByID(ids []string) ([]schema.Coins, error)
	GetCoinsByOneID(id string) (*schema.Coins, error)
	DeleteCoinByID(id string) error
	RefreshCoinListCache(ids []string) error
	RefreshAllCoinsCache() error
	AddToQueue(coin []schema.Coins) error
	ProcessQueue() error
	CheckCoinExists(coinID string) (bool, error)
}

type coinRepository struct {
	db          *database.Database
	logger      zerolog.Logger
	redisClient *shared.RedisClient
}

const (
	cacheDuration    = 72 * time.Hour // 缓存时间为3天
	cacheKeyPrefix   = "coins:"       // 公共的缓存前缀
	cacheKeyAllCoins = "coins:all"    // 缓存所有币种的key
	queueKey         = "coins:queue"  // 队列的key
	setKeyPrefix     = "coins:set"    // 集合的key，用于保证元素不重复
	queueRunSize     = 1000           // 队列的最大长度
	cacheBatchSize   = 1000           // 缓存的最大长度
	cacheMaxRetries  = 3
)

func NewCoinRepository(db *database.Database, logger zerolog.Logger, redisClient *shared.RedisClient) CoinRepository {
	return &coinRepository{
		db:          db,
		logger:      logger,
		redisClient: redisClient,
	}
}

// 使用反射构建ON CONFLICT SQL语句进行批量插入和更新
func (r *coinRepository) upsertCoins(tx *gorm.DB, coins []schema.Coins) error {
	if len(coins) == 0 {
		return nil
	}

	// 获取结构体字段和JSON标签列名的映射
	coinType := reflect.TypeOf(coins[0])
	var columns []string
	fieldIndexMap := make(map[string]int)

	for i := 0; i < coinType.NumField(); i++ {
		field := coinType.Field(i)
		if field.Name == "Base" {
			continue
		}
		// 提取json标签中的列名
		columnName := field.Tag.Get("json")
		if columnName == "" || columnName == "-" {
			columnName = field.Name // fallback to the field name if no json tag is found
		}
		columns = append(columns, columnName)
		fieldIndexMap[columnName] = i
	}

	if len(columns) == 0 {
		return fmt.Errorf("no valid columns found for upsert")
	}

	// 构建批量插入SQL的占位符
	var placeholders []string
	var insertValues []interface{}

	for _, coin := range coins {
		var values []string
		coinValue := reflect.ValueOf(coin)
		for _, column := range columns {
			field := coinValue.Field(fieldIndexMap[column])
			values = append(values, "?")
			insertValues = append(insertValues, field.Interface())
		}
		placeholders = append(placeholders, fmt.Sprintf("(%s)", strings.Join(values, ", ")))
	}

	// 构建ON CONFLICT SQL语句
	var updateColumns []string
	for _, coin := range coins {
		coinValue := reflect.ValueOf(coin)
		for _, column := range columns {
			if column == "created_at" || column == "updated_at" || column == "id" {
				continue
			}
			field := coinValue.Field(fieldIndexMap[column])
			// 只更新非空字段
			if field.Kind() == reflect.Ptr && field.IsNil() {
				continue
			}
			if field.Kind() == reflect.String && field.String() == "" {
				continue
			}
			updateColumns = append(updateColumns, fmt.Sprintf("%s = EXCLUDED.%s", column, column))
		}
		break // 只需要检查一个coin来生成updateColumns
	}
	onConflictSQL := fmt.Sprintf(
		"ON CONFLICT (id) DO UPDATE SET %s, updated_at = NOW()",
		strings.Join(updateColumns, ", "),
	)

	// 拼接完整SQL语句
	insertSQL := fmt.Sprintf(
		"INSERT INTO coins (%s) VALUES %s %s",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		onConflictSQL,
	)

	// 执行SQL
	if err := tx.Exec(insertSQL, insertValues...).Error; err != nil {
		return err
	}

	return nil
}

// 批量插入和更新币种，记录执行耗时
func (r *coinRepository) UpsertCoins(coins []schema.Coins) error {
	startTime := time.Now() // 记录开始时间

	batchSize := 1000 // 增加批次大小

	for i := 0; i < len(coins); i += batchSize {
		end := i + batchSize
		if end > len(coins) {
			end = len(coins)
		}

		tx := r.db.DB.Begin()

		err := r.upsertCoins(tx, coins[i:end])
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("批量插入或更新币种失败: %v", err)
		}

		if err := tx.Commit().Error; err != nil {
			r.logger.Error().Err(err).Msg("提交事务失败")
			return err
		}
		//刷新缓存
		var newCoinIDs []string
		for _, coin := range coins[i:end] {
			newCoinIDs = append(newCoinIDs, coin.ID)
		}
		r.RefreshCoinListCache(newCoinIDs)
	}

	elapsedTime := time.Since(startTime) // 计算耗时并直接合并到日志输出

	r.logger.Info().Msgf("UpsertCoins 执行了 %d 个币种，耗时 %s", len(coins), elapsedTime) // 输出耗时及处理数量

	r.logger.Debug().Msg("币种批量插入和更新成功")
	return nil
}
func (r *coinRepository) GetCoinsByID(ids []string) ([]schema.Coins, error) {
	var coins []schema.Coins
	var missingIDs []string
	cachedCoins := make(map[string]schema.Coins)
	ctx := context.Background()

	// 使用管道批量获取缓存数据
	pipe := r.redisClient.Client.Pipeline()
	cmds := make(map[string]*redis.StringCmd)

	for _, id := range ids {
		cacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, id)
		cmds[id] = pipe.Get(ctx, cacheKey)
	}

	// 执行管道命令
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return nil, err
	}

	// 处理管道返回结果
	for id, cmd := range cmds {
		cachedData, err := cmd.Result()
		if err == nil {
			var coin schema.Coins
			if err := json.Unmarshal([]byte(cachedData), &coin); err == nil {
				cachedCoins[id] = coin
			} else {
				missingIDs = append(missingIDs, id)
			}
		} else {
			missingIDs = append(missingIDs, id)
		}
	}

	// 将缓存中的数据加入结果
	for _, coin := range cachedCoins {
		if coin.ReturnCoinsId != nil && *coin.ReturnCoinsId != "" {
			// 如果 ReturnCoinsId 不为空，添加对应的 Coin
			returnCoin, err := r.GetCoinsByOneID(*coin.ReturnCoinsId)
			if err == nil && returnCoin != nil {
				returnCoin.ID = coin.ID // 替换 ID
				coins = append(coins, *returnCoin)
				// 更新缓存
				newCacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, coin.ID)
				data, _ := json.Marshal(returnCoin)
				r.redisClient.Client.Set(context.Background(), newCacheKey, data, cacheDuration)
			}
		} else {
			coins = append(coins, coin)
		}
	}

	return coins, nil
}

func (r *coinRepository) GetCoinsByOneID(id string) (*schema.Coins, error) {
	var coin schema.Coins
	cacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, id)

	// 尝试从缓存中获取数据
	if cachedData, err := r.redisClient.Client.Get(context.Background(), cacheKey).Result(); err == nil {
		if err := json.Unmarshal([]byte(cachedData), &coin); err == nil {
			if coin.ReturnCoinsId != nil && *coin.ReturnCoinsId != "" {
				returnCoin, err := r.GetCoinsByOneID(*coin.ReturnCoinsId)
				if err != nil || returnCoin == nil || returnCoin.Address == "" || returnCoin.ChainID == "" {
					return nil, err
				}
				returnCoin.ID = coin.ID // 替换 ID
				// 更新缓存
				newCacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, coin.ID)
				data, _ := json.Marshal(returnCoin)
				r.redisClient.Client.Set(context.Background(), newCacheKey, data, cacheDuration)
				return returnCoin, nil
			}
			return &coin, nil
		}
	}

	if coin.ReturnCoinsId != nil && *coin.ReturnCoinsId != "" {
		returnCoin, err := r.GetCoinsByOneID(*coin.ReturnCoinsId)
		if err != nil || returnCoin == nil || returnCoin.Address == "" || returnCoin.ChainID == "" {
			return nil, err
		}
		returnCoin.ID = coin.ID // 替换 ID
		// 更新缓存
		newCacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, coin.ID)
		data, _ := json.Marshal(returnCoin)
		r.redisClient.Client.Set(context.Background(), newCacheKey, data, cacheDuration)
		return returnCoin, nil
	}

	return &coin, nil
}
func (r *coinRepository) cacheCoinBatch(coins []schema.Coins) {
	ctx := context.Background()
	var returnCoins []schema.Coins
	for i := 0; i < len(coins); i += cacheBatchSize {
		end := i + cacheBatchSize
		if end > len(coins) {
			end = len(coins)
		}
		batch := coins[i:end]

		for retries := 0; retries < cacheMaxRetries; retries++ {
			pipe := r.redisClient.Client.Pipeline()
			success := true
			for _, coin := range batch {
				if coin.ChainID == "" || coin.Address == "" {
					continue
				}
				// 如果 ReturnCoinsId 不为空，将其保存在 returnCoins 列表中
				if coin.ReturnCoinsId != nil && *coin.ReturnCoinsId != "" {
					returnCoins = append(returnCoins, coin)
				}
				coinCacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, coin.ID)
				if coinData, err := json.Marshal(coin); err == nil {
					pipe.Set(ctx, coinCacheKey, coinData, cacheDuration)
				} else {
					r.logger.Error().Err(err).Msgf("序列化单个币种失败: %s", coin.ID)
					success = false
					break
				}
			}

			if !success {
				break
			}
			_, err := pipe.Exec(ctx)
			if err != nil {
				r.logger.Error().Err(err).Msgf("执行管道命令失败，重试次数: %d", retries+1)
			} else {
				break
			}
		}
	}
	// 处理包含 ReturnCoinsId 的 coins
	for _, coin := range returnCoins {
		if coin.ReturnCoinsId != nil && *coin.ReturnCoinsId != "" {
			returnCoin, err := r.GetCoinsByOneID(*coin.ReturnCoinsId)
			if err != nil {
				r.logger.Error().Err(err).Msgf("获取返回币种失败: %s", *coin.ReturnCoinsId)
				continue
			}

			returnCoin.ID = coin.ID // 替换 ID
			newCacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, coin.ID)
			data, _ := json.Marshal(returnCoin)
			err = r.redisClient.Client.Set(ctx, newCacheKey, data, cacheDuration).Err()
			if err != nil {
				r.logger.Error().Err(err).Msgf("更新缓存失败: %s", coin.ID)
			}
		}
	}
}

func (r *coinRepository) RefreshCoinListCache(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	var coins []schema.Coins
	// 从数据库中批量获取数据
	if err := r.db.DB.Where("id IN ? AND deleted_at IS NULL", ids).Find(&coins).Error; err != nil {
		return err
	}

	// 更新缓存
	r.cacheCoinBatch(coins)
	return nil
}

// 刷新所有币种的缓存
func (r *coinRepository) RefreshAllCoinsCache() error {
	var coins []schema.Coins

	// 执行更新语句
	if err := r.db.DB.Exec("UPDATE coins SET price_source = 'coingecko' WHERE coingecko_coin_id IS NOT NULL").Error; err != nil {
		return err
	}

	// 从数据库中获取数据
	if err := r.db.DB.Where("deleted_at IS NULL").Find(&coins).Error; err != nil {
		return err
	}
	//存入缓存
	r.cacheCoinBatch(coins)
	return nil
}

// 将 coin 添加到队列
func (r *coinRepository) AddToQueue(coins []schema.Coins) error {
	pipe := r.redisClient.Client.Pipeline()

	for _, coin := range coins {
		if coin.ChainID == "" || coin.Address == "" || coin.ID == "" {
			continue
		}
		setKey := fmt.Sprintf("%s%s", setKeyPrefix, coin.ID)

		// 检查 coin 是否已在集合中
		exists := pipe.SIsMember(context.Background(), setKey, coin.ID)
		if exists.Val() {
			r.logger.Debug().Msgf("币种已在队列中: %s", coin.ID)
			continue
		}

		// 添加 coin 到集合中
		pipe.SAdd(context.Background(), setKey, coin.ID)

		// 将 coin 添加到队列
		data, err := json.Marshal(coin)
		if err != nil {
			return err
		}

		pipe.RPush(context.Background(), queueKey, data)
	}

	// 执行管道中的命令
	_, err := pipe.Exec(context.Background())
	if err != nil {
		return err
	}

	// 检查队列长度，如果达到队列大小则处理队列
	queueLen, err := r.redisClient.Client.LLen(context.Background(), queueKey).Result()
	if err != nil {
		return err
	}

	if queueLen >= int64(queueRunSize) {
		err = r.ProcessQueue()
		if err != nil {
			return err
		}
	}

	return nil
}

// 处理队列中的 coins
func (r *coinRepository) ProcessQueue() error {
	lockKey := "lock:coins_queue"
	// 尝试获取锁，确保多个 Pod 之间对队列的操作是互斥的
	for attempt := 1; attempt <= lockRetryCount; attempt++ {
		ok := r.redisClient.AcquireLock(lockKey, lockTTL)
		if ok {
			defer r.redisClient.ReleaseLock(lockKey)
			return r.processQueueWithTransaction()
		}
		time.Sleep(lockRetryInterval)
	}

	return fmt.Errorf("failed to acquire lock after %d attempts", lockRetryCount)

}

// processQueueWithTransaction 处理队列中的 coins，使用事务确保操作的原子性
func (r *coinRepository) processQueueWithTransaction() error {
	ctx := context.Background()

	// 获取队列中的 coins
	coinsData, err := r.redisClient.Client.LRange(ctx, queueKey, 0, -1).Result()
	if err != nil {
		return err
	}

	// 检查队列是否为空
	if len(coinsData) == 0 {
		return nil // 队列为空，直接返回
	}
	defer func() {
		// 清空 Redis 队列
		r.redisClient.Client.Del(context.Background(), queueKey).Err()
		r.redisClient.DeleteKeysByPrefix(setKeyPrefix)
	}()
	// 解析 coins 数据并去重
	coinsMap := make(map[string]schema.Coins)
	for _, coinData := range coinsData {
		var coin schema.Coins
		if err := json.Unmarshal([]byte(coinData), &coin); err != nil {
			return err
		}
		// 使用 coin.ID 作为键来去重
		coinsMap[coin.ID] = coin
	}

	// 将去重后的 coins 存入切片
	var coins []schema.Coins
	for _, coin := range coinsMap {
		coins = append(coins, coin)
	}

	// 重试机制，处理可能的死锁和其他临时性错误
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = r.upsertCoinsTransaction(coins)
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

// upsertCoinsTransaction 在事务中批量插入或更新 coins
func (r *coinRepository) upsertCoinsTransaction(coins []schema.Coins) error {

	err := r.UpsertCoins(coins)
	if err != nil {
		return err
	}

	// 清空队列
	err = r.redisClient.Client.Del(context.Background(), queueKey).Err()
	if err != nil {
		return err
	}

	r.logger.Debug().Msg("coinRepository 处理队列成功")
	return nil
}
func (r *coinRepository) DeleteCoinByID(id string) error {
	tx := r.db.DB.Begin()

	// 删除数据库记录
	if err := tx.Where("id = ?", id).Delete(&schema.Coins{}).Error; err != nil {
		tx.Rollback()
		return err
	}

	// 删除缓存
	cacheKey := fmt.Sprintf("%s%s", cacheKeyPrefix, id)
	if err := r.redisClient.Client.Del(context.Background(), cacheKey).Err(); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// 实现 CoinChecker 接口的方法
func (r *coinRepository) CheckCoinExists(coinID string) (bool, error) {
	coin, err := r.GetCoinsByOneID(coinID)
	if err != nil {
		return false, err
	}
	return coin != nil && coin.ID != "", nil
}
