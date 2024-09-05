package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/knadh/koanf/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type PriceService interface {
	GetPrice(chainId, address, symbol, network string, useCache bool, excludeRoute bool) (*string, error)
	GetHistoricalPrice(chainId, address, symbol, network string, unixTimeStamp int64) (*string, error)
	GetBatchPrice(ctx context.Context, chainIds []string, addresses []string, symbols []string, networks []string, useCache bool, excludeRoute bool) ([]PriceResult, error)
	GetBatchHistoricalPrice(chainIds []string, addresses []string, symbols []string, networks []string, unixTimeStamp []int64, datesStr []string) ([]PriceResult, error)
}

type priceService struct {
	coinGeckoService        CoinGeckoService
	geckoTerminalService    GeckoTerminalService
	coinGeckoOnChainService CoinGeckoOnChainService
	defiLlamaService        DefiLlamaService
	dodoexRouteService      DodoexRouteService
	coinRepository          repository.CoinRepository
	throttler               *shared.CoinsThrottler
	slack                   SlackNotificationService
	redisClient             *shared.RedisClient
	logger                  zerolog.Logger

	keysRequestIDMap            sync.Map      // key: priceKey, value: requestIDs
	requestIDChannelMap         sync.Map      // key: requestID, value: channel
	requestIDKeysMap            sync.Map      // key: requestID, map: key => priceKey v 1
	processTime                 time.Duration //拉取任务间隔
	processTimeOut              time.Duration //任务超时
	prohibitedSourcesCurrent    map[string]bool
	prohibitedSourcesHistorical map[string]bool
	fetchSize                   int64 //每次从redis 取多少
	batchSize                   int64 //每个协程处理多少
}

func NewPriceService(cfg *koanf.Koanf, slack SlackNotificationService, coinGeckoService CoinGeckoService, geckoTerminalService GeckoTerminalService, defiLlamaService DefiLlamaService, dodoexRouteService DodoexRouteService, coinGeckoOnChainService CoinGeckoOnChainService, coinRepository repository.CoinRepository, logger zerolog.Logger, throttler *shared.CoinsThrottler, redisClient *shared.RedisClient) PriceService {
	// 读取当前价格禁止数据源配置
	prohibitedCurrent := cfg.MapKeys("prohibitedSources.current")
	prohibitedSourcesCurrent := make(map[string]bool, len(prohibitedCurrent))
	if len(prohibitedCurrent) > 0 {
		for _, v := range prohibitedCurrent {
			prohibitedSourcesCurrent[v] = true
		}
	}
	prohibitedHistorical := cfg.MapKeys("prohibitedSources.historical")
	prohibitedSourcesHistorical := make(map[string]bool, len(prohibitedHistorical))
	if len(prohibitedHistorical) > 0 {
		for _, v := range prohibitedHistorical {
			prohibitedSourcesHistorical[v] = true
		}
	}
	processTime := cfg.Duration("price.processTime")
	if processTime == 0 {
		processTime = 10 * time.Millisecond
	}
	processTimeOut := cfg.Duration("price.processTimeOut")
	if processTimeOut == 0 {
		processTimeOut = 15 * time.Second
	}
	fetchSize := cfg.Int64("price.fetchSize")
	if fetchSize == 0 {
		fetchSize = 2000
	}

	batchSize := cfg.Int64("price.batchSize")
	if batchSize == 0 {
		batchSize = 200
	}
	// processTime = 20 * time.Second
	// processTimeOut = 30 * time.Second
	// fetchSize = 200
	// batchSize = 1
	s := &priceService{
		coinGeckoService:            coinGeckoService,
		geckoTerminalService:        geckoTerminalService,
		coinGeckoOnChainService:     coinGeckoOnChainService,
		defiLlamaService:            defiLlamaService,
		dodoexRouteService:          dodoexRouteService,
		coinRepository:              coinRepository,
		throttler:                   throttler,
		redisClient:                 redisClient,
		slack:                       slack,
		processTime:                 processTime,    // 设置任务拉取时间
		processTimeOut:              processTimeOut, // 设置任务执行超时时间
		prohibitedSourcesCurrent:    prohibitedSourcesCurrent,
		prohibitedSourcesHistorical: prohibitedSourcesHistorical,
		fetchSize:                   fetchSize,
		batchSize:                   batchSize,
	}
	go s.ProcessPriceRequests()
	go s.startResultSubscriber("price_results_channel")
	return s
}

