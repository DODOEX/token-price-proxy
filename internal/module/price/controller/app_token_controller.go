package controller

import (
	"encoding/json"

	"github.com/DODOEX/token-price-proxy/internal/database/schema"
	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/valyala/fasthttp"
)

type AppTokenController interface {
	GetAppToken(ctx *fasthttp.RequestCtx)
	GetAllAppTokens(ctx *fasthttp.RequestCtx)
	AddAppToken(ctx *fasthttp.RequestCtx)
	UpdateAppToken(ctx *fasthttp.RequestCtx)
	DeleteAppToken(ctx *fasthttp.RequestCtx)
	CheckhHealthz(ctx *fasthttp.RequestCtx)
}

type appTokenController struct {
	appTokenService service.AppTokenService
}

func NewAppTokenController(appTokenService service.AppTokenService) AppTokenController {
	return &appTokenController{
		appTokenService: appTokenService,
	}
}

func (c *appTokenController) respond(ctx *fasthttp.RequestCtx, code int, data interface{}, message string) {
	response := map[string]interface{}{
		"code":    code,
		"data":    data,
		"message": message,
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		ctx.Error("Failed to serialize the response ", fasthttp.StatusInternalServerError)
		return
	}
	ctx.Response.Header.Set("Content-Type", "application/json; charset=utf-8")
	ctx.Response.SetBody(responseBody)
	ctx.Response.SetStatusCode(fasthttp.StatusOK)
}

func (c *appTokenController) GetAppToken(ctx *fasthttp.RequestCtx) {
	token := ctx.UserValue("token").(string)
	appToken, err := c.appTokenService.GetAppTokenByToken(token)
	if err != nil {
		c.respond(ctx, 500, nil, "Failed to retrieve AppToken")
		return
	}
	c.respond(ctx, 0, appToken, "Request successful")
}

func (c *appTokenController) GetAllAppTokens(ctx *fasthttp.RequestCtx) {
	appTokens, err := c.appTokenService.GetAllAppTokens()
	if err != nil {
		c.respond(ctx, 500, nil, "Failed to retrieve all AppTokens.")
		return
	}
	c.respond(ctx, 0, appTokens, "Request successful")
}

func (c *appTokenController) AddAppToken(ctx *fasthttp.RequestCtx) {
	var appToken schema.AppToken
	if err := json.Unmarshal(ctx.PostBody(), &appToken); err != nil {
		c.respond(ctx, 500, nil, "Failed to parse the request body")
		return
	}

	if err := c.appTokenService.AddAppToken(&appToken); err != nil {
		c.respond(ctx, 500, nil, "Failed to add AppToken")
		return
	}
	c.respond(ctx, 0, nil, "Successfully added AppToken")
}

func (c *appTokenController) UpdateAppToken(ctx *fasthttp.RequestCtx) {
	var appToken schema.AppToken
	if err := json.Unmarshal(ctx.PostBody(), &appToken); err != nil {
		c.respond(ctx, 500, nil, "Failed to parse the request body")
		return
	}

	if err := c.appTokenService.UpdateAppToken(&appToken); err != nil {
		c.respond(ctx, 500, nil, "Failed to update AppToken")
		return
	}
	c.respond(ctx, 0, nil, "Successfully updated AppToken")
}

func (c *appTokenController) DeleteAppToken(ctx *fasthttp.RequestCtx) {
	token := ctx.UserValue("token").(string)
	if err := c.appTokenService.DeleteAppToken(token); err != nil {
		c.respond(ctx, 500, nil, "Failed to delete AppToken")
		return
	}
	c.respond(ctx, 0, nil, "Successfully deleted AppToken")
}

func (c *appTokenController) CheckhHealthz(ctx *fasthttp.RequestCtx) {
	c.respond(ctx, 0, nil, "Successfully checked service status")
}
