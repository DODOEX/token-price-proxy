package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
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

type CoinGeckoOnChainService interface {
	GetCoinGeckoOnChainNetwork(coingeckoAssetPlatformId string, isCache bool) (string, error)
	GetCurrentPriceOnChain(chainId, address string, symbol string, isCache bool) (*string, error)
	GetHistoricalPriceOnChain(chainId, address string, unixTimeStamp int64) (*string, error)
	GetBatchCurrentPricesOnChain(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error)
	GetBatchHistoricalPricesOnChain(addresses []string, chainIds []string, symbols []string, networks []string, unixTimeStamps []int64) ([]PriceResult, error)
}

type coinGeckoOnChainService struct {
	config                  *koanf.Koanf
	redisClient             *shared.RedisClient
	logger                  zerolog.Logger
	coinRepository          repository.CoinRepository
	coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository
	coinGeckoService        CoinGeckoService
	apiKey                  string
}

const coingeckoV3baseURL = "https://pro-api.coingecko.com/api/v3/"

var coinGeckoOnChainRedisPrefix = map[string]string{
	"tokenPools": "coinGeckoOnChain:tokenPools:",
}

func NewCoinGeckoOnChainService(cfg *koanf.Koanf, redisClient *shared.RedisClient, logger zerolog.Logger, coinRepository repository.CoinRepository, coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository, coinGeckoService CoinGeckoService) CoinGeckoOnChainService {
	return &coinGeckoOnChainService{
		config:                  cfg,
		redisClient:             redisClient,
		logger:                  logger,
		coinRepository:          coinRepository,
		coinHistoricalPriceRepo: coinHistoricalPriceRepo,
		coinGeckoService:        coinGeckoService,
		apiKey:                  cfg.String("apiKey.coingeckoOnChain"),
	}
}

