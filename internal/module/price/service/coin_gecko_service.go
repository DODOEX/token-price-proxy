package service

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/google/uuid"
	"github.com/knadh/koanf/v2"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type AssetPlatform struct {
	ID           string `json:"id"`
	ChainID      *int   `json:"chain_identifier"`
	Name         string `json:"name"`
	ShortName    string `json:"shortname"`
	NativeCoinID string `json:"native_coin_id"`
}

type CoinGeckoService interface {
	CoinsList(useCache bool) ([]schema.Coins, error)
	getAssetPlatforms(isCache bool) (map[string]string, error)
	GetCoinGeckoChainIdByAssetPlatformId(assetPlatformId string) (string, error) // get chain id by asset platform id
	GetAssetPlatformIdByChainId(chainId string) (string, error)
	SyncCoins() error
	GetBatchPrice(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error)
	GetBatchHistoricalPrices(addresses []string, chainIds []string, symbols []string, networks []string, dates []int64) ([]PriceResult, error)
	GetSinglePrice(chainID string, address string, symbol string, network string, isCache bool) (*string, error)
	GetSingleHistoricalPrice(date int64, chainID string, address string, symbol string, network string) (*string, error)
}

type coinGeckoService struct {
	config                  *koanf.Koanf
	coinRepository          repository.CoinRepository
	coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository
	redisClient             *shared.RedisClient
	logger                  zerolog.Logger
	apiKey                  string
}

func NewCoinGeckoService(cfg *koanf.Koanf, coinRepository repository.CoinRepository, coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository, redisClient *shared.RedisClient, logger zerolog.Logger) CoinGeckoService {
	return &coinGeckoService{
		config:                  cfg,
		coinRepository:          coinRepository,
		coinHistoricalPriceRepo: coinHistoricalPriceRepo,
		redisClient:             redisClient,
		logger:                  logger,
		apiKey:                  cfg.String("apiKey.coingecko"),
	}
}

type CoinGeckoCoin struct {
	ID        string            `json:"id"`
	Symbol    string            `json:"symbol"`
	Name      string            `json:"name"`
	Platforms map[string]string `json:"platforms"`
}

// 定义价格结果结构体
type PriceResult struct {
	ChainID       string  `json:"chainId"`
	Address       string  `json:"address"`
	Price         *string `json:"price"`
	Symbol        *string `json:"symbol"`
	Network       *string `json:"network"`
	TimeStamp     string  `json:"date"`
	RequestStatus *string `json:"-"`
	Serial        int     `json:"serial"`
}

func (s *coinGeckoService) getAssetPlatforms(isCache bool) (map[string]string, error) {
	cacheKey := "coingecko:asset_platforms"

	if isCache {
		// 尝试从缓存中读取
		cachedData, err := s.redisClient.Client.Get(context.Background(), cacheKey).Result()
		if err == nil {
			var platformMap map[string]string
			if err := json.Unmarshal([]byte(cachedData), &platformMap); err == nil {
				return platformMap, nil
			}
		} else if err != redis.Nil {
			// 如果是其他错误，返回错误
			return nil, fmt.Errorf("failed to get asset platforms from cache: %v", err)
		}
	}

	url := "https://pro-api.coingecko.com/api/v3/asset_platforms"
	headers := map[string]string{
		"accept":           "application/json",
		"x-cg-pro-api-key": s.apiKey,
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 15) // 传递 0 表示使用默认超时
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get asset platforms, status code: %d, response: %s", statusCode, string(body))
	}

	var platforms []AssetPlatform
	if err := shared.ParseJSONResponse(body, &platforms); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	platformMap := make(map[string]string)
	for _, platform := range platforms {
		if platform.ChainID != nil {
			platformMap[platform.ID] = fmt.Sprintf("%d", *platform.ChainID)
		} else {
			platformMap[platform.ID] = platform.ID
		}
	}

	// 将结果存入缓存
	data, err := json.Marshal(platformMap)
	if err == nil {
		s.redisClient.Client.Set(context.Background(), cacheKey, data, 24*time.Hour).Err()
	}

	return platformMap, nil
}