// 生成哈希键
func generateHashKey(parts ...string) string {
	data := strings.Join(parts, "_")
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *priceService) GetPrice(chainId, address, symbol, network string, useCache bool, excludeRoute bool) (*string, error) {
	if !useCache {
		// 直接调用 FetchAndProcessBatchPrices 方法
		results, err := s.FetchAndProcessBatchPrices(context.Background(), []string{chainId}, []string{address}, []string{safeDereferenceString(&symbol, "")}, []string{network}, useCache, excludeRoute)
		if err != nil {
			return nil, err
		}
		if len(results) > 0 {
			return results[0].Price, nil
		}
		return nil, nil
	}
	address = strings.ToLower(address)
	// 使用缓存
	requestKey := generateHashKey(chainId, strings.ToLower(address), safeDereferenceString(&symbol, ""), network)
	if value, found := s.WaitForResult(chainId, strings.ToLower(address)); found && value != nil && *value != "" {
		return value, nil
	}
	requestID := generateRequestID(chainId, address, safeDereferenceString(&symbol, ""), network)

	resultChannel := make(chan string, 1)
	defer func() {
		s.requestIDChannelMap.Delete(requestID)
		s.requestIDKeysMap.Delete(requestID)
		close(resultChannel)
	}()
	s.requestIDChannelMap.Store(requestID, resultChannel)

	ctx := context.Background()
	requestIDs, ok := s.keysRequestIDMap.Load(requestKey)
	requestKeyMap := make(map[string]bool)
	requestKeyMap[requestKey] = true
	s.requestIDKeysMap.Store(requestID, requestKeyMap)
	if ok {
		// 将返回的值断言为 []string 类型
		requestIDSlice := requestIDs.([]string)
		// 使用 append 添加新的值
		requestIDSlice = append(requestIDSlice, requestID)
		// 将更新后的切片存回 sync.Map
		s.keysRequestIDMap.Store(requestKey, requestIDSlice)
	} else {
		s.keysRequestIDMap.Store(requestKey, []string{requestID})
	}
	requestInfo := fmt.Sprintf("%s|%s|%s|%s|%s", requestKey, chainId, address, safeDereferenceString(&symbol, ""), network)
	_, err := s.EnqueueUniqueRequest(ctx, "unique_price_requests", "price_requests_queue", requestKey, requestInfo)
	if err != nil {
		return nil, err
	}
	// 从通道中获取
	select {
	case flag := <-resultChannel:
		if flag == "ok" {
			// 等待一批结果
			if value, found := s.WaitForResult(chainId, strings.ToLower(address)); found {
				if (value == nil || *value == "") && !excludeRoute {
					results, err := s.FetchAndProcessBatchPrices(context.Background(), []string{chainId}, []string{address}, []string{safeDereferenceString(&symbol, "")}, []string{network}, useCache, excludeRoute)
					if err != nil {
						return nil, err
					}
					if len(results) > 0 {
						return results[0].Price, nil
					}
				}
				return value, nil
			}
		}
	case <-time.After(s.processTimeOut):
		// 超时逻辑
		s.logger.Warn().Msgf("GetPrice timed out after 5 seconds for chainId: %s, address: %s", chainId, address)
		// 直接调用 FetchAndProcessBatchPrices 方法
		results, err := s.FetchAndProcessBatchPrices(context.Background(), []string{chainId}, []string{address}, []string{safeDereferenceString(&symbol, "")}, []string{network}, useCache, excludeRoute)
		if err != nil {
			return nil, err
		}
		if len(results) > 0 {
			return results[0].Price, nil
		}
		return nil, nil
	}
	return nil, nil
}

func (s *priceService) GetBatchPrice(ctx context.Context, chainIds []string, addresses []string, symbols []string, networks []string, isCache bool, excludeRoute bool) ([]PriceResult, error) {
	if !isCache || !excludeRoute {
		// 直接调用 FetchAndProcessBatchPrices 方法
		return s.FetchAndProcessBatchPrices(ctx, chainIds, addresses, symbols, networks, isCache, excludeRoute)
	}

	requestID := generateRequestsID(chainIds, addresses, symbols, networks)
	resultChannel := make(chan string, 1)
	defer func() {
		s.requestIDChannelMap.Delete(requestID)
		s.requestIDKeysMap.Delete(requestID)
		close(resultChannel)
	}()
	// 保存 requestID 和 requestKeys 映射
	requestKeys := make([]string, len(chainIds))
	s.requestIDChannelMap.Store(requestID, resultChannel)

	requestKeyMap := make(map[string]bool)
	resultMap := make(map[int]*string)
	requestInfos := make([]string, len(chainIds))

	pipe := s.redisClient.Client.Pipeline()                  // 创建 Redis 管道
	resultFutures := make([]*redis.StringCmd, len(chainIds)) // 保存管道的未来结果

	for i := 0; i < len(chainIds); i++ {
		resultKey := fmt.Sprintf("price_result:%s", strings.Join([]string{chainIds[i], strings.ToLower(addresses[i])}, "_"))
		resultFutures[i] = pipe.Get(context.Background(), resultKey)
	}
	// 执行 Redis 管道
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		s.logger.Err(err).Msg("Failed to execute Redis pipeline")
		return nil, err
	}

	// 处理管道的结果
	for i := 0; i < len(chainIds); i++ {
		price, err := resultFutures[i].Result()
		if err == nil && price != "-1" {
			resultMap[i] = &price
		} else if price == "-1" {
			resultMap[i] = nil
		} else {
			// 将需要发送到 Redis 的请求记录下来
			symbol := GetOrDefault(symbols, i, "")
			network := GetOrDefault(networks, i, "")
			requestKey := generateHashKey(chainIds[i], strings.ToLower(addresses[i]), symbol, network)
			requestKeys[i] = requestKey
			requestKeyMap[requestKey] = true

			requestIDs, ok := s.keysRequestIDMap.Load(requestKey)
			if ok {
				// 将返回的值断言为 []string 类型
				requestIDSlice := requestIDs.([]string)
				// 使用 append 添加新的值
				requestIDSlice = append(requestIDSlice, requestID)
				// 将更新后的切片存回 sync.Map
				s.keysRequestIDMap.Store(requestKey, requestIDSlice)
			} else {
				s.keysRequestIDMap.Store(requestKey, []string{requestID})
			}
			requestInfo := fmt.Sprintf("%s|%s|%s|%s|%s", requestKey, chainIds[i], strings.ToLower(addresses[i]), symbol, network)
			requestInfos[i] = requestInfo
		}
	}
	s.requestIDKeysMap.Store(requestID, requestKeyMap)
	// 执行管道中的命令
	newRequestsCount, err := s.EnqueueUniqueRequests(ctx, "unique_price_requests", "price_requests_queue", requestKeys, requestInfos)
	if err != nil {
		return nil, err
	}
	if newRequestsCount > 0 {
		s.logger.Debug().Msgf("%d new unique requests enqueued successfully", newRequestsCount)
	}
	if len(resultMap) == len(chainIds) {
		results, err := s.WaitForBatchResults(chainIds, addresses, symbols, networks, resultMap)
		if err != nil {
			return nil, err
		}

		if !excludeRoute {
			run := false
			for _, result := range results {
				if result.Price == nil || *result.Price == "" {
					run = true
				}
			}
			if run {
				return s.FetchAndProcessBatchPrices(ctx, chainIds, addresses, symbols, networks, isCache, excludeRoute)
			}
		}
		return results, nil
	} else {
		// 从通道中获取
		select {
		case flag := <-resultChannel:
			if flag == "ok" {
				// 等待一批结果
				results, err := s.WaitForBatchResults(chainIds, addresses, symbols, networks, resultMap)
				if err != nil {
					return nil, err
				}

				if !excludeRoute {
					run := false
					for _, result := range results {
						if result.Price == nil || *result.Price == "" {
							run = true
						}
					}
					if run {
						return s.FetchAndProcessBatchPrices(ctx, chainIds, addresses, symbols, networks, isCache, excludeRoute)
					}
				}
				return results, nil
			}
		case <-time.After(s.processTimeOut):
			// 超时逻辑
			s.logger.Warn().Msg("GetBatchPrice timed out after 5 seconds")
			return s.FetchAndProcessBatchPrices(ctx, chainIds, addresses, symbols, networks, isCache, excludeRoute)
		}
	}

	return nil, nil
}

