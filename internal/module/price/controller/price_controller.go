package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

type priceController struct {
	coinGeckoService service.CoinGeckoService
	priceService     service.PriceService
	requestLogRepo   repository.RequestLogRepository
	logger           zerolog.Logger
}

type PriceController interface {
	GetCoinList(ctx *fasthttp.RequestCtx)
	SyncCoins(ctx *fasthttp.RequestCtx)
	GetBatchPrice(ctx *fasthttp.RequestCtx)
	GetBatchHistoricalPrice(ctx *fasthttp.RequestCtx)
	GetPrice(ctx *fasthttp.RequestCtx)
	GetHistoricalPrice(ctx *fasthttp.RequestCtx)
}

func NewPriceController(priceService service.PriceService, coinGeckoService service.CoinGeckoService, requestLogRepo repository.RequestLogRepository, logger zerolog.Logger) PriceController {
	return &priceController{
		coinGeckoService: coinGeckoService,
		priceService:     priceService,
		requestLogRepo:   requestLogRepo,
		logger:           logger,
	}
}

func (_i *priceController) respond(ctx *fasthttp.RequestCtx, code int, data interface{}, message string) {
	response := map[string]interface{}{
		"code":    code,
		"data":    data,
		"message": message,
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		ctx.Error("failed to serialize response ", fasthttp.StatusInternalServerError)
		return
	}
	ctx.Response.Header.Set("Content-Type", "application/json; charset=utf-8")
	ctx.Response.SetBody(responseBody)
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
}
func (_i *priceController) logRequest(ctx *fasthttp.RequestCtx, endpoint, requestParams, response string, executionTime int64) {
	log := schema.RequestLog{
		IPAddress:     ctx.RemoteIP().String(),
		Endpoint:      endpoint,
		RequestParams: requestParams,
		Response:      response,
		ExecutionTime: executionTime,
	}
	_i.requestLogRepo.InsertLog(context.Background(), log)
}
func (_i *priceController) withTimeout(ctx *fasthttp.RequestCtx, fn func(context.Context) error) {
	// 创建带有超时时间的上下文
	c, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	// 创建一个通道来接收函数执行结果
	done := make(chan error, 1)
	go func() {
		done <- fn(c)
	}()

	select {
	case <-c.Done():
		if c.Err() == context.DeadlineExceeded {
			_i.respond(ctx, 504, nil, "Request timed out")
		} else {
			_i.respond(ctx, 500, nil, "Request canceled")
		}
	case err := <-done:
		if err != nil {
			_i.logger.Err(err).Msg("failed to retrieve price")
			_i.respond(ctx, 500, nil, err.Error())
		}
	}
}
func (_i *priceController) GetCoinList(ctx *fasthttp.RequestCtx) {
	useCache := true
	coins, err := _i.coinGeckoService.CoinsList(useCache)
	if err != nil {
		_i.respond(ctx, 500, nil, "failed to retrieve token list")
		return
	}
	_i.respond(ctx, 0, coins, "Request successful")
}

func (_i *priceController) SyncCoins(ctx *fasthttp.RequestCtx) {
	err := _i.coinGeckoService.SyncCoins()
	if err != nil {
		_i.respond(ctx, 500, nil, "failed to synchronize tokens")
		return
	}
	_i.respond(ctx, 0, nil, "Tokens synchronized successfully")
}

