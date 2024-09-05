package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/knadh/koanf/v2"
	"github.com/rs/zerolog"
)

type GeckoTerminalService interface {
	GetCurrentPrice(chainId, address string, isCache bool) (*string, error)
	GetHistoricalPrice(chainId, address string, unixTimeStamp int64) (*string, error)
	GetBatchCurrentPrices(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error)
	GetBatchHistoricalPrices(addresses []string, chainIds []string, symbols []string, networks []string, unixTimeStamps []int64) ([]PriceResult, error)
}

type geckoTerminalService struct {
	config                  *koanf.Koanf
	redisClient             *shared.RedisClient
	logger                  zerolog.Logger
	coinRepository          repository.CoinRepository
	coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository
	totalReserveThreshold   float64
	priceUsdThreshold       float64
	apiKey                  string
}

const baseURL = "https://api.geckoterminal.com/api/v2/"

var chainIdMap = map[string]string{
	"1":           "eth",
	"56":          "bsc",
	"128":         "heco",
	"137":         "polygon_pos",
	"66":          "okexchain",
	"42161":       "arbitrum",
	"1285":        "movr",
	"1313161554":  "aurora",
	"288":         "boba",
	"43114":       "avax",
	"10":          "optimism",
	"25":          "cro",
	"321":         "kcc",
	"100":         "xdai",
	"1030":        "cfx",
	"1088":        "metis",
	"4444":        "qkc",
	"30":          "rsk",
	"60":          "gochain",
	"122":         "fuse",
	"333999":      "okex-testnet",
	"11297108109": "iotex",
	"42220":       "celo",
	"4689":        "iotex",
	"1666600000":  "harmony",
	"10000":       "smartbch",
	"181":         "meter",
	"57":          "sys",
	"1229":        "polis",
	"80001":       "mumbai",
	"65":          "okex-testnet",
	"97":          "bsc-testnet",
	"3":           "ropsten",
	"4":           "rinkeby",
	"5":           "goerli",
	"42":          "kovan",
	"1287":        "moonbeam-testnet",
	"256":         "heco-testnet",
	"4002":        "ftm-testnet",
	"43113":       "avax-testnet",
	"534352":      "scroll",
	"250":         "ftm",
	"59144":       "linea",
	"8453":        "base",
	"5000":        "mantle",
	"169":         "manta-pacific",
}

var redisPrefix = map[string]string{
	"price":           "geckoterminal:price:current:",
	"tokenPools":      "geckoterminal:tokenPools:",
	"token":           "geckoterminal:token:",
	"ohlcv":           "geckoterminal:ohlcv:",
	"historicalPrice": "price:historical:",
	"limit":           "geckoterminal:limit:",
}

func NewGeckoTerminalService(cfg *koanf.Koanf, redisClient *shared.RedisClient, logger zerolog.Logger, coinRepository repository.CoinRepository, coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository) GeckoTerminalService {
	totalReserveThreshold := cfg.Float64("token.totalReserveThreshold")
	if totalReserveThreshold == 0 {
		totalReserveThreshold = 1000 // 默认值
	}

	priceUsdThreshold := cfg.Float64("token.priceUsdThreshold")
	if priceUsdThreshold == 0 {
		priceUsdThreshold = 100000 // 默认值
	}
	return &geckoTerminalService{
		config:                  cfg,
		redisClient:             redisClient,
		logger:                  logger,
		coinRepository:          coinRepository,
		coinHistoricalPriceRepo: coinHistoricalPriceRepo,
		totalReserveThreshold:   totalReserveThreshold,
		priceUsdThreshold:       priceUsdThreshold,
		apiKey:                  cfg.String("apiKey.geckoterminal"),
	}
}

func getTokenInfo(network, address string) string {
	return fmt.Sprintf("%s:%s", network, address)
}

