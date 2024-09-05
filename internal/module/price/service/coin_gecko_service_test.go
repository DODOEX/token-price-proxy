package service_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCoinGeckoService() service.CoinGeckoService {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	redis.Client.Del(context.Background(), "coingecko:asset_platforms_chain_id_map")
	redis.Client.Del(context.Background(), "coingecko:asset_platforms")
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	coinRepo.RefreshCoinListCache([]string{"1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"})
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	return service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
}

func TestGetAssetPlatforms(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	platforms, err := coinGeckoService.GetAssetPlatformIdByChainId("1")
	require.NoError(t, err)
	assert.NotNil(t, platforms)
	platforms, err = coinGeckoService.GetAssetPlatformIdByChainId("1")
	require.NoError(t, err)
	assert.NotNil(t, platforms)
	//删除缓存再来一次
	platforms, err = coinGeckoService.GetAssetPlatformIdByChainId("1")
	require.NoError(t, err)
	assert.NotNil(t, platforms)
}

func TestGetCoinGeckoChainIdByAssetPlatformId(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	chainID, err := coinGeckoService.GetCoinGeckoChainIdByAssetPlatformId("ethereum")
	require.NoError(t, err)
	assert.Equal(t, "1", chainID)

	chainID, err = coinGeckoService.GetCoinGeckoChainIdByAssetPlatformId("ethereum")
	require.NoError(t, err)
	assert.Equal(t, "1", chainID)

	chainID, err = coinGeckoService.GetCoinGeckoChainIdByAssetPlatformId("ethereum")
	require.NoError(t, err)
	assert.Equal(t, "1", chainID)
}

func TestCoinsList(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	coins, err := coinGeckoService.CoinsList(false)
	require.NoError(t, err)
	assert.NotEmpty(t, coins)
}

func TestGetBatchPrice(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}

	prices, err := coinGeckoService.GetBatchPrice(addresses, chainIds, symbols, networks, false)
	require.NoError(t, err)
	assert.NotEmpty(t, prices)
	fmt.Printf("TestGetBatchPrice prices: %v\n", prices)
	assert.NotNil(t, prices[0].Price)
}

func TestGetBatchHistoricalPrices(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-158 * time.Hour).Unix()}

	prices, err := coinGeckoService.GetBatchHistoricalPrices(addresses, chainIds, symbols, networks, timestamps)
	require.NoError(t, err)
	assert.NotEmpty(t, prices)
	assert.NotNil(t, prices[0].Price)
}

func TestGetSinglePrice(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	price, err := coinGeckoService.GetSinglePrice("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "ETH", "ethereum", false)
	require.NoError(t, err)
	t.Logf("price: %v", price)
	assert.NotNil(t, price)
}

func TestGetSingleHistoricalPrice(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	timestamp := time.Now().Add(-24 * time.Hour).Unix()
	price, err := coinGeckoService.GetSingleHistoricalPrice(timestamp, "1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "ETH", "ethereum")
	require.NoError(t, err)
	assert.NotNil(t, price)
}

func TestGetBatchPriceIsNil(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	addresses := []string{"0x1"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}

	prices, _ := coinGeckoService.GetBatchPrice(addresses, chainIds, symbols, networks, false)
	fmt.Printf("TestGetBatchPrice prices: %v\n", prices)
	assert.Nil(t, prices[0].Price)
}

func TestGetBatchHistoricalPricesNil(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	addresses := []string{"0x1"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-24 * time.Hour).Unix()}

	prices, _ := coinGeckoService.GetBatchHistoricalPrices(addresses, chainIds, symbols, networks, timestamps)
	assert.Nil(t, prices[0].Price)
}

func TestGetSinglePriceNil(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	price, _ := coinGeckoService.GetSinglePrice("1", "0x1", "ETH", "ethereum", false)
	t.Logf("price: %v", price)
	assert.Nil(t, price)
}

func TestGetSingleHistoricalPriceNil(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	timestamp := time.Now().Add(-24 * time.Hour).Unix()
	price, _ := coinGeckoService.GetSingleHistoricalPrice(timestamp, "1", "0x1", "ETH", "ethereum")
	assert.Nil(t, price)
}

func TestSyncCoins(t *testing.T) {
	coinGeckoService := setupCoinGeckoService()

	err := coinGeckoService.SyncCoins()
	assert.NoError(t, err)
}