func (_i *priceController) GetBatchPrice(ctx *fasthttp.RequestCtx) {
	_i.withTimeout(ctx, func(c context.Context) error {
		startTime := time.Now()
		var addresses, chainIds, symbols, networks []string
		var isCache bool = true // 默认值为 true
		var excludeRoute bool = true
		defer func() {
			_i.logger.Debug().Dur("execution_time", time.Since(startTime)).Msg("GetBatchPrice executed")

			// 创建请求参数的 map
			requestParamsMap := map[string]interface{}{
				"addresses":    addresses,
				"networks":     networks,
				"chainIds":     chainIds,
				"symbols":      symbols,
				"isCache":      isCache,
				"excludeRoute": excludeRoute,
			}

			// 将请求参数 map 转换为 JSON
			requestParamsJSON, err := json.Marshal(requestParamsMap)
			if err != nil {
				_i.logger.Error().Err(err).Msg("JSON marshaling of requestParams failed")
			} else {
				requestParams := string(requestParamsJSON)
				_i.logRequest(ctx, "GetBatchPrice", requestParams, string(ctx.Response.Body()), time.Since(startTime).Milliseconds())
			}
		}()

		if string(ctx.Method()) == fasthttp.MethodGet {
			addresses = convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("addresses"))
			networks = convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("networks"))
			symbols = convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("symbols"))
			chainIds = convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("chainIds"))
			if ctx.QueryArgs().Has("isCache") {
				isCache = string(ctx.QueryArgs().Peek("isCache")) != "false"
			}
			if ctx.QueryArgs().Has("excludeRoute") {
				excludeRoute = string(ctx.QueryArgs().Peek("excludeRoute")) != "false"
			}
		} else if string(ctx.Method()) == fasthttp.MethodPost {
			var requestData struct {
				Addresses    []string `json:"addresses"`
				Networks     []string `json:"networks"`
				Symbols      []string `json:"symbols"`
				ChainIds     []string `json:"chainIds"`
				IsCache      *bool    `json:"isCache"`
				ExcludeRoute *bool    `json:"excludeRoute"`
			}
			if err := json.Unmarshal(ctx.PostBody(), &requestData); err != nil {
				return err
			}
			addresses = requestData.Addresses
			networks = requestData.Networks
			symbols = requestData.Symbols
			chainIds = requestData.ChainIds
			if requestData.IsCache != nil {
				isCache = *requestData.IsCache
			}
			if requestData.ExcludeRoute != nil {
				excludeRoute = *requestData.ExcludeRoute
			}
		} else {
			return fmt.Errorf("Method not supported" + string(ctx.Method()))
		}

		if len(chainIds) == 0 && len(networks) > 0 {
			chainIds = make([]string, len(networks))
			for i, network := range networks {
				chainId, err := shared.GetChainID(network)
				if err != nil {
					return fmt.Errorf("%s Unsupported network", network)
				}
				chainIds[i] = chainId
			}
		} else if len(networks) == 0 && len(chainIds) > 0 {
			networks = make([]string, len(chainIds))
			for i, chainId := range chainIds {
				network, err := shared.GetChainName(chainId)
				if err != nil {
					return fmt.Errorf("%s Unsupported network", chainId)
				}
				networks[i] = network
			}
		}

		if len(addresses) != len(networks) {
			return fmt.Errorf("the lengths of the addresses and networks arrays must be the same")
		}

		chainIds = make([]string, len(networks))
		for i, network := range networks {
			chainId, err := shared.GetChainID(network)
			if err != nil {
				return fmt.Errorf("%s Unsupported network", network)
			}
			chainIds[i] = chainId
		}

		prices, err := _i.priceService.GetBatchPrice(c, chainIds, addresses, symbols, networks, isCache, excludeRoute)
		if err != nil {
			return err
		}
		_i.respond(ctx, 0, prices, "Request successful")
		return nil
	})
}

