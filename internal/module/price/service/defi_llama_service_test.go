package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDefiLlamaService() service.DefiLlamaService {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	//删除缓存测试
	redis.Client.Del(context.Background(), "defiLlama:"+"chainNamesAndTVL")
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	return service.NewDefiLlamaService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo)
}

func TestGetCurrentPrice(t *testing.T) {
	defiLlamaService := setupDefiLlamaService()

	// 测试使用有效的链 ID 和地址
	price, err := defiLlamaService.GetCurrentPrice("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", false)
	require.NoError(t, err)
	assert.NotNil(t, price)

	// 测试使用无效的链 ID 和地址
	price, _ = defiLlamaService.GetCurrentPrice("1", "0xInvalidAddress", false)
	assert.Nil(t, price)

	// 测试使用有效的链 ID 和地址
	price, err = defiLlamaService.GetCurrentPrice("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", false)
	require.NoError(t, err)
	assert.NotNil(t, price)

}

func TestGetHistoricalPrice(t *testing.T) {
	defiLlamaService := setupDefiLlamaService()

	timestamp := time.Now().Add(-24 * time.Hour).Unix()

	// 测试使用有效的链 ID 和地址
	price, err := defiLlamaService.GetHistoricalPrice("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", timestamp)
	require.NoError(t, err)
	assert.NotNil(t, price)

	// 测试使用无效的链 ID 和地址
	price, _ = defiLlamaService.GetHistoricalPrice("1", "0xInvalidAddress", timestamp)
	assert.Nil(t, price)
}

func TestGetBatchCurrentPrices1(t *testing.T) {
	defiLlamaService := setupDefiLlamaService()

	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}

	// 测试批量获取当前价格
	prices, err := defiLlamaService.GetBatchCurrentPrices(addresses, chainIds, symbols, networks, false)
	require.NoError(t, err)
	assert.NotEmpty(t, prices)
	assert.NotNil(t, prices[0].Price)

	// 测试无效的地址
	invalidAddresses := []string{"0xInvalidAddress"}
	prices, _ = defiLlamaService.GetBatchCurrentPrices(invalidAddresses, chainIds, symbols, networks, false)
	assert.Nil(t, prices[0].Price)
}

func TestGetBatchHistoricalPrices1(t *testing.T) {
	defiLlamaService := setupDefiLlamaService()

	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-24 * time.Hour).Unix()}

	// 测试批量获取历史价格
	prices, err := defiLlamaService.GetBatchHistoricalPrices(addresses, chainIds, symbols, networks, timestamps)
	require.NoError(t, err)
	assert.NotEmpty(t, prices)
	assert.NotNil(t, prices[0].Price)

	// 测试无效的地址
	invalidAddresses := []string{"0xInvalidAddress"}
	prices, _ = defiLlamaService.GetBatchHistoricalPrices(invalidAddresses, chainIds, symbols, networks, timestamps)
	assert.Nil(t, prices[0].Price)
}
