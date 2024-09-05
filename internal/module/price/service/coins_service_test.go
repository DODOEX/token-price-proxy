package service_test

import (
	"testing"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/repository"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupCoinsService() service.CoinsService {
	db := shared.SetupRealDB()
	redis := shared.SetupRealRedis()
	coinRepo := repository.NewCoinRepository(db, zerolog.New(nil), redis)
	return service.NewCoinsService(coinRepo)
}

func TestAddCoin(t *testing.T) {
	coinsService := setupCoinsService()

	coin := schema.Coins{
		ID:      "1_0x1",
		Address: "0x1",
		ChainID: "1",
	}
	err := coinsService.AddCoin(coin)

	require.NoError(t, err)
}

func TestUpdateCoin(t *testing.T) {
	coinsService := setupCoinsService()

	updatedCoin := schema.Coins{
		Address: "0x12",
		ChainID: "1",
	}
	err := coinsService.UpdateCoin("1_0x1", updatedCoin)

	require.NoError(t, err)
}

func TestDeleteCoin(t *testing.T) {
	coinsService := setupCoinsService()

	err := coinsService.DeleteCoin("1_0x1")

	require.NoError(t, err)
}

func TestGetCoinByID(t *testing.T) {
	coinsService := setupCoinsService()

	result, err := coinsService.GetCoinByID("1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2")

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", result.ID)
}

func TestRefreshAllCoinsCache(t *testing.T) {
	coinsService := setupCoinsService()

	err := coinsService.RefreshAllCoinsCache()

	require.NoError(t, err)
}

func TestRefreshCoinListCache(t *testing.T) {
	coinsService := setupCoinsService()

	ids := []string{"1_0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"}
	err := coinsService.RefreshCoinListCache(ids)

	require.NoError(t, err)
}