func (_i *priceController) GetBatchHistoricalPrice(ctx *fasthttp.RequestCtx) {
	_i.withTimeout(ctx, func(c context.Context) error {
		startTime := time.Now()
		var addresses, chainIds, symbols, networks, datesStr []string
		var dates []int64

		defer func() {
			_i.logger.Debug().Dur("execution_time", time.Since(startTime)).Msg("GetBatchHistoricalPrice executed")

			// 创建请求参数的 map
			requestParamsMap := map[string]interface{}{
				"addresses": addresses,
				"chainIds":  chainIds,
				"networks":  networks,
				"symbols":   symbols,
				"dates":     dates,
			}

			// 将请求参数 map 转换为 JSON
			requestParamsJSON, err := json.Marshal(requestParamsMap)
			if err != nil {
				_i.logger.Error().Err(err).Msg("JSON marshaling of requestParams failed")
			} else {
				requestParams := string(requestParamsJSON)
				_i.logRequest(ctx, "GetBatchHistoricalPrice", requestParams, string(ctx.Response.Body()), time.Since(startTime).Milliseconds())
			}
		}()

		if string(ctx.Method()) == fasthttp.MethodGet {
			addresses = convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("addresses"))
			networks = convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("networks"))
			symbols = convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("symbols"))
			chainIds = convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("chainIds"))
			datesStr := convertQueryArgsToStringSlice(ctx.QueryArgs().PeekMulti("dates"))
			dates = make([]int64, len(datesStr))
			for i, ds := range datesStr {
				if len(ds) == 10 && ds[4] == '-' && ds[7] == '-' {
					date, err := time.Parse("2006-01-02", ds)
					if err != nil {
						return fmt.Errorf("failed to parse date: %s", ds)
					}
					dates[i] = date.Unix()
				} else {
					date, err := strconv.ParseInt(ds, 10, 64)
					if err != nil {
						return fmt.Errorf("failed to parse date: %s", ds)
					}
					dates[i] = date
				}
			}
		} else if string(ctx.Method()) == fasthttp.MethodPost {
			var requestData struct {
				Addresses []string      `json:"addresses"`
				Networks  []string      `json:"networks"`
				ChainIds  []string      `json:"chainIds"`
				Symbols   []string      `json:"symbols"`
				Dates     []interface{} `json:"dates"`
			}
			if err := json.Unmarshal(ctx.PostBody(), &requestData); err != nil {
				return err
			}
			addresses = requestData.Addresses
			networks = requestData.Networks
			chainIds = requestData.ChainIds
			symbols = requestData.Symbols
			datesStr = make([]string, len(requestData.Dates))
			dates = make([]int64, len(requestData.Dates))
			for i, d := range requestData.Dates {
				switch v := d.(type) {
				case string:
					datesStr[i] = v
					if len(v) == 10 && v[4] == '-' && v[7] == '-' {
						date, err := time.Parse("2006-01-02", v)
						if err != nil {
							return fmt.Errorf("failed to parse date: %s", v)
						}
						dates[i] = date.Unix()
					} else {
						date, err := strconv.ParseInt(v, 10, 64)
						if err != nil {
							return fmt.Errorf("failed to parse date: %s", v)
						}
						dates[i] = date
					}
				case float64:
					dates[i] = int64(v)
					datesStr[i] = strconv.FormatInt(int64(v), 10)
				default:
					return fmt.Errorf("invalid date format: %v", v)
				}
			}
		} else {
			return fmt.Errorf("Method not supported" + string(ctx.Method()))
		}
		if len(chainIds) == 0 && len(networks) > 0 {
			chainIds = make([]string, len(networks))
			for i, network := range networks {
				chainId, err := shared.GetChainID(network)
				if err != nil {
					return fmt.Errorf("%s Unsupported network", network)
				}
				chainIds[i] = chainId
			}
		} else if len(networks) == 0 && len(chainIds) > 0 {
			networks = make([]string, len(chainIds))
			for i, chainId := range chainIds {
				network, err := shared.GetChainName(chainId)
				if err != nil {
					return fmt.Errorf("%s Unsupported network", chainId)
				}
				networks[i] = network
			}
		}

		if len(addresses) != len(networks) || len(networks) != len(dates) {
			return fmt.Errorf("the lengths of the addresses and networks arrays must be the same")
		}

		chainIds = make([]string, len(networks))
		for i, network := range networks {
			chainId, err := shared.GetChainID(network)
			if err != nil {
				return err
			}
			chainIds[i] = chainId
		}

		results, err := _i.priceService.GetBatchHistoricalPrice(chainIds, addresses, symbols, networks, dates, datesStr)
		if err != nil {
			return err
		}
		_i.respond(ctx, 0, results, "Request successful")
		return nil
	})
}