func (s *coinGeckoService) getAssetPlatformsChainIdMap(isCache bool) (map[string]string, error) {
	cacheKey := "coingecko:asset_platforms_chain_id_map"

	if isCache {
		// 尝试从缓存中读取
		cachedData, err := s.redisClient.Client.Get(context.Background(), cacheKey).Result()
		if err == nil {
			var platformMap map[string]string
			if err := json.Unmarshal([]byte(cachedData), &platformMap); err == nil {
				return platformMap, nil
			}
		} else if err != redis.Nil {
			// 如果是其他错误，返回错误
			return nil, fmt.Errorf("failed to get asset platforms from cache: %v", err)
		}
	}

	url := "https://pro-api.coingecko.com/api/v3/asset_platforms"
	headers := map[string]string{
		"accept":           "application/json",
		"x-cg-pro-api-key": s.apiKey,
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 15) // 传递 0 表示使用默认超时
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get asset platforms, status code: %d, response: %s", statusCode, string(body))
	}

	var platforms []AssetPlatform
	if err := shared.ParseJSONResponse(body, &platforms); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	platformMap := make(map[string]string)
	for _, platform := range platforms {
		if platform.ChainID != nil {
			platformMap[fmt.Sprintf("%d", *platform.ChainID)] = platform.ID
		}
	}

	// 将结果存入缓存
	data, err := json.Marshal(platformMap)
	if err == nil {
		s.redisClient.Client.Set(context.Background(), cacheKey, data, 24*time.Hour).Err()
	}

	return platformMap, nil
}
func (s *coinGeckoService) GetCoinGeckoChainIdByAssetPlatformId(assetPlatformId string) (string, error) {
	platformMap, err := s.getAssetPlatforms(true)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get asset platforms")
		return "", fmt.Errorf("failed to get asset platforms: %v", err)
	}
	if res, ok := platformMap[assetPlatformId]; !ok {
		return "", fmt.Errorf("asset platform id %s not found", assetPlatformId)
	} else {
		return res, nil
	}
}

func (s *coinGeckoService) GetAssetPlatformIdByChainId(chainId string) (string, error) {
	platformMap, err := s.getAssetPlatformsChainIdMap(true)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get asset platforms")
		return "", fmt.Errorf("failed to get asset platforms: %v", err)
	}
	if res, ok := platformMap[chainId]; !ok {
		return "", fmt.Errorf("asset platform chainId %s not found", chainId)
	} else {
		return res, nil
	}
}