func (s *coinGeckoOnChainService) GetCoinGeckoOnChainNetwork(coingeckoAssetPlatformId string, isCache bool) (string, error) {
	const networksListCacheKey = "coinGeckoOnChain:supported_networks"
	// 尝试从缓存中读取
	if isCache {
		cachedData, err := s.redisClient.Client.Get(context.Background(), networksListCacheKey).Result()
		if err == nil {
			var networkMap map[string]string
			if err := json.Unmarshal([]byte(cachedData), &networkMap); err == nil {
				if networkId, exists := networkMap[coingeckoAssetPlatformId]; exists {
					return networkId, nil
				} else {
					s.logger.Debug().Str("coingeckoAssetPlatformId", coingeckoAssetPlatformId).Msg("network with coingecko_asset_platform_id not found in cache")
					return coingeckoAssetPlatformId, nil
				}
			}
		} else if err != redis.Nil {
			// 如果是其他错误，返回错误
			return "", fmt.Errorf("failed to get supported networks from cache: %v", err)
		}
	}

	url := "https://pro-api.coingecko.com/api/v3/onchain/networks"
	headers := map[string]string{
		"accept":           "application/json",
		"x-cg-pro-api-key": s.apiKey,
	}
	body, _, err := shared.DoRequest(http.DefaultClient, url, headers, 0) // 传递 0 表示使用默认超时
	if err != nil {
		return "", err
	}
	var result struct {
		Data []struct {
			ID         string `json:"id"`
			Type       string `json:"type"`
			Attributes struct {
				Name                   string `json:"name"`
				CoingeckoAssetPlatform string `json:"coingecko_asset_platform_id"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return "", err
	}

	networkMap := make(map[string]string)
	var foundNetworkId string
	for _, network := range result.Data {
		networkMap[network.Attributes.CoingeckoAssetPlatform] = network.ID
		if network.Attributes.CoingeckoAssetPlatform == coingeckoAssetPlatformId {
			foundNetworkId = network.ID
		}
	}

	// 将结果存入缓存
	if isCache {
		data, err := json.Marshal(networkMap)
		if err == nil {
			s.redisClient.Client.Set(context.Background(), networksListCacheKey, data, 72*time.Hour).Err()
		}
	}

	if foundNetworkId == "" {
		s.logger.Debug().Str("coingeckoAssetPlatformId", coingeckoAssetPlatformId).Msg("network with coingecko_asset_platform_id not found")
		return coingeckoAssetPlatformId, nil
	}

	return foundNetworkId, nil
}

func (s *coinGeckoOnChainService) GetCurrentPriceOnChain(chainId, address string, symbol string, isCache bool) (*string, error) {
	if symbol == "" || !isSymbolAllowed(symbol) {
		return nil, nil
	}
	assetPlatformId, err := s.coinGeckoService.GetAssetPlatformIdByChainId(chainId)
	if err != nil {
		return nil, err
	}
	network, err := s.GetCoinGeckoOnChainNetwork(assetPlatformId, true)
	if err != nil {
		return nil, err
	}
	coinID := chainId + "_" + address
	if isCache {
		cachedPrice, err := s.redisClient.GetCurrentPriceCache(coinID)
		if err == nil && cachedPrice != "" {
			return &cachedPrice, nil
		}
	}

	url := fmt.Sprintf("%sonchain/simple/networks/%s/token_price/%s", coingeckoV3baseURL, network, address)
	headers := map[string]string{
		"accept":           "application/json",
		"x-cg-pro-api-key": s.apiKey,
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 0) // 传递 0 表示使用默认超时
	if err != nil {
		if statusCode != http.StatusTooManyRequests {
			shared.HandleErrorWithThrottling(s.redisClient, s.logger, "CoinGeckoOnChainService-GetCurrentPriceOnChain", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
		}
		return nil, err
	}

	var result struct {
		Data struct {
			Attributes struct {
				TokenPrices map[string]string `json:"token_prices"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return nil, err
	}

	if tokenPrices, ok := result.Data.Attributes.TokenPrices[address]; ok {
		priceUsd := tokenPrices
		coinID := chainId + "_" + address
		priceToSave := schema.CoinHistoricalPrice{
			CoinID:  coinID,
			Date:    time.Now().Unix(),
			DayDate: time.Now().Format("02-01-2006"),
			Price:   priceUsd,
			Source:  "coinGeckoOnChain",
		}
		// 保存到 redis
		s.redisClient.SetCurrentPriceCache(coinID, priceUsd)
		s.coinHistoricalPriceRepo.SaveHistoricalPrices([]schema.CoinHistoricalPrice{priceToSave})
		return &priceUsd, nil
	}
	return nil, nil
}

func (s *coinGeckoOnChainService) GetHistoricalPriceOnChain(chainId, address string, unixTimeStamp int64) (*string, error) {
	assetPlatformId, err := s.coinGeckoService.GetAssetPlatformIdByChainId(chainId)
	if err != nil {
		return nil, err
	}
	network, err := s.GetCoinGeckoOnChainNetwork(assetPlatformId, true)
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

	tokenPoolsKey := coinGeckoOnChainRedisPrefix["tokenPools"] + tokenInfo
	tokenPools, err := s.redisClient.Client.Get(context.Background(), tokenPoolsKey).Result()
	if err != nil {
		url := fmt.Sprintf("%sonchain/networks/%s/tokens/%s/pools", coingeckoV3baseURL, network, address)
		headers := map[string]string{
			"accept":           "application/json",
			"x-cg-pro-api-key": s.apiKey,
		}

		body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 0) // 传递 0 表示使用默认超时
		if err != nil {
			if statusCode != http.StatusTooManyRequests {
				shared.HandleErrorWithThrottling(s.redisClient, s.logger, "CoinGeckoOnChainService-GetHistoricalPriceOnChain", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
			}
			return nil, err
		}
		var result struct {
			Data []struct {
				ID         string `json:"id"`
				Type       string `json:"type"`
				Attributes struct {
					Address string `json:"address"`
				} `json:"attributes"`
				Relationships struct {
					QuoteToken struct {
						Data struct {
							ID string `json:"id"`
						} `json:"data"`
					} `json:"quote_token"`
				} `json:"relationships"`
			} `json:"data"`
		}

		if err := shared.ParseJSONResponse(body, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %v", err)
		}

		tokenPoolsBytes, _ := json.Marshal(result.Data)
		s.redisClient.Client.Set(context.Background(), tokenPoolsKey, tokenPoolsBytes, 1000*time.Minute)
		tokenPools = string(tokenPoolsBytes)
	}

	var pools []map[string]interface{}
	if err := json.Unmarshal([]byte(tokenPools), &pools); err != nil || len(pools) == 0 {
		return nil, nil
	}

	poolAddress := pools[0]["attributes"].(map[string]interface{})["address"].(string)
	poolQuoteToken := pools[0]["relationships"].(map[string]interface{})["quote_token"].(map[string]interface{})["data"].(map[string]interface{})["id"].(string)
	poolQuoteToken = strings.Split(poolQuoteToken, "_")[1]

	token := "base"
	if address == poolQuoteToken {
		token = "quote"
	}

	ohlcvs, err := s.getOhlcvsOnChain(network, poolAddress, token)
	if err != nil || len(ohlcvs) == 0 {
		return nil, err
	}

	var prices []schema.CoinHistoricalPrice
	priceMap := make(map[string]string)
	for _, item := range ohlcvs {
		itemDate := time.Unix(int64(item[0].(float64)), 0).Format("02-01-2006")
		prices = append(prices, schema.CoinHistoricalPrice{
			CoinID:  coinId,
			Date:    int64(item[0].(float64)),
			DayDate: itemDate,
			Price:   fmt.Sprintf("%f", item[4].(float64)),
			Source:  "coinGeckoOnChain",
		})
		priceMap[coinId+"_"+itemDate] = fmt.Sprintf("%f", item[4].(float64))
	}

	if err := s.coinHistoricalPriceRepo.SaveHistoricalPrices(prices); err != nil {
		s.logger.Error().Err(err).Msg("Failed to save historical prices")
	}

	if price, exists := priceMap[coinId+"_"+date]; exists {
		return &price, nil
	}
	return nil, nil
}

func (s *coinGeckoOnChainService) getOhlcvsOnChain(network, poolAddress, token string) ([][]interface{}, error) {
	url := fmt.Sprintf("%sonchain/networks/%s/pools/%s/ohlcv/day?limit=1000&token=%s", coingeckoV3baseURL, network, poolAddress, token)
	headers := map[string]string{
		"accept":           "application/json",
		"x-cg-pro-api-key": s.apiKey,
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 0) // 传递 0 表示使用默认超时
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get ohlcvs onchain, status code: %d, response: %s", statusCode, string(body))
	}

	var result struct {
		Data struct {
			Attributes struct {
				OhlcvList [][]interface{} `json:"ohlcv_list"`
			} `json:"attributes"`
		} `json:"data"`
	}

	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return nil, err
	}

	return result.Data.Attributes.OhlcvList, nil
}