func (s *geckoTerminalService) GetCurrentPrice(chainId, address string, isCache bool) (*string, error) {
	network, err := chainIdToNetwork(chainId)
	if err != nil {
		return nil, err
	}

	tokenInfo := getTokenInfo(network, address)
	coinID := chainId + "_" + address
	// 检查是否存在历史记录
	historicalPrices, err := s.redisClient.GetCurrentPricesCache([]string{coinID})
	if err == nil {
		if price, exists := historicalPrices[coinID]; exists && isCache {
			return &price, nil
		}
	}

	splitTokenInfo := strings.Split(tokenInfo, ":")
	network = splitTokenInfo[0]
	address = splitTokenInfo[1]

	limitKey := redisPrefix["limit"] + network
	limit, err := s.redisClient.Client.Get(context.Background(), limitKey).Int()
	if err == nil && limit > 30 {
		return nil, nil
	}

	s.redisClient.Client.Incr(context.Background(), limitKey)
	s.redisClient.Client.Expire(context.Background(), limitKey, 60*time.Second)

	url := fmt.Sprintf("%snetworks/%s/tokens/%s?partner_api_key=%s", baseURL, network, address, s.apiKey)
	headers := map[string]string{
		"accept": "application/json",
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 15) // 指定 15 秒超时
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNotFound {
		if statusCode != http.StatusTooManyRequests {
			shared.HandleErrorWithThrottling(s.redisClient, s.logger, "GeckoTerminalService-GetCurrentPrice", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
		}
		return nil, fmt.Errorf("failed to get prices, status code: %d, response: %s", statusCode, string(body))
	}

	var result map[string]interface{}
	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return nil, err
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		if attributes, ok := data["attributes"].(map[string]interface{}); ok {
			if priceUsd, ok := attributes["price_usd"].(string); ok {
				// 新增逻辑：total_reserve_in_usd 小于 1000 并且 price_usd 大于 100000 则返回 nil, nil
				if totalReserveInUsd, ok := attributes["total_reserve_in_usd"].(string); ok {
					totalReserve, err := strconv.ParseFloat(totalReserveInUsd, 64)
					if err == nil && totalReserve < s.totalReserveThreshold {
						price, err := strconv.ParseFloat(priceUsd, 64)
						if err == nil && price > s.priceUsdThreshold {
							return nil, nil
						}
					}
				}
				s.redisClient.SetCurrentPriceCache(coinID, priceUsd)
				priceToSave := schema.CoinHistoricalPrice{
					CoinID:  chainId + "_" + address,
					Date:    time.Now().Unix(),
					DayDate: time.Now().Format("02-01-2006"),
					Price:   priceUsd,
					Source:  "geckoterminal",
				}
				//如果coins 表没有这个coin 就先保存
				// 检查 coins 表中是否存在该 coin
				existingCoin, err := s.coinRepository.GetCoinsByOneID(coinID)
				if err != nil {
					return nil, err
				}
				if existingCoin == nil || existingCoin.ID == "" {
					// 插入新 coin 记录
					newCoin := schema.Coins{
						ID:                   coinID,
						Address:              address,
						ChainID:              chainId,
						Symbol:               shared.GetStringPtr(attributes["symbol"]),
						Name:                 shared.GetStringPtr(attributes["name"]),
						GeckoterminalNetwork: &network,
						Decimals:             shared.GetIntPtr(attributes["decimals"]),
						TotalSupply:          shared.GetStringPtr(attributes["total_supply"]),
						PriceSource:          shared.GetStringPtr("geckoterminal"),
					}
					err = s.coinRepository.UpsertCoins([]schema.Coins{newCoin})
					if err != nil {
						return nil, err
					}
				}
				s.coinHistoricalPriceRepo.SaveHistoricalPrices([]schema.CoinHistoricalPrice{priceToSave})
				return &priceUsd, nil
			}
		}
	}
	return nil, nil
}