func (s *coinGeckoService) CoinsList(useCache bool) ([]schema.Coins, error) {
	// 首先检查缓存中是否存在 coins 列表
	if useCache {
		cachedCoins, cacheErr := s.redisClient.Client.Get(context.Background(), "coins_list").Result()
		if cacheErr == nil {
			var coins []schema.Coins
			if err := json.Unmarshal([]byte(cachedCoins), &coins); err == nil {
				s.logger.Info().Msg("Fetched coins list from cache")
				return coins, nil
			}
		}
	}

	// 获取平台映射

	platformMap, err := s.getAssetPlatforms(true)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get asset platforms")
		return nil, fmt.Errorf("failed to get asset platforms: %v", err)
	}

	url := "https://pro-api.coingecko.com/api/v3/coins/list?include_platform=true"
	headers := map[string]string{
		"accept":           "application/json",
		"x-cg-pro-api-key": s.apiKey,
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 15) // 传递 0 表示使用默认超时
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to execute request")
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}

	if statusCode != http.StatusOK {
		s.logger.Error().Str("response", string(body)).Int("status", statusCode).Msg("failed to get coins list")
		return nil, fmt.Errorf("failed to get coins list, status code: %d, response: %s", statusCode, string(body))
	}

	var geckoCoins []CoinGeckoCoin
	if err := shared.ParseJSONResponse(body, &geckoCoins); err != nil {
		s.logger.Error().Err(err).Msg("failed to decode response")
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	var coins []schema.Coins
	coinMap := make(map[string]struct{}) // 用于记录已经处理过的 id
	for _, geckoCoin := range geckoCoins {
		for chainName, address := range geckoCoin.Platforms {
			if address == "" {
				continue
			}
			chainID, ok := platformMap[chainName]
			if !ok {
				continue
			}
			id := chainID + "_" + address

			// 检查是否已经处理过这个 id
			if _, exists := coinMap[id]; exists {
				continue
			}

			symbol := geckoCoin.Symbol
			name := geckoCoin.Name
			coingeckoCoinID := geckoCoin.ID
			priceSource := "coingecko"
			coin := schema.Coins{
				ID:                 id,
				Address:            address,
				ChainID:            chainID,
				Symbol:             &symbol,
				Name:               &name,
				CoingeckoCoinID:    &coingeckoCoinID,
				CoingeckoPlatforms: geckoCoin.Platforms,
				PriceSource:        &priceSource,
			}
			coins = append(coins, coin)
			coinMap[id] = struct{}{} // 记录已经处理过的 id
		}
	}
	// 将结果存储到缓存中
	cacheData, _ := json.Marshal(coins)
	s.redisClient.Client.Set(context.Background(), "coins_list", cacheData, 72*time.Hour)

	return coins, nil
}

func (s *coinGeckoService) SyncCoins() error {
	coins, err := s.CoinsList(false)
	if err != nil {
		return err
	}
	if err := s.coinRepository.UpsertCoins(coins); err != nil {
		s.logger.Error().Err(err).Msg("批量插入或更新coins失败")
	}
	return nil
}