func (s *coinGeckoOnChainService) GetBatchCurrentPricesOnChain(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error) {
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
			price, err := s.GetCurrentPriceOnChain(chainIds[i], addresses[i], GetOrDefault(symbols, i, ""), isCache)
			if err != nil {
				s.logger.Err(err).Msg("Failed to btch get current price on chain")
			}

			priceResult := PriceResult{
				ChainID:   chainIds[i],
				Address:   addresses[i],
				Price:     price,
				Symbol:    GetOrNil(symbols, i),
				Network:   GetOrNil(networks, i),
				TimeStamp: strconv.FormatInt(time.Now().Unix(), 10),
			}
			results[i] = priceResult
		}(i)
	}

	wg.Wait()
	close(errCh)
	return results, nil
}

func (s *coinGeckoOnChainService) GetBatchHistoricalPricesOnChain(addresses []string, chainIds []string, symbols []string, networks []string, unixTimeStamps []int64) ([]PriceResult, error) {
	if len(networks) != len(addresses) || len(addresses) != len(unixTimeStamps) {
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
				price, err := s.GetHistoricalPriceOnChain(chainIds[i], addresses[i], unixTimeStamps[i])
				if err != nil {
					errCh <- err
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

	// Check for errors
	for err := range errCh {
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

func isSymbolAllowed(symbol string) bool {
	if symbol == "" {
		return false
	}
	for _, pattern := range shared.AllowedTokens {
		if matched, _ := filepath.Match(pattern, symbol); matched {
			return true
		}
	}
	return false
}