func (s *priceService) WaitForBatchResults(chainIds []string, addresses []string, symbols []string, networks []string, resultMap map[int]*string) ([]PriceResult, error) {
	results := make([]PriceResult, len(chainIds))
	pipe := s.redisClient.Client.Pipeline()                  // 创建 Redis 管道
	resultFutures := make([]*redis.StringCmd, len(chainIds)) // 保存未来结果的引用

	// 构造请求并添加到管道
	shouldExecutePipeline := false // 标记是否需要执行 Redis 管道

	for idx, address := range addresses {
		if result, ok := resultMap[idx]; ok {
			// 如果 resultMap 中已经有结果，直接使用
			results[idx] = PriceResult{
				ChainID:   chainIds[idx],
				Address:   addresses[idx],
				Price:     result,
				Symbol:    GetOrNil(symbols, idx),
				Network:   GetOrNil(networks, idx),
				TimeStamp: strconv.FormatInt(time.Now().Unix(), 10),
				Serial:    idx,
			}
		} else {
			// 如果没有缓存结果，将请求添加到 Redis 管道中
			resultKey := fmt.Sprintf("price_result:%s_%s", chainIds[idx], strings.ToLower(address))
			resultFutures[idx] = pipe.Get(context.Background(), resultKey)
			shouldExecutePipeline = true // 标记需要执行管道
		}
	}

	// 只有在需要时才执行 Redis 管道
	if shouldExecutePipeline {
		_, err := pipe.Exec(context.Background())
		if err != nil && err != redis.Nil {
			s.logger.Err(err).Msg("Failed to execute Redis pipeline")
			return nil, err
		}
	}

	// 获取管道执行的结果
	for idx, cmd := range resultFutures {
		if cmd == nil {
			// 如果该项已经从缓存中获取，则继续
			continue
		}
		price, err := cmd.Result()
		if err != nil {
			s.logger.Debug().Err(err).Msgf("Failed to get price for key %s", fmt.Sprintf("price_result:%s_%s", chainIds[idx], strings.ToLower(addresses[idx])))
		}
		if price != "-1" {
			// 正常获取价格
			results[idx] = PriceResult{
				ChainID:   chainIds[idx],
				Address:   addresses[idx],
				Price:     &price,
				Symbol:    GetOrNil(symbols, idx),
				Network:   GetOrNil(networks, idx),
				TimeStamp: strconv.FormatInt(time.Now().Unix(), 10),
				Serial:    idx,
			}
		} else {
			results[idx] = PriceResult{
				ChainID:   chainIds[idx],
				Address:   addresses[idx],
				Price:     nil,
				Symbol:    GetOrNil(symbols, idx),
				Network:   GetOrNil(networks, idx),
				TimeStamp: "0",
				Serial:    idx,
			}
		}
	}

	return results, nil
}

func (s *priceService) WaitForResult(parts ...string) (*string, bool) {
	// 先检查缓存
	resultKey := fmt.Sprintf("price_result:%s", strings.Join(parts, "_"))
	price, err := s.redisClient.Client.Get(context.Background(), resultKey).Result()
	if err == nil && price != "-1" {
		return &price, true
	}
	if price == "-1" {
		return nil, true
	}
	return nil, false
}

