package controller

import (
	"encoding/json"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/valyala/fasthttp"
)

type CoinsController interface {
	AddCoin(ctx *fasthttp.RequestCtx)
	UpdateCoin(ctx *fasthttp.RequestCtx)
	DeleteCoin(ctx *fasthttp.RequestCtx)
	GetCoinByID(ctx *fasthttp.RequestCtx)
	DeleteRedisKey(ctx *fasthttp.RequestCtx)
	RefreshAllCoinsCache(ctx *fasthttp.RequestCtx)
	RefreshCoinListCache(ctx *fasthttp.RequestCtx)
}

type coinsController struct {
	coinsService service.CoinsService
	redisClient  *shared.RedisClient
}

func NewCoinsController(coinsService service.CoinsService, redisClient *shared.RedisClient) CoinsController {
	return &coinsController{
		coinsService: coinsService,
		redisClient:  redisClient,
	}
}

func (c *coinsController) respond(ctx *fasthttp.RequestCtx, code int, data interface{}, message string) {
	response := map[string]interface{}{
		"code":    code,
		"data":    data,
		"message": message,
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		ctx.Error("Failed to serialize response ", fasthttp.StatusInternalServerError)
		return
	}

	ctx.Response.Header.Set("Content-Type", "application/json; charset=utf-8")
	ctx.Response.SetBody(responseBody)
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
}

func (c *coinsController) AddCoin(ctx *fasthttp.RequestCtx) {
	var coin schema.Coins
	if err := json.Unmarshal(ctx.PostBody(), &coin); err != nil {
		c.respond(ctx, 400, nil, "Failed to parse request body")
		return
	}

	if err := c.coinsService.AddCoin(coin); err != nil {
		c.respond(ctx, 500, nil, "Failed to add token")
		return
	}

	c.respond(ctx, 200, nil, "Token added successfully")
}

func (c *coinsController) UpdateCoin(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	var coin schema.Coins
	if err := json.Unmarshal(ctx.PostBody(), &coin); err != nil {
		c.respond(ctx, 400, nil, "Failed to parse request body")
		return
	}

	if err := c.coinsService.UpdateCoin(id, coin); err != nil {
		c.respond(ctx, 500, nil, "Failed to update token")
		return
	}

	c.respond(ctx, 200, nil, "Failed to delete token")
}

func (c *coinsController) DeleteCoin(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	if err := c.coinsService.DeleteCoin(id); err != nil {
		c.respond(ctx, 500, nil, "Failed to delete token")
		return
	}

	c.respond(ctx, 200, nil, "Token deleted successfully")
}

func (c *coinsController) DeleteRedisKey(ctx *fasthttp.RequestCtx) {
	key := ctx.UserValue("key").(string)

	if err := c.redisClient.DeleteKeysByPrefix(key); err != nil {
		c.respond(ctx, 500, nil, "Failed to delete Redis key")
		return
	}

	c.respond(ctx, 200, nil, "Redis key deleted successfully")
}

func (c *coinsController) GetCoinByID(ctx *fasthttp.RequestCtx) {
	id := ctx.UserValue("id").(string)

	coin, err := c.coinsService.GetCoinByID(id)
	if err != nil {
		c.respond(ctx, 500, nil, "Failed to retrieve token")
		return
	}

	c.respond(ctx, 200, coin, "Token retrieved successfully")
}

func (c *coinsController) RefreshAllCoinsCache(ctx *fasthttp.RequestCtx) {

	if err := c.coinsService.RefreshAllCoinsCache(); err != nil {
		c.respond(ctx, 500, nil, "Failed to refresh cache")
		return
	}

	c.respond(ctx, 200, nil, "Cache refreshed successfully")
}

func (c *coinsController) RefreshCoinListCache(ctx *fasthttp.RequestCtx) {
	var requestData struct {
		Ids []string `json:"ids"`
	}
	if err := json.Unmarshal(ctx.PostBody(), &requestData); err != nil {
		c.respond(ctx, 500, nil, "Failed to refresh cache due to parameter parsing error.")
		return
	}
	ids := requestData.Ids
	if err := c.coinsService.RefreshCoinListCache(ids); err != nil {
		c.respond(ctx, 500, nil, "Failed to refresh cache")
		return
	}
	c.respond(ctx, 200, nil, "Cache refreshed successfully")
}
