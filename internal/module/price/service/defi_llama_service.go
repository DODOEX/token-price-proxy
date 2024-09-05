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

var (
	chainNames        map[string]string
	defiLlamaBaseURL  = "https://coins.llama.fi"
	defiLlamaRedisKey = "defiLlama:"
)

type DefiLlamaService interface {
	GetCurrentPrice(chainId, address string, isCache bool) (*string, error)
	GetHistoricalPrice(chainId, address string, unixTimeStamp int64) (*string, error)
	GetBatchCurrentPrices(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error)
	GetBatchHistoricalPrices(addresses []string, chainIds []string, symbols []string, networks []string, unixTimeStamps []int64) ([]PriceResult, error)
}

type defiLlamaService struct {
	config                  *koanf.Koanf
	redisClient             *shared.RedisClient
	logger                  zerolog.Logger
	coinRepository          repository.CoinRepository
	coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository
}

func NewDefiLlamaService(cfg *koanf.Koanf, redisClient *shared.RedisClient, logger zerolog.Logger, coinRepository repository.CoinRepository, coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository) DefiLlamaService {
	return &defiLlamaService{
		config:                  cfg,
		redisClient:             redisClient,
		logger:                  logger,
		coinRepository:          coinRepository,
		coinHistoricalPriceRepo: coinHistoricalPriceRepo,
	}
}