func (s *priceService) ProcessPriceRequests() {
	time.Sleep(3 * time.Second) // 初始化5秒等待
	ctx := context.Background()
	// Lua 脚本：原子性地获取请求并清理
	luaScript := `
	    local requests = redis.call('LRANGE', KEYS[1], 0, tonumber(ARGV[1]) - 1)
	    if #requests > 0 then
	        redis.call('LTRIM', KEYS[1], tonumber(ARGV[1]), -1)
	        for i = 1, #requests do
	            local requestKey = requests[i]:match("([^|]+)")
	            if requestKey then
	                redis.call('SREM', KEYS[2], requestKey)
	            end
	        end
	    end
	    return requests
	    `
	// 加载 Lua 脚本
	// 检查脚本是否已经存在，避免重复加载
	sha, err := s.redisClient.Client.ScriptLoad(ctx, luaScript).Result()
	if err != nil {
		s.logger.Err(err).Msg("Failed to load Lua script")
		return
	}

	for {
		time.Sleep(s.processTime) // 每 10 毫秒检查一次队列
		// 使用 Lua 脚本从 Redis 列表中获取指定数量的数据，并将这些数据从列表中移除
		result, err := s.redisClient.Client.EvalSha(ctx, sha, []string{"price_requests_queue", "unique_price_requests"}, s.fetchSize).Result()
		if err != nil {
			s.logger.Err(err).Msg("Failed to execute Lua script")
			continue
		}

		requests := result.([]interface{})
		if len(requests) == 0 {
			continue
		}

		// 将获取的请求切分为每组指定数量的数据，并通过协程处理
		numWorkers := (len(requests) + int(s.batchSize) - 1) / int(s.batchSize)

		var wg sync.WaitGroup
		wg.Add(numWorkers)

		for i := 0; i < numWorkers; i++ {
			start := i * int(s.batchSize)
			end := start + int(s.batchSize)
			if end > len(requests) {
				end = len(requests)
			}

			go func(requestBatch []interface{}) {
				defer wg.Done()

				var requestKeys, chainIds, addresses, symbols, networks []string

				// 解析每个请求并提取参数
				for _, req := range requestBatch {
					parts := strings.Split(req.(string), "|")
					if len(parts) == 5 {
						chainIds = append(chainIds, parts[1])
						addresses = append(addresses, parts[2])
						symbols = append(symbols, parts[3])
						networks = append(networks, parts[4])
						requestKeys = append(requestKeys, parts[0])
					} else {
						s.logger.Warn().Msgf("Invalid request format: %s", req.(string))
					}
				}

				// 批量处理请求
				results, err := s.FetchAndProcessBatchPrices(ctx, chainIds, addresses, symbols, networks, true, true)
				if err != nil {
					s.logger.Err(err).Msg("Failed to fetch batch prices")
					return
				}

				// 将结果缓存并通知生产者
				for i, result := range results {
					resultKey := fmt.Sprintf("price_result:%s_%s", result.ChainID, result.Address)
					if err := s.redisClient.Client.Set(ctx, resultKey, safeDereferenceString(result.Price, "-1"), time.Minute*5).Err(); err != nil {
						s.logger.Err(err).Msg("Failed to cache price result")
						continue
					}

					// 通知生产者，传递哈希键
					s.NotifyProducer("price_results_channel", resultKey, requestKeys[i])
				}
			}(requests[start:end])
		}

		// 等待所有协程完成处理
		wg.Wait()
	}
}

func (s *priceService) NotifyProducer(channel, resultKey, requestKey string) {
	// 发布结果到固定的 channel，并包含哈希键
	ctx := context.Background()
	message := fmt.Sprintf("%s|%s", resultKey, requestKey)
	s.redisClient.Client.Publish(ctx, channel, message)
}

func (s *priceService) startResultSubscriber(channelName string) {
	time.Sleep(3 * time.Second) // 延迟3秒进行
	ctx := context.Background()
	pubsub := s.redisClient.Client.Subscribe(ctx, channelName)
	defer pubsub.Close()

	for {
		msg, err := pubsub.ReceiveMessage(ctx)
		if err != nil {
			s.logger.Err(err).Msgf("Error receiving message from channel: %s", channelName)
			continue
		}

		// 使用协程处理每个消息
		go s.processMessage(msg)
	}
}

func (s *priceService) processMessage(msg *redis.Message) {
	parts := strings.Split(msg.Payload, "|")
	if len(parts) != 2 {
		return
	}

	requestKey := parts[1]
	requestIDs, ok := s.keysRequestIDMap.Load(requestKey)
	if !ok {
		return
	}

	// 使用一个等待组同步多个 requestID 的处理
	var wg sync.WaitGroup
	var mu sync.Mutex // 锁定用于保护对 map 的访问
	// 删除已处理的 requestKey
	defer s.keysRequestIDMap.Delete(requestKey)
	for _, requestId := range requestIDs.([]string) {
		wg.Add(1)

		// 处理每个 requestId，但不再启动新的协程
		go func(requestId string) {
			defer wg.Done()

			// 获取并更新 requestIDKeysMap
			requestKeyMap, ok := s.requestIDKeysMap.Load(requestId)
			if ok {
				mu.Lock() // 在删除操作前锁定
				keyMap := requestKeyMap.(map[string]bool)

				delete(keyMap, requestKey) // 安全删除操作

				if len(keyMap) == 0 {
					// 如果 requestKeyMap 为空，通知相应的 channel
					if channel, ok := s.requestIDChannelMap.Load(requestId); ok {
						channel.(chan string) <- "ok"
					}
				}
				mu.Unlock() // 删除操作后解锁
			}
		}(requestId)
	}
	// 等待所有 requestID 处理完成
	wg.Wait()
}

