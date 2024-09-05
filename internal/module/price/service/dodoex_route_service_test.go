package service_test

import (
	"testing"

	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDodoexRouteService() service.DodoexRouteService {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	shared.LoadEnv()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	return service.NewDodoexRouteService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo)
}

func TestGetCurrentPrice1(t *testing.T) {
	dodoexRouteService := setupDodoexRouteService()

	// 测试使用有效的链 ID 和地址
	price, err := dodoexRouteService.GetCurrentPrice("0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "1", false)
	t.Logf("price: " + *price)
	require.NoError(t, err)
	assert.NotNil(t, price)

	// 测试使用无效的链 ID 和地址
	price, _ = dodoexRouteService.GetCurrentPrice("0xInvalidAddress", "1", false)
	assert.Nil(t, price)
}

func TestGetBatchCurrentPrices(t *testing.T) {
	dodoexRouteService := setupDodoexRouteService()

	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}

	// 测试批量获取当前价格
	prices, err := dodoexRouteService.GetBatchCurrentPrices(chainIds, addresses, symbols, networks, false)
	require.NoError(t, err)
	assert.NotEmpty(t, prices)
	// 输出数组内容
	t.Logf("Prices: %v", prices)
	for _, price := range prices {
		t.Logf("ChainID: %s, Address: %s, Price: %v, Symbol: %v, Network: %v", price.ChainID, price.Address, *price.Price, *price.Symbol, *price.Network)
	}
	assert.NotNil(t, prices[0].Price)

	// 测试无效的地址
	invalidAddresses := []string{"0xInvalidAddress"}
	prices, _ = dodoexRouteService.GetBatchCurrentPrices(chainIds, invalidAddresses, symbols, networks, false)
	assert.Nil(t, prices[0].Price)
}

func TestGetBatchCurrentPricesMismatchedLengths(t *testing.T) {
	dodoexRouteService := setupDodoexRouteService()

	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH", "BTC"} // Length mismatch
	networks := []string{"ethereum"}

	// 测试输入切片长度不匹配
	_, err := dodoexRouteService.GetBatchCurrentPrices(chainIds, addresses, symbols, networks, false)
	require.Error(t, err)
}
