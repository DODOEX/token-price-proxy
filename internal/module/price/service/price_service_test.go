package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

// MockLifecycle 模拟的 fx.Lifecycle
type MockLifecycle struct {
	hooks []fx.Hook
}

func NewMockLifecycle() *MockLifecycle {
	return &MockLifecycle{}
}

func (m *MockLifecycle) Append(hook fx.Hook) {
	m.hooks = append(m.hooks, hook)
}

func (m *MockLifecycle) Start(ctx context.Context) error {
	for _, hook := range m.hooks {
		if hook.OnStart != nil {
			if err := hook.OnStart(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *MockLifecycle) Stop(ctx context.Context) error {
	for _, hook := range m.hooks {
		if hook.OnStop != nil {
			if err := hook.OnStop(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func setupPriceService() service.PriceService {
	// 设置生命周期
	lc := NewMockLifecycle() // 创建一个新的生命周期对象
	cfg := shared.SetupCfg()
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	slackRepo := repository.NewSlackNotificationRepository(lc, db, redis, zerolog.New(nil))
	historicalPriceRepo := repository.NewCoinHistoricalPriceRepository(db, zerolog.New(nil), redis, coinRepo)
	coinGeckoService := service.NewCoinGeckoService(cfg, coinRepo, historicalPriceRepo, redis, zerolog.New(nil))
	geckoTerminalService := service.NewGeckoTerminalService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo)
	defiLlamaService := service.NewDefiLlamaService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo)
	dodoexRouteService := service.NewDodoexRouteService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo)
	coinGeckoOnChainService := service.NewCoinGeckoOnChainService(cfg, redis, zerolog.New(nil), coinRepo, historicalPriceRepo, coinGeckoService)
	slackService := service.NewSlackNotificationService(slackRepo, redis, zerolog.New(nil))
	throttler := shared.NewCoinsThrottler(redis, zerolog.New(nil), coinRepo)
	return service.NewPriceService(
		slackService, coinGeckoService, geckoTerminalService, defiLlamaService,
		dodoexRouteService, coinGeckoOnChainService, coinRepo,
		zerolog.New(nil), throttler, redis,
	)
}

var priceService service.PriceService
var once sync.Once

func setupOnce() {
	once.Do(func() {
		priceService = setupPriceService()
	})
}

func TestGetPrice_Valid(t *testing.T) {
	setupOnce()
	price, _ := priceService.GetPrice("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "ETH", "ethereum", true, false)
	assert.NotNil(t, price)
	t.Logf("Valid Price: %v", *price)
}

func TestGetPrice_Invalid(t *testing.T) {
	setupOnce()
	price, _ := priceService.GetPrice("9999", "0xInvalidAddress", "ETH", "ethereum", true, false)
	assert.Nil(t, price)
}

func TestGetHistoricalPrice_Valid(t *testing.T) {
	setupOnce()
	timestamp := time.Now().Add(-24 * time.Hour).Unix()
	price, _ := priceService.GetHistoricalPrice("1", "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "ETH", "ethereum", timestamp)
	assert.NotNil(t, price)
	t.Logf("Valid Historical Price: %v", *price)
}

func TestGetHistoricalPrice_Invalid(t *testing.T) {
	priceService := setupPriceService()

	timestamp := time.Now().Add(-24 * time.Hour).Unix()
	price, _ := priceService.GetHistoricalPrice("9999", "0xInvalidAddress", "ETH", "ethereum", timestamp)
	assert.Nil(t, price)
}

func TestGetBatchPrice_Valid(t *testing.T) {
	setupOnce()
	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}

	prices, _ := priceService.GetBatchPrice(context.Background(), chainIds, addresses, symbols, networks, true, false)
	assert.NotNil(t, prices[0].Price)
	t.Logf("Batch Valid Prices: %v", prices)
}

func TestGetBatchPrice_Invalid(t *testing.T) {
	setupOnce()
	addresses := []string{"0xInvalidAddress"}
	chainIds := []string{"9999"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}

	prices, _ := priceService.GetBatchPrice(context.Background(), chainIds, addresses, symbols, networks, true, false)
	assert.Nil(t, prices[0].Price)
	t.Logf("Batch Invalid Prices: %v", prices)
}

func TestGetBatchHistoricalPrice_Valid(t *testing.T) {
	setupOnce()
	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-24 * time.Hour).Unix()}
	datesStr := []string{time.Now().Add(-24 * time.Hour).Format("02-01-2006")}

	prices, _ := priceService.GetBatchHistoricalPrice(chainIds, addresses, symbols, networks, timestamps, datesStr)
	assert.NotNil(t, prices[0].Price)
	t.Logf("Batch Valid Historical Prices: %v", prices)
}

func TestGetBatchHistoricalPrice_Invalid(t *testing.T) {
	setupOnce()
	addresses := []string{"0x1"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{time.Now().Add(-24 * time.Hour).Unix()}
	datesStr := []string{time.Now().Add(-24 * time.Hour).Format("02-01-2006")}

	prices, _ := priceService.GetBatchHistoricalPrice(chainIds, addresses, symbols, networks, timestamps, datesStr)
	assert.Nil(t, prices[0].Price)
	t.Logf("Batch Invalid Historical Prices: %v", prices)
}

func TestEdgeCases(t *testing.T) {
	setupOnce()
	// 测试空数组
	prices, _ := priceService.GetBatchPrice(context.Background(), []string{}, []string{}, []string{}, []string{}, true, false)
	assert.Empty(t, prices)
	t.Log("Empty array test passed.")

	// 测试边界时间戳
	addresses := []string{"0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	chainIds := []string{"1"}
	symbols := []string{"ETH"}
	networks := []string{"ethereum"}
	timestamps := []int64{0} // 边界时间戳
	datesStr := []string{"01-01-1970"}

	prices, _ = priceService.GetBatchHistoricalPrice(chainIds, addresses, symbols, networks, timestamps, datesStr)
	assert.Nil(t, prices[0].Price)
	t.Log("Boundary timestamp test passed.")
}