func (s *priceService) FetchAndProcessBatchPrices(ctx context.Context, chainIds []string, addresses []string, symbols []string, networks []string, isCache bool, excludeRoute bool) ([]PriceResult, error) {
	// 在执行耗时操作前和期间检查 ctx 的状态
	lowerAddresses := make([]string, len(addresses))
	resultsMap := make(map[string]PriceResult)
	for i, addr := range addresses {
		lowerAddresses[i] = strings.ToLower(addr)
	}

	// 根据地址和链ID获取coin数据
	ids := make([]string, len(addresses))
	idToIndexMap := make(map[string]int)
	for i, addr := range lowerAddresses {
		id := chainIds[i] + "_" + addr
		ids[i] = id
		idToIndexMap[id] = i
	}

	coins, err := s.coinRepository.GetCoinsByID(ids)
	if err != nil {
		return nil, err
	}

	// 转换coins为map
	coinMap := make(map[string]schema.Coins)
	//关联coins
	retrunCoinMap := make(map[string]string)
	retrunCoinToMap := make(map[string][]string) // 修改为保存切片
	for _, coin := range coins {
		coinMap[coin.ID] = coin
		returnCoinId := fmt.Sprintf("%s_%s", coin.ChainID, coin.Address)
		if coin.ID != "" && coin.ChainID != "" && coin.Address != "" && coin.ID != returnCoinId {
			retrunCoinMap[coin.ID] = returnCoinId
			retrunCoinToMap[returnCoinId] = append(retrunCoinToMap[returnCoinId], coin.ID) // 追加到切片
		}
	}
	geckoIDsSetSource := make(map[string]struct{})
	terminalIDsSetSource := make(map[string]struct{})
	llamaIDsSetSource := make(map[string]struct{})
	geckoOnChainIDsSetSource := make(map[string]struct{})
	// 初始化来源映射
	geckoIDsSet := make(map[string]struct{})
	terminalIDsSet := make(map[string]struct{})
	llamaIDsSet := make(map[string]struct{})
	dodoexIDsSet := make(map[string]struct{})
	geckoOnChainIDsSet := make(map[string]struct{})
	// 填充映射
	for _, id := range ids {
		parts := strings.Split(id, "_")
		coinID := fmt.Sprintf("%s_%s", parts[0], parts[1])
		if coin, exists := coinMap[coinID]; exists {
			if coin.PriceSource != nil && (*coin.PriceSource == "coingecko") {
				geckoIDsSetSource[id] = struct{}{}
			} else if coin.PriceSource != nil && *coin.PriceSource == "geckoterminal" {
				terminalIDsSetSource[id] = struct{}{}
			} else if coin.PriceSource != nil && *coin.PriceSource == "coinGeckoOnChain" {
				geckoOnChainIDsSetSource[id] = struct{}{}
			} else if coin.LastPriceSource != nil && *coin.LastPriceSource == "coingecko" {
				geckoIDsSetSource[id] = struct{}{}
			} else if coin.LastPriceSource != nil && *coin.LastPriceSource == "geckoterminal" {
				terminalIDsSetSource[id] = struct{}{}
			} else if coin.LastPriceSource != nil && *coin.LastPriceSource == "defillama" {
				llamaIDsSetSource[id] = struct{}{}
			} else if coin.LastPriceSource != nil && *coin.LastPriceSource == "coinGeckoOnChain" {
				geckoOnChainIDsSetSource[id] = struct{}{}
			}
			geckoIDsSet[id] = struct{}{}
		}
		terminalIDsSet[id] = struct{}{}
		llamaIDsSet[id] = struct{}{}
		geckoOnChainIDsSet[id] = struct{}{}
		if !excludeRoute {
			dodoexIDsSet[id] = struct{}{}
		}
	}
	// 批量查询指定数据源的历史价格
	batchQuery := func(source string, idSet map[string]struct{}) {
		if prohibited, ok := s.prohibitedSourcesCurrent[source]; ok && prohibited {
			return
		}
		var bChainIds, bAddresses, bSymbols, bNetworks []string

		for id := range idSet {
			index := idToIndexMap[id]
			if s.throttler.IsCoinsThrottled(id) {
				resultsMap[id] = PriceResult{ChainID: chainIds[index], Address: lowerAddresses[index], Price: nil, Symbol: GetOrNil(symbols, index), Network: GetOrNil(networks, index), TimeStamp: "0"}
				s.slack.SaveLog(context.Background(), "priceService-GetBatchPrice", chainIds[index], lowerAddresses[index], time.Now().Format("2006-01-02"), time.Now().Unix())
				continue
			}
			if retrunCoinId, exists := retrunCoinMap[id]; exists {
				bChainIds = append(bChainIds, strings.Split(retrunCoinId, "_")[0])
				bAddresses = append(bAddresses, strings.Split(retrunCoinId, "_")[1])
			} else {
				bChainIds = append(bChainIds, chainIds[index])
				bAddresses = append(bAddresses, lowerAddresses[index])
			}
			bSymbols = append(bSymbols, GetOrDefault(symbols, index, ""))
			bNetworks = append(bNetworks, GetOrDefault(networks, index, ""))
		}

		// 根据 source 调用不同的数据源方法
		var results []PriceResult
		var err error
		switch source {
		case "coingecko":
			results, err = s.coinGeckoService.GetBatchPrice(bAddresses, bChainIds, bSymbols, bNetworks, isCache)
		case "geckoterminal":
			results, err = s.geckoTerminalService.GetBatchCurrentPrices(bAddresses, bChainIds, bSymbols, bNetworks, isCache)
		case "defillama":
			results, err = s.defiLlamaService.GetBatchCurrentPrices(bAddresses, bChainIds, bSymbols, bNetworks, isCache)
		case "coinGeckoOnChain":
			results, err = s.coinGeckoOnChainService.GetBatchCurrentPricesOnChain(bAddresses, bChainIds, bSymbols, bNetworks, isCache)
		case "dodoexRoute":
			results, err = s.dodoexRouteService.GetBatchCurrentPrices(bAddresses, bChainIds, bSymbols, bNetworks, isCache)
		}

		if err == nil {
			for _, result := range results {
				key := result.ChainID + "_" + result.Address
				if result.Price != nil && *result.Price != "" {
					resultsMap[key] = result
					if coinIds, exists := retrunCoinToMap[key]; exists {
						for _, coinId := range coinIds {
							result.ChainID = strings.Split(coinId, "_")[0]
							result.Address = strings.Split(coinId, "_")[1]
							resultsMap[coinId] = result
						}
					}
					// 删除其他数据源集合中的 ID
					delete(geckoIDsSet, key)
					delete(terminalIDsSet, key)
					delete(llamaIDsSet, key)
					delete(dodoexIDsSet, key)
					delete(geckoOnChainIDsSet, key)
				}
			}
		} else {
			s.logger.Err(err).Msgf("GetBatchPrice 获取%s价格失败", source)
		}
	}
	//查询指定数据源
	if len(geckoIDsSetSource) > 0 {
		batchQuery("coingecko", geckoIDsSetSource)
	}
	if len(terminalIDsSetSource) > 0 {
		batchQuery("geckoterminal", terminalIDsSetSource)
	}
	if len(llamaIDsSetSource) > 0 {
		batchQuery("defillama", llamaIDsSetSource)
	}

	// 数据源顺序查询
	// 查询剩余的默认数据源
	if len(geckoIDsSet) > 0 {
		batchQuery("coingecko", geckoIDsSet)
	}
	if len(llamaIDsSet) > 0 {
		batchQuery("defillama", llamaIDsSet)
	}
	if len(terminalIDsSet) > 0 {
		batchQuery("geckoterminal", terminalIDsSet)
	}
	if len(geckoOnChainIDsSet) > 0 {
		batchQuery("coinGeckoOnChain", geckoOnChainIDsSet)
	}
	if len(dodoexIDsSet) > 0 {
		batchQuery("dodoexRoute", dodoexIDsSet)
	}

	results := make([]PriceResult, len(addresses))
	for i, addr := range addresses {
		key := chainIds[i] + "_" + strings.ToLower(addr)
		result, exists := resultsMap[key]
		if !exists {
			result = PriceResult{
				ChainID:   chainIds[i],
				Address:   addr,
				Price:     nil,
				Symbol:    GetOrNil(symbols, i),
				Network:   GetOrNil(networks, i),
				TimeStamp: "0",
				Serial:    i,
			}
		} else {
			result.Serial = i
			result.Symbol = GetOrNil(symbols, i)
			result.Network = GetOrNil(networks, i)
			result.Address = addr
		}
		if result.Price == nil || *result.Price == "" {
			status := "200"
			if result.RequestStatus != nil && *result.RequestStatus != "" {
				status = *result.RequestStatus
			}
			if s.throttler.CoinsThrottle(key, status) {
				s.slack.SaveLog(context.Background(), "priceService-GetBatchPrice", result.ChainID, result.Address, time.Now().Format("2006-01-02"), time.Now().Unix())
			}
		}
		results[i] = result
	}

	return results, nil
}

