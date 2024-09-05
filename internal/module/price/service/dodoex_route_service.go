package service

import (
	"encoding/json"
	"fmt"
	"math/big"
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

type DodoexRouteService interface {
	GetCurrentPrice(address string, chainId string, isCache bool) (*string, error)
	GetBatchCurrentPrices(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error)
}

type dodoexRouteService struct {
	config                  *koanf.Koanf
	redisClient             *shared.RedisClient
	logger                  zerolog.Logger
	coinRepository          repository.CoinRepository
	coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository
	client                  *http.Client
}

func NewDodoexRouteService(cfg *koanf.Koanf, redisClient *shared.RedisClient, logger zerolog.Logger, coinRepository repository.CoinRepository, coinHistoricalPriceRepo repository.CoinHistoricalPriceRepository) DodoexRouteService {
	return &dodoexRouteService{
		config:                  cfg,
		redisClient:             redisClient,
		logger:                  logger,
		coinRepository:          coinRepository,
		coinHistoricalPriceRepo: coinHistoricalPriceRepo,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *dodoexRouteService) GetCurrentPrice(address string, chainId string, isCache bool) (*string, error) {
	usdtAddress, err := shared.GetUSDTAddress(chainId)
	if err != nil || usdtAddress.Address == "" || usdtAddress.Decimal == 0 {
		return nil, err
	}

	coinID := chainId + "_" + address

	// 检查缓存
	if isCache {
		cachedPrice, err := s.redisClient.GetCurrentPricesCache([]string{coinID})
		if err == nil {
			if price, exists := cachedPrice[coinID]; exists {
				return &price, nil
			}
		}
	}

	// 计算 fromAmount = 100 * 10^usdtAddress.Decimal 0x43dfc4159d86f3a37a5a4b3d4580b888ad7d4ddd
	fromAmount := new(big.Float).Mul(big.NewFloat(100), new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(usdtAddress.Decimal)), nil))).Text('f', 0)
	url := fmt.Sprintf("%s?fromTokenAddress=%s&toTokenAddress=%s&fromAmount=%s&slippage=1&userAddr=0x0000000000000000000000000000000000000000&chainId=%s", shared.DodoexRouteUrl, usdtAddress.Address, address, fromAmount, chainId)
	headers := map[string]string{
		"accept": "application/json",
	}

	body, statusCode, err := shared.DoRequest(s.client, url, headers, 10) // 使用默认超时
	if err != nil {
		return nil, fmt.Errorf("failed to get token price: %w", err)
	}

	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get token price, status code: %d", statusCode)
	}

	var result struct {
		Status  int             `json:"status"`
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}

	// 检查响应是否为有效的 JSON
	if !json.Valid(body) {
		return nil, fmt.Errorf("invalid JSON response: %s", string(body))
	}

	if err := shared.ParseJSONResponse(body, &result); err != nil {
		return nil, fmt.Errorf("failed to decode token price response: %w", err)
	}

	if result.Status != 200 {
		s.logger.Debug().Msg(fmt.Sprintf("DodoexRouteService-GetCurrentPrice failed, status: %d, url: %s", result.Status, url))
		return nil, nil
	}

	var priceData struct {
		ResAmount            float64 `json:"resAmount"`
		ResPricePerToToken   float64 `json:"resPricePerToToken"`
		ResPricePerFromToken float64 `json:"resPricePerFromToken"`
		PriceImpact          float64 `json:"priceImpact"`
		UseSource            string  `json:"useSource"`
		TargetDecimals       int     `json:"targetDecimals"`
		TargetApproveAddr    string  `json:"targetApproveAddr"`
		To                   string  `json:"to"`
		Data                 string  `json:"data"`
		MinReturnAmount      string  `json:"minReturnAmount"`
		GasLimit             string  `json:"gasLimit"`
		RouteInfo            struct {
			SubRouteTotalPart int `json:"subRouteTotalPart"`
			SubRoute          []struct {
				MidPathPart int `json:"midPathPart"`
				MidPath     []struct {
					FromToken         string `json:"fromToken"`
					ToToken           string `json:"toToken"`
					OneSplitTotalPart int    `json:"oneSplitTotalPart"`
					PoolDetails       []struct {
						PoolName string `json:"poolName"`
						Pool     string `json:"pool"`
						PoolPart int    `json:"poolPart"`
					} `json:"poolDetails"`
				} `json:"midPath"`
			} `json:"subRoute"`
		} `json:"routeInfo"`
		Value string `json:"value"`
		ID    string `json:"id"`
	}

	if err := json.Unmarshal(result.Data, &priceData); err != nil {
		return nil, fmt.Errorf("failed to decode token price data: %w", err)
	}

	if priceData.PriceImpact > 0.1 {
		return nil, fmt.Errorf("price impact too high: %f", priceData.PriceImpact)
	}

	price := strconv.FormatFloat(priceData.ResPricePerToToken, 'f', -1, 64)

	// 保存缓存和历史记录
	s.redisClient.SetCurrentPriceCache(coinID, price)
	if price != "" {
		priceToSave := schema.CoinHistoricalPrice{
			CoinID:  coinID,
			Date:    time.Now().Unix(),
			DayDate: time.Now().Format("02-01-2006"),
			Price:   price,
			Source:  "dodoexRoute",
		}
		s.coinHistoricalPriceRepo.SaveHistoricalPrices([]schema.CoinHistoricalPrice{priceToSave})
	}

	return &price, nil
}

func (s *dodoexRouteService) GetBatchCurrentPrices(addresses []string, chainIds []string, symbols []string, networks []string, isCache bool) ([]PriceResult, error) {
	if len(chainIds) != len(addresses) || len(addresses) != len(symbols) || len(symbols) != len(networks) {
		return nil, fmt.Errorf("all input slices must have the same length")
	}

	results := make([]PriceResult, len(addresses))
	var wg sync.WaitGroup
	errCh := make(chan error, len(addresses))

	for i := range chainIds {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			price, err := s.GetCurrentPrice(addresses[i], chainIds[i], isCache)
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
				Symbol:        &symbols[i],
				Network:       &networks[i],
				TimeStamp:     fmt.Sprintf("%d", time.Now().Unix()),
				RequestStatus: &requestStatus,
				Serial:        i,
			}
			results[i] = priceResult
		}(i)
	}

	wg.Wait()
	close(errCh)

	return results, nil
}