func (s *geckoTerminalService) GetHistoricalPrice(chainId, address string, unixTimeStamp int64) (*string, error) {
	network, err := chainIdToNetwork(chainId)
	if err != nil {
		return nil, err
	}

	coinId := chainId + "_" + strings.ToLower(address)

	tokenInfo := getTokenInfo(network, address)
	splitTokenInfo := strings.Split(tokenInfo, ":")
	network = splitTokenInfo[0]
	address = splitTokenInfo[1]

	date := time.Unix(unixTimeStamp, 0).Format("02-01-2006")
	// 检查是否存在历史记录
	historicalPrices, err := s.coinHistoricalPriceRepo.GetHistoricalPrices([]string{coinId}, []int64{unixTimeStamp})
	if err == nil {
		if price, exists := historicalPrices[coinId+"_"+date]; exists {
			return &price, nil
		}
	}
	if date == time.Now().Format("02-01-2006") {
		price, err := s.GetCurrentPrice(chainId, address, true)
		if err != nil && price != nil && *price != "" {
			s.redisClient.SetHistoricalPriceCache(coinId, date, *price)
		}
		return price, err
	}

	// 检查Redis缓存中是否存在token信息
	tokenCacheKey := fmt.Sprintf("%stokenInfo:%s", redisPrefix["token"], tokenInfo)
	cachedTokenInfo, err := s.redisClient.Client.Get(context.Background(), tokenCacheKey).Result()
	if err == nil {
		var tokenResult map[string]interface{}
		if err := json.Unmarshal([]byte(cachedTokenInfo), &tokenResult); err == nil {
			if price, err := s.processTokenData(tokenResult, coinId, date, network, address, chainId); err == nil && price != nil {
				return price, nil
			}
		}
	}

	url := fmt.Sprintf("%snetworks/%s/tokens/%s?partner_api_key=%s", baseURL, network, address, s.apiKey)
	headers := map[string]string{
		"accept": "application/json",
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 15) // 使用15秒超时
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK && statusCode != http.StatusNotFound {
		if statusCode != http.StatusTooManyRequests {
			shared.HandleErrorWithThrottling(s.redisClient, s.logger, "GeckoTerminalService-GetHistoricalPrice", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
		}
		return nil, fmt.Errorf("failed to get prices, status code: %d, response: %s", statusCode, string(body))
	}

	var result map[string]interface{}
	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return nil, err
	}

	// 缓存token信息
	tokenInfoBytes, _ := json.Marshal(result)
	s.redisClient.Client.Set(context.Background(), tokenCacheKey, tokenInfoBytes, 24*time.Hour)

	return s.processTokenData(result, coinId, date, network, address, chainId)
}

func (s *geckoTerminalService) processTokenData(tokenData map[string]interface{}, coinId, date, network, address string, chainId string) (*string, error) {
	if data, ok := tokenData["data"].(map[string]interface{}); ok {
		if attributes, ok := data["attributes"].(map[string]interface{}); ok {
			if priceUsd, ok := attributes["price_usd"].(string); ok {
				// 新增逻辑：total_reserve_in_usd 小于 1000 并且 price_usd 大于 100000 则返回 nil, nil
				if totalReserveInUsd, ok := attributes["total_reserve_in_usd"].(string); ok {
					totalReserve, err := strconv.ParseFloat(totalReserveInUsd, 64)
					if err == nil && totalReserve < s.totalReserveThreshold {
						price, err := strconv.ParseFloat(priceUsd, 64)
						if err == nil && price > s.priceUsdThreshold {
							return nil, nil
						}
					}
				}
			}
			//如果coins 表没有这个coin 就先保存
			// 检查 coins 表中是否存在该 coin
			existingCoin, err := s.coinRepository.GetCoinsByOneID(coinId)
			if err != nil {
				return nil, err
			}
			if existingCoin == nil || existingCoin.ID == "" {
				// 插入新 coin 记录
				newCoin := schema.Coins{
					ID:                   coinId,
					Address:              address,
					ChainID:              chainId,
					Symbol:               shared.GetStringPtr(attributes["symbol"]),
					Name:                 shared.GetStringPtr(attributes["name"]),
					GeckoterminalNetwork: &network,
					Decimals:             shared.GetIntPtr(attributes["decimals"]),
					TotalSupply:          shared.GetStringPtr(attributes["total_supply"]),
					PriceSource:          shared.GetStringPtr("geckoterminal"),
				}
				s.coinRepository.UpsertCoins([]schema.Coins{newCoin})
			}
		}
		if relationships, ok := data["relationships"].(map[string]interface{}); ok {
			if topPools, ok := relationships["top_pools"].(map[string]interface{}); ok {
				if poolData, ok := topPools["data"].([]interface{}); ok && len(poolData) > 0 {
					poolID := poolData[0].(map[string]interface{})["id"].(string)
					poolAddress := extractPoolAddress(poolID)

					// 检查Redis缓存中是否存在池子信息
					poolCacheKey := fmt.Sprintf("%spoolInfo:%s", redisPrefix["tokenPools"], poolAddress)
					cachedPoolInfo, err := s.redisClient.Client.Get(context.Background(), poolCacheKey).Result()
					if err == nil {
						var poolResult map[string]interface{}
						if err := json.Unmarshal([]byte(cachedPoolInfo), &poolResult); err == nil {
							return s.processPoolData(poolResult, coinId, date, network, address, poolAddress)
						}
					}

					poolUrl := fmt.Sprintf("%snetworks/%s/pools/%s?partner_api_key=%s", baseURL, network, poolAddress, s.apiKey)
					headers := map[string]string{
						"accept": "application/json",
					}

					poolBody, statusCode, err := shared.DoRequest(http.DefaultClient, poolUrl, headers, 15) // 使用15秒超时
					if err != nil {
						return nil, err
					}

					if statusCode != http.StatusOK && statusCode != http.StatusNotFound {
						if statusCode != http.StatusTooManyRequests {
							shared.HandleErrorWithThrottling(s.redisClient, s.logger, "GeckoTerminalService-GetHistoricalPrice-Pool", fmt.Sprintf("url: %s, status code: %d, response: %s", poolUrl, statusCode, string(poolBody)))
						}
						return nil, fmt.Errorf("failed to get pool info, status code: %d, response: %s", statusCode, string(poolBody))
					}

					var poolResult map[string]interface{}
					if err := shared.ParseJSONResponse(poolBody, &poolResult); err != nil {
						return nil, err
					}

					// 缓存池子信息
					poolInfoBytes, _ := json.Marshal(poolResult)
					s.redisClient.Client.Set(context.Background(), poolCacheKey, poolInfoBytes, 24*time.Hour)

					return s.processPoolData(poolResult, coinId, date, network, address, poolAddress)
				}
			}
		}
	}
	return nil, nil
}