func (s *priceService) GetHistoricalPrice(chainId, address, symbol, network string, unixTimeStamp int64) (*string, error) {
	address = strings.ToLower(address)
	coin, err := s.coinRepository.GetCoinsByOneID(chainId + "_" + address)
	if err != nil {
		return nil, err
	}
	if coin != nil && coin.ChainID != "" && coin.Address != "" {
		chainId = coin.ChainID
		address = coin.Address
	}
	date := time.Unix(unixTimeStamp, 0).Format("02-01-2006")
	coindId := chainId + "_" + address + "_" + date
	// 如果被节流，直接返回 nil 价格
	if s.throttler.IsCoinsThrottled(coindId) {
		s.slack.SaveLog(context.Background(), "priceService-GetHistoricalPrice", chainId, address, time.Unix(unixTimeStamp, 0).Format("2006-01-02"), time.Now().Unix())
		return nil, nil
	}

	var price *string

	// 封装查询历史价格的逻辑
	getHistoricalPriceFromSource := func(source string) (*string, error) {
		if prohibited, ok := s.prohibitedSourcesHistorical[source]; ok && prohibited {
			return nil, nil
		}
		switch source {
		case "geckoterminal":
			return s.geckoTerminalService.GetHistoricalPrice(chainId, address, unixTimeStamp)
		case "coingecko":
			return s.coinGeckoService.GetSingleHistoricalPrice(unixTimeStamp, chainId, address, symbol, network)
		// case "coinGeckoOnChain":
		// 	return s.coinGeckoOnChainService.GetHistoricalPriceOnChain(chainId, address, unixTimeStamp)
		case "defillama":
			return s.defiLlamaService.GetHistoricalPrice(chainId, address, unixTimeStamp)
		default:
			return s.geckoTerminalService.GetHistoricalPrice(chainId, address, unixTimeStamp)
		}
	}
	if coin != nil && coin.PriceSource != nil && *coin.PriceSource != "" {
		price, err = getHistoricalPriceFromSource(*coin.PriceSource)
		if err == nil && price != nil {
			return price, nil
		} else {
			s.logger.Err(err).Msg("GetHistoricalPrice 获取价格失败 " + coindId)
		}
	}
	if coin != nil && coin.LastPriceSource != nil && *coin.LastPriceSource != "" && price == nil {
		price, err = getHistoricalPriceFromSource(*coin.LastPriceSource)
		if err == nil && price != nil {
			return price, nil
		} else {
			s.logger.Err(err).Msg("GetHistoricalPrice 获取价格失败 " + coindId)
		}
	}
	// 默认优先使用 coingecko，其次是 geckoterminal
	price, err = s.coinGeckoService.GetSingleHistoricalPrice(unixTimeStamp, chainId, address, symbol, network)
	if err == nil && price != nil {
		return price, nil
	} else {
		s.logger.Err(err).Msg("GetHistoricalPrice-coinGeckoService 获取价格失败 " + coindId)
	}

	price, err = s.defiLlamaService.GetHistoricalPrice(chainId, address, unixTimeStamp)
	if err == nil && price != nil {
		return price, nil
	} else {
		s.logger.Err(err).Msg("GetHistoricalPrice-defiLlamaService 获取价格失败 " + coindId)
	}

	price, err = s.geckoTerminalService.GetHistoricalPrice(chainId, address, unixTimeStamp)
	if err == nil && price != nil {
		return price, nil
	} else {
		s.logger.Err(err).Msg("GetHistoricalPrice-geckoTerminalService 获取价格失败 " + coindId)
	}

	// price, err = s.coinGeckoOnChainService.GetHistoricalPriceOnChain(chainId, address, unixTimeStamp)
	// if err == nil && price != nil {
	// 	return price, nil
	// } else {
	// 	s.logger.Err(err).Msg("GetHistoricalPrice-coinGeckoOnChainService 获取价格失败 " + coindId)
	// }

	// 如果价格为空，则设置节流并发送警告
	if price == nil {
		requestStatus := "200"
		if err != nil && strings.Contains(err.Error(), "429") {
			requestStatus = "429"
		}
		if s.throttler.CoinsThrottle(coindId, requestStatus) {
			s.slack.SaveLog(context.Background(), "priceService-GetHistoricalPrice", chainId, address, time.Unix(unixTimeStamp, 0).Format("2006-01-02"), time.Now().Unix())
		}
	}

	return nil, nil
}

