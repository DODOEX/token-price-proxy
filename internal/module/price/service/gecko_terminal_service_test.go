package service_test

import (
	"testing"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGeckoTerminalService() service.GeckoTerminalService {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	return service.NewGeckoTerminalService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo)
}

func TestGetCurrentPrice_Valid(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 有效的查询
	price, err := geckoTerminalService.GetCurrentPrice("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", false)
	require.NoError(t, err)
	assert.NotNil(t, price)
	t.Logf("Valid Price: %v", *price)
}

func TestGetCurrentPrice_Invalid(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 无效的链ID或地址
	price, _ := geckoTerminalService.GetCurrentPrice("9999", "0xInvalidAddress", false)
	assert.Nil(t, price)
}

func TestGetHistoricalPrice_Valid1(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 有效的历史价格查询
	timestamp := time.Now().Add(-24 * time.Hour).Unix()
	price, err := geckoTerminalService.GetHistoricalPrice("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", timestamp)
	require.NoError(t, err)
	assert.NotNil(t, price)
	t.Logf("Valid Historical Price: %v", *price)
}

func TestGetHistoricalPrice_Invalid1(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 无效的链ID或地址
	timestamp := time.Now().Add(-24 * time.Hour).Unix()
	price, _ := geckoTerminalService.GetHistoricalPrice("9999", "0xInvalidAddress", timestamp)
	assert.Nil(t, price)
}

func TestGetBatchCurrentPrices_Valid(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 有效的批量查询
	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}

	prices, _ := geckoTerminalService.GetBatchCurrentPrices(addresses, chainIds, symbols, networks, false)
	assert.NotNil(t, prices[0].Price)
	t.Logf("Batch Valid Prices: %v", prices)
}

func TestGetBatchCurrentPrices_Invalid(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 无效的批量查询
	addresses := []string{"0xInvalidAddress"}
	chainIds := []string{"9999"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}

	prices, _ := geckoTerminalService.GetBatchCurrentPrices(addresses, chainIds, symbols, networks, false)
	assert.Nil(t, prices[0].Price)
	t.Logf("Batch Invalid Prices: %v", prices)
}

func TestGetBatchHistoricalPrices_Valid(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 有效的批量历史价格查询
	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-24 * time.Hour).Unix()}

	prices, _ := geckoTerminalService.GetBatchHistoricalPrices(addresses, chainIds, symbols, networks, timestamps)
	assert.NotNil(t, prices[0].Price)
	t.Logf("Batch Valid Historical Prices: %v", prices)
}

func TestGetBatchHistoricalPrices_Invalid(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 无效的批量历史价格查询
	addresses := []string{"0x1"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-24 * time.Hour).Unix()}

	prices, _ := geckoTerminalService.GetBatchHistoricalPrices(addresses, chainIds, symbols, networks, timestamps)
	assert.Nil(t, prices[0].Price)
	t.Logf("Batch Invalid Historical Prices: %v", prices)
}

func TestEdgeCases1(t *testing.T) {
	geckoTerminalService := setupGeckoTerminalService()

	// 测试空数组
	prices, err := geckoTerminalService.GetBatchCurrentPrices([]string{}, []string{}, []string{}, []string{}, false)
	require.NoError(t, err)
	assert.Empty(t, prices)
	t.Log("Empty array test passed.")

	// 测试边界时间戳
	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{0} // 边界时间戳

	prices, _ = geckoTerminalService.GetBatchHistoricalPrices(addresses, chainIds, symbols, networks, timestamps)
	assert.Nil(t, prices[0].Price)
	t.Log("Boundary timestamp test passed.")
}