func (s *geckoTerminalService) processPoolData(poolData map[string]interface{}, coinId, date, network, address, poolAddress string) (*string, error) {
	if data, ok := poolData["data"].(map[string]interface{}); ok {
		baseTokenId := data["relationships"].(map[string]interface{})["base_token"].(map[string]interface{})["data"].(map[string]interface{})["id"].(string)
		baseTokenAddress := extractTokenAddress(baseTokenId)

		token := "base"
		if address != baseTokenAddress {
			token = "quote"
		}

		ohlcvs, err := s.getOhlcvs(network, poolAddress, token)
		if err != nil || len(ohlcvs) == 0 {
			return nil, err
		}

		var prices []schema.CoinHistoricalPrice
		priceMap := make(map[string]string)
		for _, item := range ohlcvs {
			itemDate := time.Unix(int64(item[0].(float64)), 0).Format("02-01-2006")
			priceStr := strconv.FormatFloat(item[4].(float64), 'f', -1, 64)
			prices = append(prices, schema.CoinHistoricalPrice{
				CoinID:  coinId,
				Date:    int64(item[0].(float64)),
				DayDate: itemDate,
				Price:   priceStr,
				Source:  "geckoterminal",
			})
			priceMap[coinId+"_"+itemDate] = priceStr
		}

		if err := s.coinHistoricalPriceRepo.SaveHistoricalPrices(prices); err != nil {
			s.logger.Error().Err(err).Msg("Failed to save historical prices")
		}

		if price, exists := priceMap[coinId+"_"+date]; exists {
			return &price, nil
		}
	}
	return nil, nil
}

// 提取池子地址
func extractPoolAddress(poolID string) string {
	parts := strings.Split(poolID, "_")
	return parts[len(parts)-1]
}

// 提取token地址
func extractTokenAddress(tokenID string) string {
	parts := strings.Split(tokenID, "_")
	return parts[len(parts)-1]
}

func (s *geckoTerminalService) getOhlcvs(network, poolAddress, token string) ([][]interface{}, error) {
	url := fmt.Sprintf("%snetworks/%s/pools/%s/ohlcv/day?limit=1000&token=%s&partner_api_key=%s", baseURL, network, poolAddress, token, s.apiKey)
	headers := map[string]string{
		"accept": "application/json",
	}

	body, _, err := shared.DoRequest(http.DefaultClient, url, headers, 0) // 使用默认超时
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return nil, err
	}

	if data, ok := result["data"].(map[string]interface{}); ok {
		if attributes, ok := data["attributes"].(map[string]interface{}); ok {
			if ohlcvList, ok := attributes["ohlcv_list"].([]interface{}); ok {
				var ohlcvs [][]interface{}
				for _, item := range ohlcvList {
					ohlcv := item.([]interface{})
					ohlcvs = append(ohlcvs, ohlcv)
				}
				return ohlcvs, nil
			}
		}
	}

	return nil, nil
}