func (s *priceService) GetBatchHistoricalPrice(chainIds []string, addresses []string, symbols []string, networks []string, unixTimeStamp []int64, datesStr []string) ([]PriceResult, error) {
	lowerAddresses := make([]string, len(addresses))
	for i, addr := range addresses {
		lowerAddresses[i] = strings.ToLower(addr)
	}

	ids := make([]string, len(addresses))
	idToIndexMap := make(map[string]int)
	for i, addr := range lowerAddresses {
		id := fmt.Sprintf("%s_%s_%d", chainIds[i], addr, unixTimeStamp[i])
		ids[i] = id
		idToIndexMap[id] = i
	}
	coinIds := make([]string, len(addresses))
	for i, addr := range lowerAddresses {
		coinIds[i] = fmt.Sprintf("%s_%s", chainIds[i], addr)
	}

	coins, err := s.coinRepository.GetCoinsByID(coinIds)
	if err != nil {
		return nil, err
	}

	// 转换coins为map
	coinMap := make(map[string]schema.Coins)
	//关联coins
	//关联coins
	retrunCoinMap := make(map[string]string)
	retrunCoinToMap := make(map[string][]string) // 修改为保存切片
	for _, coin := range coins {
		coinMap[coin.ID] = coin
		returnCoinId := fmt.Sprintf("%s_%s", coin.ChainID, coin.Address)
		if coin.ID != "" && coin.ChainID != "" && coin.Address != "" && coin.ID != returnCoinId {
			retrunCoinMap[coin.ID] = returnCoinId
			retrunCoinToMap[returnCoinId] = append(retrunCoinToMap[returnCoinId], coin.ID) // 追加到切片
		}
	}

	// 初始化来源映射
	geckoIDsSetSource := make(map[string]struct{})    // 用于去重
	terminalIDsSetSource := make(map[string]struct{}) // 用于去重
	llamaIDsSetSource := make(map[string]struct{})    // 用于去重

	geckoIDsSet := make(map[string]struct{})    // 用于去重
	terminalIDsSet := make(map[string]struct{}) // 用于去重
	llamaIDsSet := make(map[string]struct{})    // 用于去重

	// 填充映射
	for _, id := range ids {
		parts := strings.Split(id, "_")
		coinID := fmt.Sprintf("%s_%s", parts[0], parts[1])
		if coin, exists := coinMap[coinID]; exists {
			if coin.PriceSource != nil && (*coin.PriceSource == "coingecko") {
				geckoIDsSetSource[id] = struct{}{}
			} else if coin.PriceSource != nil && *coin.PriceSource == "geckoterminal" {
				terminalIDsSetSource[id] = struct{}{}
			} else if coin.LastPriceSource != nil && *coin.LastPriceSource == "coingecko" {
				geckoIDsSetSource[id] = struct{}{}
			} else if coin.LastPriceSource != nil && *coin.LastPriceSource == "geckoterminal" {
				terminalIDsSetSource[id] = struct{}{}
			} else if coin.LastPriceSource != nil && *coin.LastPriceSource == "defillama" {
				llamaIDsSetSource[id] = struct{}{}
			}
			geckoIDsSet[id] = struct{}{}
		}
		terminalIDsSet[id] = struct{}{}
		llamaIDsSet[id] = struct{}{}
	}

	resultsMap := make(map[string]PriceResult)

	// 批量查询指定数据源的历史价格
	batchQueryHistorical := func(source string, idSet map[string]struct{}) {
		if prohibited, ok := s.prohibitedSourcesHistorical[source]; ok && prohibited {
			return
		}
		var bChainIds, bAddresses, bSymbols, bNetworks []string
		var bUnixTimeStamps []int64

		for id := range idSet {
			index := idToIndexMap[id]
			if s.throttler.IsCoinsThrottled(id) {
				resultsMap[id] = PriceResult{
					ChainID:   chainIds[index],
					Address:   lowerAddresses[index],
					TimeStamp: datesStr[index],
					Price:     nil,
					Symbol:    GetOrNil(symbols, index),
					Network:   GetOrNil(networks, index),
				}
				s.slack.SaveLog(context.Background(), "priceService-GetBatchHistoricalPrice", chainIds[index], lowerAddresses[index], time.Unix(unixTimeStamp[index], 0).Format("2006-01-02"), time.Now().Unix())
				continue
			}

			coinId := chainIds[index] + "_" + lowerAddresses[index]
			if retrunCoinId, exists := retrunCoinMap[coinId]; exists {
				bChainIds = append(bChainIds, strings.Split(retrunCoinId, "_")[0])
				bAddresses = append(bAddresses, strings.Split(retrunCoinId, "_")[1])
			} else {
				bChainIds = append(bChainIds, chainIds[index])
				bAddresses = append(bAddresses, lowerAddresses[index])
			}
			bSymbols = append(bSymbols, GetOrDefault(symbols, index, ""))
			bNetworks = append(bNetworks, GetOrDefault(networks, index, ""))
			bUnixTimeStamps = append(bUnixTimeStamps, unixTimeStamp[index])
		}

		// 根据 source 调用不同的数据源方法
		var results []PriceResult
		var err error
		switch source {
		case "coingecko":
			results, err = s.coinGeckoService.GetBatchHistoricalPrices(bAddresses, bChainIds, bSymbols, bNetworks, bUnixTimeStamps)
		case "geckoterminal":
			results, err = s.geckoTerminalService.GetBatchHistoricalPrices(bAddresses, bChainIds, bSymbols, bNetworks, bUnixTimeStamps)
		case "defillama":
			results, err = s.defiLlamaService.GetBatchHistoricalPrices(bAddresses, bChainIds, bSymbols, bNetworks, bUnixTimeStamps)
		}

		if err == nil {
			for _, result := range results {
				key := fmt.Sprintf("%s_%s_%s", result.ChainID, result.Address, result.TimeStamp)
				if result.Price != nil && *result.Price != "" {
					resultsMap[key] = result
					returnCoinId := fmt.Sprintf("%s_%s", result.ChainID, result.Address)
					if coinIds, exists := retrunCoinToMap[returnCoinId]; exists {
						for _, coinId := range coinIds {
							result.ChainID = strings.Split(coinId, "_")[0]
							result.Address = strings.Split(coinId, "_")[1]
							resultsMap[fmt.Sprintf("%s_%s", coinId, result.TimeStamp)] = result
						}
					}
					// 删除其他数据源集合中的 ID
					delete(geckoIDsSet, key)
					delete(terminalIDsSet, key)
					delete(llamaIDsSet, key)
				}
			}
		} else {
			s.logger.Err(err).Msgf("GetBatchHistoricalPrice 获取%s价格失败", source)
		}
	}

	//查询指定数据源
	if len(geckoIDsSetSource) > 0 {
		batchQueryHistorical("coingecko", geckoIDsSetSource)
	}
	if len(terminalIDsSetSource) > 0 {
		batchQueryHistorical("geckoterminal", terminalIDsSetSource)
	}
	if len(llamaIDsSetSource) > 0 {
		batchQueryHistorical("defillama", llamaIDsSetSource)
	}

	// 数据源顺序查询
	// 查询剩余的默认数据源
	if len(geckoIDsSet) > 0 {
		batchQueryHistorical("coingecko", geckoIDsSet)
	}
	if len(llamaIDsSet) > 0 {
		batchQueryHistorical("defillama", llamaIDsSet)
	}
	if len(terminalIDsSet) > 0 {
		batchQueryHistorical("geckoterminal", terminalIDsSet)
	}

	// 构造最终结果
	results := make([]PriceResult, len(addresses))
	for i, addr := range addresses {
		key := chainIds[i] + "_" + strings.ToLower(addr) + "_" + strconv.FormatInt(unixTimeStamp[i], 10)
		result, exists := resultsMap[key]
		if !exists {
			result = PriceResult{
				ChainID: chainIds[i],
				Address: addr,
				Price:   nil,
				Symbol:  GetOrNil(symbols, i),
				Network: GetOrNil(networks, i),
			}
		}
		result.Serial = i
		result.TimeStamp = datesStr[i]
		result.Symbol = GetOrNil(symbols, i)
		result.Network = GetOrNil(networks, i)
		result.Address = addr

		if result.Price == nil || *result.Price == "" {
			status := "200"
			if result.RequestStatus != nil && *result.RequestStatus != "" {
				status = *result.RequestStatus
			}
			if s.throttler.CoinsThrottle(key, status) {
				s.slack.SaveLog(context.Background(), "priceService-GetBatchHistoricalPrice", result.ChainID, result.Address, time.Unix(unixTimeStamp[i], 0).Format("2006-01-02"), time.Now().Unix())
			}
		}
		results[i] = result
	}

	return results, nil
}