func (s *coinGeckoService) GetBatchPrice(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error) {
	ids := make([]string, len(addresses))
	nowTime := time.Now().Unix()
	nowDay := time.Now().Format("02-01-2006")
	dates := make([]int64, len(ids))

	for i, addr := range addresses {
		ids[i] = chainIds[i] + "_" + addr
		dates[i] = nowTime
	}

	existingPrices, err := s.redisClient.GetCurrentPricesCache(ids)

	if err != nil {
		return nil, fmt.Errorf("批量查询历史价格失败: %v", err)
	}

	coins, err := s.coinRepository.GetCoinsByID(ids)
	if err != nil {
		return nil, err
	}

	var coingeckoIDs []string
	coingeckoIDMap := make(map[string]string, len(coins))
	coinsMap := make(map[string]schema.Coins, len(coins))
	for i, coin := range coins {
		if coin.ID == "" || coin.CoingeckoCoinID == nil {
			continue
		}
		coinID := ids[i]
		cacheKey := coinID + "_" + nowDay
		_, priceExists := existingPrices[cacheKey]

		// 如果缓存中没有价格 或者 不使用缓存
		if !priceExists || !isCache {
			coingeckoID := *coin.CoingeckoCoinID
			coingeckoIDs = append(coingeckoIDs, coingeckoID)
			coingeckoIDMap[coin.ID] = coingeckoID
		}
		coinsMap[coin.ID] = coin
	}
	var priceMap map[string]map[string]float64
	if len(coingeckoIDs) > 0 {
		url := fmt.Sprintf("https://pro-api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd", strings.Join(coingeckoIDs, "%2C"))
		headers := map[string]string{
			"accept":           "application/json",
			"x-cg-pro-api-key": s.apiKey,
		}

		body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 0) // 传递 0 表示使用默认超时
		if err != nil {
			return nil, fmt.Errorf("failed to execute request: %v", err)
		}

		if statusCode != http.StatusOK {
			if statusCode != http.StatusTooManyRequests {
				shared.HandleErrorWithThrottling(s.redisClient, s.logger, "CoinGeckoService-GetBatchPrice", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
			}
			return nil, fmt.Errorf("failed to get prices, status code: %d, response: %s", statusCode, string(body))
		}

		if err := shared.ParseJSONResponse(body, &priceMap); err != nil {
			return nil, fmt.Errorf("failed to decode response: %v", err)
		}

	}

	var results []PriceResult
	var pricesToSave []schema.CoinHistoricalPrice

	for i := range addresses {
		coinID := chainIds[i] + "_" + addresses[i]
		var price *string

		// 优先使用缓存中的历史价格
		historicalPrice, exists := existingPrices[coinID]
		if exists {
			price = &historicalPrice
		} else {
			// 从 coingeckoIDMap 中获取 coingeckoCoinID 并请求 API 获取最新价格
			if coingeckoCoinID, exists := coingeckoIDMap[coinID]; exists {
				if coinPrice, ok := priceMap[coingeckoCoinID]; ok {
					if usdPrice, ok := coinPrice["usd"]; ok {
						// priceStr := fmt.Sprintf("%g", usdPrice)
						priceStr := strconv.FormatFloat(usdPrice, 'f', -1, 64)
						price = &priceStr

						// 保存到 redis
						s.redisClient.SetCurrentPriceCache(coinID, priceStr)

						// 保存到 coinHistoricalPriceRepository
						coin, coinsExists := coinsMap[coinID]
						if coinsExists {
							coinID = coin.ChainID + "_" + coin.Address
						}
						priceToSave := schema.CoinHistoricalPrice{
							CoinID:  coinID,
							Date:    nowTime,
							DayDate: nowDay,
							Price:   priceStr,
							Source:  "coingecko",
						}
						pricesToSave = append(pricesToSave, priceToSave)
					}
				}
			}
		}

		results = append(results, PriceResult{
			ChainID:   chainIds[i],
			Address:   addresses[i],
			Price:     price,
			Symbol:    GetOrNil(symbols, i),
			Network:   GetOrNil(networks, i),
			TimeStamp: strconv.FormatInt(nowTime, 10),
		})
	}

	if err := s.coinHistoricalPriceRepo.SaveHistoricalPrices(pricesToSave); err != nil {
		return nil, fmt.Errorf("failed to save historical prices: %v", err)
	}

	return results, nil
}