func (_i *priceController) GetPrice(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	var chainID, address, symbol, network string
	var isCache bool = true // 默认值为 true
	var excludeRoute bool = true
	defer func() {
		_i.logger.Debug().Dur("execution_time", time.Since(startTime)).Msg("GetPrice executed")
		// 创建请求参数的 map
		requestParamsMap := map[string]interface{}{
			"network":      network,
			"address":      address,
			"symbol":       symbol,
			"isCache":      isCache,
			"chainId":      chainID,
			"excludeRoute": excludeRoute,
		}

		// 将请求参数 map 转换为 JSON
		requestParamsJSON, err := json.Marshal(requestParamsMap)
		if err != nil {
			_i.logger.Error().Err(err).Msg("JSON marshaling of requestParams failed")
		} else {
			requestParams := string(requestParamsJSON)
			_i.logRequest(ctx, "GetPrice", requestParams, string(ctx.Response.Body()), time.Since(startTime).Milliseconds())
		}
	}()

	if string(ctx.Method()) == fasthttp.MethodGet {
		network = string(ctx.QueryArgs().Peek("network"))
		chainID = string(ctx.QueryArgs().Peek("chainId"))
		address = string(ctx.QueryArgs().Peek("address"))
		symbol = string(ctx.QueryArgs().Peek("symbol"))
		if ctx.QueryArgs().Has("isCache") {
			isCache = string(ctx.QueryArgs().Peek("isCache")) != "false"
		}
		if ctx.QueryArgs().Has("excludeRoute") {
			excludeRoute = string(ctx.QueryArgs().Peek("excludeRoute")) != "false"
		}
	} else if string(ctx.Method()) == fasthttp.MethodPost {
		var requestData struct {
			Network      string `json:"network"`
			ChainId      string `json:"chainId"`
			Address      string `json:"address"`
			Symbol       string `json:"symbol"`
			IsCache      *bool  `json:"isCache"`
			ExcludeRoute *bool  `json:"excludeRoute"`
		}
		if err := json.Unmarshal(ctx.PostBody(), &requestData); err != nil {
			_i.respond(ctx, 500, nil, "failed to parse request body")
			return
		}
		network = requestData.Network
		chainID = requestData.ChainId
		address = requestData.Address
		symbol = requestData.Symbol
		if requestData.IsCache != nil {
			isCache = *requestData.IsCache
		}
		if requestData.ExcludeRoute != nil {
			excludeRoute = *requestData.ExcludeRoute
		}
	} else {
		_i.respond(ctx, 500, nil, "Method not supported"+string(ctx.Method()))
		return
	}
	if chainID == "" {
		chainIDNew, err := shared.GetChainID(network)
		if err != nil {
			_i.respond(ctx, 500, nil, network+" Unsupported network")
			return
		}
		chainID = chainIDNew
	}
	if network == "" {
		networkNew, err := shared.GetChainName(chainID)
		if err != nil {
			_i.respond(ctx, 500, nil, chainID+" Unsupported network")
			return
		}
		network = networkNew
	}

	price, err := _i.priceService.GetPrice(chainID, address, symbol, network, isCache, excludeRoute)
	if err != nil {
		_i.logger.Err(err).Msg("GetPrice Failed to retrieve single price")
		_i.respond(ctx, 500, nil, "Failed to retrieve single price")
		return
	}

	_i.respond(ctx, 0, price, "Request successful")
}