func chainIdToNetwork(chainId string) (string, error) {
	network, exists := chainIdMap[chainId]
	if !exists {
		return "", fmt.Errorf("invalid chainId: %s", chainId)
	}
	return network, nil
}

func (s *geckoTerminalService) GetBatchCurrentPrices(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error) {
	if len(chainIds) != len(addresses) {
		return nil, fmt.Errorf("chainIds and addresses must have the same length")
	}

	results := make([]PriceResult, len(addresses))
	errCh := make(chan error, len(addresses))
	var wg sync.WaitGroup

	for i := range chainIds {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			price, err := s.GetCurrentPrice(chainIds[i], addresses[i], isCache)
			requestStatus := "200"
			if err != nil {
				if strings.Contains(err.Error(), "429") {
					requestStatus = "429"
				}
			}
			priceResult := PriceResult{
				ChainID:       chainIds[i],
				Address:       addresses[i],
				Price:         price,
				Symbol:        GetOrNil(symbols, i),
				Network:       GetOrNil(networks, i),
				TimeStamp:     strconv.FormatInt(time.Now().Unix(), 10),
				RequestStatus: &requestStatus,
			}
			results[i] = priceResult
		}(i)
	}

	wg.Wait()
	close(errCh)

	return results, nil
}

func (s *geckoTerminalService) GetBatchHistoricalPrices(addresses []string, chainIds []string, symbols []string, networks []string, unixTimeStamps []int64) ([]PriceResult, error) {
	if len(chainIds) != len(addresses) || len(addresses) != len(unixTimeStamps) {
		return nil, fmt.Errorf("chainIds, addresses and unixTimeStamps must have the same length")
	}

	results := make([]PriceResult, len(addresses))
	errCh := make(chan error, len(addresses))
	var wg sync.WaitGroup

	// 创建一个存放 CoinID 和日期的切片
	coinIds := make([]string, len(addresses))
	dates := make([]int64, len(addresses))
	for i := range addresses {
		coinIds[i] = chainIds[i] + "_" + addresses[i]
		dates[i] = unixTimeStamps[i]
	}

	historicalPrices, err := s.coinHistoricalPriceRepo.GetHistoricalPrices(coinIds, dates)
	if err != nil {
		return nil, fmt.Errorf("批量查询历史价格失败: %v", err)
	}

	for i := range chainIds {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			coinId := chainIds[i] + "_" + addresses[i]
			date := time.Unix(unixTimeStamps[i], 0).Format("02-01-2006")
			historicalPrice, exists := historicalPrices[coinId+"_"+date]
			if exists {
				priceResult := PriceResult{
					ChainID:   chainIds[i],
					Address:   addresses[i],
					Price:     &historicalPrice,
					Symbol:    GetOrNil(symbols, i),
					Network:   GetOrNil(networks, i),
					TimeStamp: strconv.FormatInt(unixTimeStamps[i], 10),
				}
				results[i] = priceResult
			} else {
				price, err := s.GetHistoricalPrice(chainIds[i], addresses[i], unixTimeStamps[i])
				if err != nil {
					errCh <- err
					priceResult := PriceResult{
						ChainID:   chainIds[i],
						Address:   addresses[i],
						Price:     &historicalPrice,
						Symbol:    GetOrNil(symbols, i),
						Network:   GetOrNil(networks, i),
						TimeStamp: strconv.FormatInt(unixTimeStamps[i], 10),
					}
					results[i] = priceResult
					return
				}

				priceResult := PriceResult{
					ChainID:   chainIds[i],
					Address:   addresses[i],
					Price:     price,
					Symbol:    GetOrNil(symbols, i),
					Network:   GetOrNil(networks, i),
					TimeStamp: strconv.FormatInt(unixTimeStamps[i], 10),
				}
				results[i] = priceResult
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	return results, nil
}
