package middleware

import (
	"context"

	"github.com/DODOEX/token-price-proxy/internal/module/price/service"
	"github.com/DODOEX/token-price-proxy/internal/module/shared"
	"github.com/rs/zerolog"
	"github.com/valyala/fasthttp"
)

func RateLimitMiddleware(rateLimiterService *service.RateLimiterService, logger zerolog.Logger) func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
	return func(next fasthttp.RequestHandler) fasthttp.RequestHandler {
		return func(ctx *fasthttp.RequestCtx) {
			// 处理 CORS 预检请求
			if string(ctx.Method()) == fasthttp.MethodOptions {
				handleCors(ctx)
				ctx.SetStatusCode(fasthttp.StatusNoContent)
				return
			}
			// 尝试从请求头中获取 x_api_key
			apiKey := string(ctx.Request.Header.Peek("X-API-KEY"))

			// 如果请求头中没有 x_api_key，则尝试从请求参数中获取
			if apiKey == "" {
				apiKey = string(ctx.QueryArgs().Peek("x_api_key"))
			}

			if apiKey == "" && !shared.AllowApiKeyNil {
				ctx.SetStatusCode(fasthttp.StatusForbidden)
				ctx.SetBody([]byte("Forbidden"))
				return
			}

			allowed, err := rateLimiterService.Allow(context.Background(), apiKey)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to check rate limiter")
				ctx.SetStatusCode(fasthttp.StatusInternalServerError)
				ctx.SetBody([]byte("Api key invalid"))
				return
			}

			if !allowed {
				ctx.SetStatusCode(fasthttp.StatusTooManyRequests)
				ctx.SetBody([]byte("Too Many Requests"))
				return
			}

			// 调用下一个处理程序
			next(ctx)
		}
	}
}

// 处理 CORS 头信息
func handleCors(ctx *fasthttp.RequestCtx) {
	ctx.Response.Header.Set("Access-Control-Allow-Origin", "*")
	ctx.Response.Header.Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, UPDATE")
	ctx.Response.Header.Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, X-Extra-Header, Content-Type, Accept, Authorization")
	ctx.Response.Header.Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Cache-Control, Content-Language, Content-Type")
	ctx.Response.Header.Set("Access-Control-Allow-Credentials", "true")
	ctx.Response.Header.Set("Access-Control-Max-Age", "86400")
	ctx.SetContentType("application/json")
}