func (s *priceService) EnqueueUniqueRequest(ctx context.Context, setKey string, queueKey string, requestKey string, requestInfo string) (bool, error) {
	luaScript := `
        if redis.call('SADD', KEYS[1], ARGV[1]) == 1 then
            redis.call('RPUSH', KEYS[2], ARGV[2])
            return 1
        else
            return 0
        end
    `
	keys := []string{setKey, queueKey}
	args := []interface{}{requestKey, requestInfo}
	result, err := s.redisClient.Client.Eval(ctx, luaScript, keys, args...).Result()
	if err != nil {
		return false, err
	}
	return result.(int64) == 1, nil
}
func (s *priceService) EnqueueUniqueRequests(ctx context.Context, setKey string, queueKey string, requestKeys []string, requestInfos []string) (int64, error) {
	if len(requestKeys) != len(requestInfos) {
		return 0, fmt.Errorf("requestKeys and requestInfos must have the same length")
	}

	luaScript := `
        local new_requests_count = 0
        for i = 1, #ARGV, 2 do
            local requestKey = ARGV[i]
            local requestInfo = ARGV[i + 1]
            if redis.call('SADD', KEYS[1], requestKey) == 1 then
                redis.call('RPUSH', KEYS[2], requestInfo)
                new_requests_count = new_requests_count + 1
            end
        end
        return new_requests_count
    `

	keys := []string{setKey, queueKey}
	args := make([]interface{}, 0, len(requestKeys)*2)
	for i := range requestKeys {
		args = append(args, requestKeys[i], requestInfos[i])
	}

	result, err := s.redisClient.Client.Eval(ctx, luaScript, keys, args...).Result()
	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}