func (s *defiLlamaService) initializeChainNames() {
	cacheKey := defiLlamaRedisKey + "chainNamesAndTVL"
	cachedChainNames, err := s.redisClient.Client.Get(context.Background(), cacheKey).Result()
	if err == nil && cachedChainNames != "" {
		var tempChainNames map[string]string
		if err := json.Unmarshal([]byte(cachedChainNames), &tempChainNames); err == nil {
			chainNames = tempChainNames
			return
		}
	}

	url := "https://api.llama.fi/v2/chains"
	resp, err := http.Get(url)
	if err != nil {
		s.logger.Error().Err(err).Msg("Failed to fetch chain names from DefiLlama")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		s.logger.Error().Msgf("DefiLlama API request failed with status code: %d", resp.StatusCode)
		return
	}

	var chains []struct {
		Name    string      `json:"name"`
		ChainID interface{} `json:"chainId"`
		TVL     float64     `json:"tvl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&chains); err != nil {
		s.logger.Error().Err(err).Msg("Failed to decode DefiLlama chain names response")
		return
	}

	tempChainNames := make(map[string]string)
	chainIdToTVL := make(map[string]float64)

	for _, chain := range chains {
		var c string
		switch v := chain.ChainID.(type) {
		case string:
			c = v
		case float64:
			c = strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			c = strconv.Itoa(v)
		}

		if c != "" {
			if existingTVL, exists := chainIdToTVL[c]; !exists || chain.TVL > existingTVL {
				tempChainNames[c] = chain.Name
				chainIdToTVL[c] = chain.TVL
			}
		}
	}

	cachedChainNamesBytes, _ := json.Marshal(tempChainNames)
	cachedChainNames = string(cachedChainNamesBytes)
	s.redisClient.Client.Set(context.Background(), cacheKey, cachedChainNames, 3*24*time.Hour)

	chainNames = tempChainNames
}

func (s *defiLlamaService) getChainNameById(chainId string) (string, error) {
	if len(chainNames) == 0 {
		s.initializeChainNames()
	}

	if name, exists := chainNames[chainId]; exists {
		return name, nil
	}
	s.logger.Debug().Msg("defiLlama chainId:" + chainId + " not found")
	return "", nil
}

func (s *defiLlamaService) ensureCoinExists(chainId, address string, attributes map[string]interface{}) error {
	coinId := chainId + "_" + address
	existingCoin, err := s.coinRepository.GetCoinsByOneID(coinId)
	if err != nil {
		return err
	}
	if existingCoin == nil || existingCoin.ID == "" {
		newCoin := schema.Coins{
			ID:          coinId,
			Address:     address,
			ChainID:     chainId,
			Symbol:      shared.GetStringPtr(attributes["symbol"]),
			Name:        shared.GetStringPtr(attributes["name"]),
			Decimals:    shared.GetIntPtr(attributes["decimals"]),
			TotalSupply: shared.GetStringPtr(attributes["total_supply"]),
			PriceSource: shared.GetStringPtr("defillama"),
		}
		return s.coinRepository.UpsertCoins([]schema.Coins{newCoin})
	}
	return nil
}

func (s *defiLlamaService) GetCurrentPrice(chainId, address string, isCache bool) (*string, error) {
	chainName, err := s.getChainNameById(chainId)
	if err != nil || chainName == "" {
		return nil, err
	}

	coinID := chainId + "_" + address

	if isCache {
		cachedPrice, err := s.redisClient.GetCurrentPriceCache(coinID)
		if err == nil && cachedPrice != "" {
			return &cachedPrice, nil
		}
	}

	url := fmt.Sprintf("%s/prices/current/%s:%s", defiLlamaBaseURL, chainName, address)
	headers := map[string]string{
		"accept": "application/json",
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 10) // 传递 10 秒超时
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		if statusCode != http.StatusTooManyRequests {
			shared.HandleErrorWithThrottling(s.redisClient, s.logger, "DefiLlamaService-GetCurrentPrice", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
		}
		return nil, fmt.Errorf("failed to get prices, status code: %d, response: %s", statusCode, string(body))
	}

	var result struct {
		Coins map[string]struct {
			Price  float64 `json:"price"`
			Symbol string  `json:"symbol"`
		} `json:"coins"`
	}

	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return nil, err
	}

	priceData, ok := result.Coins[fmt.Sprintf("%s:%s", chainName, address)]
	if !ok {
		s.logger.Debug().Msg(fmt.Sprintf("%s price not found", fmt.Sprintf("%s:%s", chainName, address)))
		return nil, nil
	}

	priceStr := strconv.FormatFloat(priceData.Price, 'f', -1, 64)

	// 确保 coinID 存在
	err = s.ensureCoinExists(chainId, address, map[string]interface{}{
		"symbol":       priceData.Symbol,
		"name":         "",
		"decimals":     nil,
		"total_supply": nil,
	})
	if err != nil {
		return nil, err
	}

	s.redisClient.SetCurrentPriceCache(coinID, priceStr)

	priceToSave := schema.CoinHistoricalPrice{
		CoinID:  coinID,
		Date:    time.Now().Unix(),
		DayDate: time.Now().Format("02-01-2006"),
		Price:   priceStr,
		Source:  "defillama",
	}
	s.coinHistoricalPriceRepo.SaveHistoricalPrices([]schema.CoinHistoricalPrice{priceToSave})

	return &priceStr, nil
}

func (s *defiLlamaService) GetHistoricalPrice(chainId, address string, unixTimeStamp int64) (*string, error) {
	chainName, err := s.getChainNameById(chainId)
	if err != nil || chainName == "" {
		return nil, err
	}

	coinID := chainId + "_" + address
	date := time.Unix(unixTimeStamp, 0).Format("02-01-2006")

	historicalPrices, err := s.coinHistoricalPriceRepo.GetHistoricalPrices([]string{coinID}, []int64{unixTimeStamp})
	if err == nil {
		if price, exists := historicalPrices[coinID+"_"+date]; exists {
			return &price, nil
		}
	}

	url := fmt.Sprintf("%s/prices/historical/%d/%s:%s", defiLlamaBaseURL, unixTimeStamp, chainName, address)
	headers := map[string]string{
		"accept": "application/json",
	}

	body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 10) // 指定 10 秒超时
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		if statusCode != http.StatusTooManyRequests {
			shared.HandleErrorWithThrottling(s.redisClient, s.logger, "DefiLlamaService-GetHistoricalPrice", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
		}
		return nil, fmt.Errorf("failed to get prices, status code: %d, response: %s", statusCode, string(body))
	}

	var result struct {
		Coins map[string]struct {
			Price     float64 `json:"price"`
			Symbol    string  `json:"symbol"`
			Timestamp int64   `json:"timestamp"`
		} `json:"coins"`
	}

	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return nil, err
	}

	priceData, ok := result.Coins[fmt.Sprintf("%s:%s", chainName, address)]
	if !ok {
		s.logger.Debug().Msg(fmt.Sprintf("%s historical price not found", fmt.Sprintf("%s:%s:%d", chainName, address, unixTimeStamp)))
		return nil, nil
	}

	priceStr := strconv.FormatFloat(priceData.Price, 'f', -1, 64)

	// 确保 coinID 存在
	err = s.ensureCoinExists(chainId, address, map[string]interface{}{
		"symbol":       priceData.Symbol,
		"name":         "",
		"decimals":     nil,
		"total_supply": nil,
	})
	if err != nil {
		return nil, err
	}

	priceToSave := schema.CoinHistoricalPrice{
		CoinID:  coinID,
		Date:    unixTimeStamp,
		DayDate: date,
		Price:   priceStr,
		Source:  "defillama",
	}
	s.coinHistoricalPriceRepo.SaveHistoricalPrices([]schema.CoinHistoricalPrice{priceToSave})

	return &priceStr, nil
}

func (s *defiLlamaService) GetBatchCurrentPrices(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error) {
	if len(chainIds) != len(addresses) {
		return nil, fmt.Errorf("chainIds 和 addresses 的长度必须相同")
	}

	results := make([]PriceResult, len(addresses))
	coinIDs := make([]string, len(addresses))
	coinsToFetchMap := make(map[string]int)

	for i, address := range addresses {
		coinID := chainIds[i] + "_" + address
		coinIDs[i] = coinID

		chainName, err := s.getChainNameById(chainIds[i])
		if err != nil || chainName == "" {
			return nil, err
		}

		coinKey := fmt.Sprintf("%s:%s", chainName, address)
		coinsToFetchMap[coinKey] = i
	}

	var cachedPrices map[string]string
	if isCache {
		cachedPrices, _ = s.redisClient.GetCurrentPricesCache(coinIDs)
	}

	coinsToFetch := make([]string, 0)
	for i, coinID := range coinIDs {
		if cachedPrice, exists := cachedPrices[coinID]; exists && isCache {
			results[i] = PriceResult{
				ChainID:   chainIds[i],
				Address:   addresses[i],
				Price:     &cachedPrice,
				Symbol:    GetOrNil(symbols, i),
				Network:   GetOrNil(networks, i),
				TimeStamp: strconv.FormatInt(time.Now().Unix(), 10),
			}
		} else {
			chainName, _ := s.getChainNameById(chainIds[i])
			if chainName != "" {
				coinsToFetch = append(coinsToFetch, fmt.Sprintf("%s:%s", chainName, addresses[i]))
			}
		}
	}

	if len(coinsToFetch) > 0 {
		const batchSize = 50
		var wg sync.WaitGroup
		var mu sync.Mutex

		for start := 0; start < len(coinsToFetch); start += batchSize {
			end := start + batchSize
			if end > len(coinsToFetch) {
				end = len(coinsToFetch)
			}
			batch := coinsToFetch[start:end]

			wg.Add(1)
			go func(batch []string) {
				defer wg.Done()

				url := fmt.Sprintf("%s/prices/current/%s", defiLlamaBaseURL, strings.Join(batch, ","))
				headers := map[string]string{
					"accept": "application/json",
				}

				body, statusCode, err := shared.DoRequest(http.DefaultClient, url, headers, 10) // 指定 10 秒超时
				if err != nil {
					s.logger.Error().Err(err).Msgf("Failed to fetch prices for url %s", url)
					return
				}

				if statusCode != http.StatusOK {
					if statusCode != http.StatusTooManyRequests {
						shared.HandleErrorWithThrottling(s.redisClient, s.logger, "DefiLlamaService-GetBatchCurrentPrices", fmt.Sprintf("url: %s, status code: %d, response: %s", url, statusCode, string(body)))
					}
					s.logger.Error().Msgf("Failed to get prices, status code: %d, response: %s", statusCode, string(body))
					return
				}

				var apiResult struct {
					Coins map[string]struct {
						Price  float64 `json:"price"`
						Symbol string  `json:"symbol"`
					} `json:"coins"`
				}

				if err := shared.ParseJSONResponse(body, &apiResult); err != nil {
					s.logger.Error().Err(err).Msgf("Failed to decode response for url %s", url)
					return
				}

				mu.Lock()
				defer mu.Unlock()
				for coinKey, data := range apiResult.Coins {
					i := coinsToFetchMap[coinKey]
					priceStr := strconv.FormatFloat(data.Price, 'f', -1, 64)
					results[i] = PriceResult{
						ChainID:   chainIds[i],
						Address:   addresses[i],
						Price:     &priceStr,
						Symbol:    GetOrNil(symbols, i),
						Network:   GetOrNil(networks, i),
						TimeStamp: strconv.FormatInt(time.Now().Unix(), 10),
					}

					// 确保 coinID 存在
					err = s.ensureCoinExists(chainIds[i], addresses[i], map[string]interface{}{
						"symbol":       data.Symbol,
						"name":         "",
						"decimals":     nil,
						"total_supply": nil,
					})
					if err != nil {
						s.logger.Error().Err(err).Msgf("Failed to ensure coin exists for %s", coinIDs[i])
						continue
					}

					coinID := chainIds[i] + "_" + addresses[i]
					s.redisClient.SetCurrentPriceCache(coinID, priceStr)

					priceToSave := schema.CoinHistoricalPrice{
						CoinID:  coinID,
						Date:    time.Now().Unix(),
						DayDate: time.Now().Format("02-01-2006"),
						Price:   priceStr,
						Source:  "defillama",
					}
					s.coinHistoricalPriceRepo.SaveHistoricalPrices([]schema.CoinHistoricalPrice{priceToSave})
				}
			}(batch)
		}
		wg.Wait()
	}
	return results, nil
}
func (s *defiLlamaService) GetBatchHistoricalPrices(addresses []string, chainIds []string, symbols []string, networks []string, unixTimeStamps []int64) ([]PriceResult, error) {
	if len(chainIds) != len(addresses) || len(addresses) != len(unixTimeStamps) {
		return nil, fmt.Errorf("chainIds, addresses and unixTimeStamps must have the same length")
	}

	results := make([]PriceResult, len(addresses))
	var wg sync.WaitGroup
	errCh := make(chan error, len(addresses))

	for i := range addresses {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			price, err := s.GetHistoricalPrice(chainIds[i], addresses[i], unixTimeStamps[i])
			requestStatus := "200"
			if err != nil {
				if strings.Contains(err.Error(), "429") {
					requestStatus = "429"
				}
			}
			results[i] = PriceResult{
				ChainID:       chainIds[i],
				Address:       addresses[i],
				Price:         price,
				Symbol:        GetOrNil(symbols, i),
				Network:       GetOrNil(networks, i),
				TimeStamp:     strconv.FormatInt(unixTimeStamps[i], 10),
				RequestStatus: &requestStatus,
			}
		}(i)
	}
	wg.Wait()
	close(errCh)

	return results, nil
}
