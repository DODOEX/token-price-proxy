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

func TestGetCoinGeckoOnChainNetwork(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	network, err := coinGeckoOnChainService.GetCoinGeckoOnChainNetwork("ethereum", false)
	assert.NoError(t, err)
	assert.Equal(t, "eth", network)
	network, err = coinGeckoOnChainService.GetCoinGeckoOnChainNetwork("ethereum", false)
	assert.NoError(t, err)
	assert.Equal(t, "eth", network)
	//删除缓存再来一次 防止命中缓存
	redis.Client.Del(context.Background(), "coinGeckoOnChain:supported_networks")
	network, err = coinGeckoOnChainService.GetCoinGeckoOnChainNetwork("ethereum", false)
	assert.NoError(t, err)
	assert.Equal(t, "eth", network)
}

func TestGetCurrentPriceOnChain(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	price, err := coinGeckoOnChainService.GetCurrentPriceOnChain("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "ETH", false)
	assert.NoError(t, err)
	assert.NotNil(t, price)
}

func TestGetHistoricalPriceOnChain(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	price, err := coinGeckoOnChainService.GetHistoricalPriceOnChain("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", 1704959441)
	assert.NoError(t, err)
	assert.NotNil(t, price)
}

func TestGetBatchCurrentPricesOnChain(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	prices, err := coinGeckoOnChainService.GetBatchCurrentPricesOnChain(addresses, chainIds, symbols, networks, false)
	require.NoError(t, err)
	assert.NotEmpty(t, prices)
}

func TestGetBatchHistoricalPricesOnChain(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-24 * time.Hour).Unix()}
	prices, err := coinGeckoOnChainService.GetBatchHistoricalPricesOnChain(addresses, chainIds, symbols, networks, timestamps)
	require.NoError(t, err)
	assert.NotEmpty(t, prices)
}

func TestGetCurrentPriceOnChainIsNil(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	price, err := coinGeckoOnChainService.GetCurrentPriceOnChain("1", "0x1", "ETH", false)
	assert.NoError(t, err)
	assert.Nil(t, price)
}

func TestGetHistoricalPriceOnChainIsNil(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	timestamp := time.Now().Add(-24 * time.Hour).Unix()
	price, err := coinGeckoOnChainService.GetHistoricalPriceOnChain("1", "0x1", timestamp)
	assert.Error(t, err)
	assert.Nil(t, price)
}

func TestGetBatchCurrentPricesOnChainIsNil(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	addresses := []string{"0x1"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	prices, err := coinGeckoOnChainService.GetBatchCurrentPricesOnChain(addresses, chainIds, symbols, networks, false)
	require.NoError(t, err)
	assert.Nil(t, prices[0].Price)
}

func TestGetBatchHistoricalPricesOnChainIsNil(t *testing.T) {
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)

	addresses := []string{"0x2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-24 * time.Hour).Unix()}
	prices, err := coinGeckoOnChainService.GetBatchHistoricalPricesOnChain(addresses, chainIds, symbols, networks, timestamps)
	require.Error(t, err)

	// t.Logf("prices array: %v", prices)
	assert.Empty(t, prices)
}