func (_i *priceController) GetHistoricalPrice(ctx *fasthttp.RequestCtx) {
	startTime := time.Now()
	var chainID, address, symbol, dateStr, network string
	var date int64
	defer func() {
		_i.logger.Debug().Dur("execution_time", time.Since(startTime)).Msg("GetHistoricalPrice executed")

		// 创建请求参数的 map
		requestParamsMap := map[string]interface{}{
			"network": network,
			"chainId": chainID,
			"address": address,
			"symbol":  symbol,
			"date":    date,
		}
		// 将请求参数 map 转换为 JSON
		requestParamsJSON, err := json.Marshal(requestParamsMap)
		if err != nil {
			_i.logger.Error().Err(err).Msg("JSON marshaling of requestParams failed")
		} else {
			requestParams := string(requestParamsJSON)
			_i.logRequest(ctx, "GetHistoricalPrice", requestParams, string(ctx.Response.Body()), time.Since(startTime).Milliseconds())
		}
	}()
	var err error

	if string(ctx.Method()) == fasthttp.MethodGet {
		network = string(ctx.QueryArgs().Peek("network"))
		chainID = string(ctx.QueryArgs().Peek("chainId"))
		address = string(ctx.QueryArgs().Peek("address"))
		symbol = string(ctx.QueryArgs().Peek("symbol"))
		dateStr = string(ctx.QueryArgs().Peek("date"))
		if len(dateStr) == 10 && dateStr[4] == '-' && dateStr[7] == '-' {
			parsedDate, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				_i.respond(ctx, 500, nil, "failed to parse date "+dateStr)
				return
			}
			date = parsedDate.Unix()
		} else {
			date, err = strconv.ParseInt(dateStr, 10, 64)
			if err != nil {
				_i.respond(ctx, 500, nil, "failed to parse date "+dateStr)
				return
			}
		}
	} else if string(ctx.Method()) == fasthttp.MethodPost {
		var requestData struct {
			Network string      `json:"network"`
			ChainID string      `json:"chainId"`
			Address string      `json:"address"`
			Symbol  string      `json:"symbol"`
			Date    interface{} `json:"date"`
		}
		if err := json.Unmarshal(ctx.PostBody(), &requestData); err != nil {
			_i.respond(ctx, 500, nil, "failed to parse request body")
			return
		}
		network = requestData.Network
		chainID = requestData.ChainID
		address = requestData.Address
		symbol = requestData.Symbol
		switch v := requestData.Date.(type) {
		case string:
			if len(v) == 10 && v[4] == '-' && v[7] == '-' {
				parsedDate, err := time.Parse("2006-01-02", v)
				if err != nil {
					_i.respond(ctx, 500, nil, "failed to parse date "+v)
					return
				}
				date = parsedDate.Unix()
			} else {
				date, err = strconv.ParseInt(v, 10, 64)
				if err != nil {
					_i.respond(ctx, 500, nil, "failed to parse date "+v)
					return
				}
			}
		case float64:
			date = int64(v)
		default:
			_i.respond(ctx, 500, nil, "invalid date format  "+v.(string))
			return
		}
	} else {
		_i.respond(ctx, 500, nil, "Method not supported"+string(ctx.Method()))
		return
	}

	if chainID == "" {
		chainIDNew, err := shared.GetChainID(network)
		if err != nil {
			_i.respond(ctx, 500, nil, network+" Unsupported network")
			return
		}
		chainID = chainIDNew
	}
	if network == "" {
		networkNew, err := shared.GetChainName(chainID)
		if err != nil {
			_i.respond(ctx, 500, nil, chainID+" Unsupported network")
			return
		}
		network = networkNew
	}

	price, err := _i.priceService.GetHistoricalPrice(chainID, address, symbol, network, date)
	if err != nil {
		_i.logger.Err(err).Msg("GetHistoricalPrice Failed to retrieve historical price")
		_i.respond(ctx, 500, nil, "Failed to retrieve historical price")
		return
	}

	_i.respond(ctx, 0, price, "Request successful")
}

func convertQueryArgsToStringSlice(args [][]byte) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		result[i] = string(arg)
	}
	return result
}