// 获取批量历史价格的方法
func (s *coinGeckoService) GetBatchHistoricalPrices(addresses []string, chainIds []string, symbols []string, networks []string, dates []int64) ([]PriceResult, error) {
	ids := make([]string, len(addresses))
	for i, addr := range addresses {
		ids[i] = chainIds[i] + "_" + addr
	}

	coins, err := s.coinRepository.GetCoinsByID(ids)
	if err != nil {
		return nil, err
	}

	coinMap := make(map[string]schema.Coins)
	for _, coin := range coins {
		if coin.ID != "" && coin.CoingeckoCoinID != nil {
			coinMap[coin.ID] = coin
		}
	}

	existingPrices, err := s.coinHistoricalPriceRepo.GetHistoricalPrices(ids, dates)
	if err != nil {
		return nil, fmt.Errorf("批量查询历史价格失败: %v", err)
	}

	var results []PriceResult
	var pricesToSave []schema.CoinHistoricalPrice
	for i, id := range ids {
		historicalPrice, exists := existingPrices[id+"_"+time.Unix(dates[i], 0).Format("02-01-2006")]
		var price *string
		if exists {
			price = &historicalPrice
		} else if coin, ok := coinMap[id]; ok && coin.CoingeckoCoinID != nil {
			date := time.Unix(dates[i], 0).Format("02-01-2006")
			if date == time.Now().Format("02-01-2006") {
				re, err := s.GetBatchPrice([]string{addresses[i]}, []string{chainIds[i]}, []string{symbols[i]}, []string{networks[i]}, true)
				if err == nil {
					price = re[0].Price
				}
			} else {
				url := fmt.Sprintf("https://pro-api.coingecko.com/api/v3/coins/%s/history?date=%s", *coin.CoingeckoCoinID, date)
				headers := map[string]string{
					"accept":           "application/json",
					"x-cg-pro-api-key": s.apiKey,
				}

				body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 0) // 传递 0 表示使用默认超时
				if err != nil {
					return nil, fmt.Errorf("执行请求失败: %v", err)
				}

				if statusCode != http.StatusOK {
					if statusCode != http.StatusTooManyRequests {
						shared.HandleErrorWithThrottling(s.redisClient, s.logger, "CoinGeckoService-GetBatchPrice", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
					}
					return nil, fmt.Errorf("获取历史价格失败，状态码: %d，响应: %s", statusCode, string(body))
				}

				var priceData struct {
					ID         string `json:"id"`
					MarketData struct {
						CurrentPrice map[string]float64 `json:"current_price"`
					} `json:"market_data"`
				}
				if err := shared.ParseJSONResponse(body, &priceData); err != nil {
					return nil, fmt.Errorf("解析响应失败: %v", err)
				}

				priceFloat, ok := priceData.MarketData.CurrentPrice["usd"]
				if ok {
					priceStr := strconv.FormatFloat(priceFloat, 'f', -1, 64)
					price = &priceStr

					// 保存到 coinHistoricalPriceRepository
					coinID := coin.ChainID + "_" + coin.Address
					priceToSave := schema.CoinHistoricalPrice{
						CoinID:  coinID,
						Date:    dates[i],
						DayDate: date,
						Price:   priceStr,
						Source:  "coingecko",
					}
					pricesToSave = append(pricesToSave, priceToSave)
				}
			}
		}

		results = append(results, PriceResult{
			ChainID:   chainIds[i],
			Address:   addresses[i],
			Price:     price,
			Symbol:    GetOrNil(symbols, i),
			Network:   GetOrNil(networks, i),
			TimeStamp: strconv.FormatInt(dates[i], 10),
		})
	}

	if err := s.coinHistoricalPriceRepo.SaveHistoricalPrices(pricesToSave); err != nil {
		return nil, fmt.Errorf("failed to save historical prices: %v", err)
	}

	return results, nil
}

func (s *coinGeckoService) GetSinglePrice(chainID, address, symbol, network string, isCache bool) (*string, error) {
	prices, err := s.GetBatchPrice([]string{address}, []string{chainID}, []string{symbol}, []string{network}, isCache)
	if err != nil || len(prices) == 0 {
		return nil, err
	}
	return prices[0].Price, nil
}

func (s *coinGeckoService) GetSingleHistoricalPrice(date int64, chainID, address, symbol, network string) (*string, error) {
	results, err := s.GetBatchHistoricalPrices([]string{address}, []string{chainID}, []string{symbol}, []string{network}, []int64{date})
	if err != nil || len(results) == 0 {
		return nil, err
	}
	return results[0].Price, nil
}

func GetOrNil(slice []string, index int) *string {
	if index >= 0 && index < len(slice) {
		if slice[index] == "" {
			return nil
		}
		return &slice[index]
	}
	return nil
}
func GetOrDefault(slice []string, index int, defaultValue string) string {
	if index >= 0 && index < len(slice) {
		return slice[index]
	}
	return defaultValue
}

// 安全地获取 *string 的值，如果为空则返回默认值
func safeDereferenceString(s *string, defaultVal string) string {
	if s == nil || *s == "" {
		return defaultVal
	}
	return *s
}

// 生成 requestID
func generateRequestsID(parts ...[]string) string {
	var data string
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		data += strings.Join(part, "_")
	}
	data += uuid.New().String()
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

func generateRequestID(parts ...string) string {
	data := strings.Join(parts, "_") + "_" + uuid.New().String()
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}
