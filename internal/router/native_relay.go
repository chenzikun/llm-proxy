package router

import (
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/handler"
	"github.com/zicorn/llm-proxy/internal/middleware"
)

// SetNativeRelayRouter 注册原生格式 API 路由。
//
// 支持的路径前缀及对应协议：
//   - /anthropic/*  — Anthropic Messages API（claude-* 系列）
//   - /google/*     — Google Gemini API（gemini-* 系列）
//   - /vertexai/*   — Vertex AI API（GCP 上的 Claude / Gemini 等）
//
// 客户端只需将 SDK 的 base_url 改为：
//
//	https://<proxy>/anthropic
//	https://<proxy>/google
//	https://<proxy>/vertexai
//
// 其余（model、请求体、response）与原生 SDK 完全一致。
func SetNativeRelayRouter(router *gin.Engine) {
	middlewares := []gin.HandlerFunc{
		middleware.RelayPanicRecover(),
		middleware.TokenAuth(),
		middleware.Distribute(),
	}

	// Anthropic Messages API
	// 文档：https://docs.anthropic.com/en/api/messages
	anthropicRouter := router.Group("/anthropic")
	anthropicRouter.Use(middlewares...)
	{
		anthropicRouter.Any("/*path", controller.RelayNative)
	}

	// Google Gemini API
	// 文档：https://ai.google.dev/api/generate-content
	googleRouter := router.Group("/google")
	googleRouter.Use(middlewares...)
	{
		googleRouter.Any("/*path", controller.RelayNative)
	}

	// Vertex AI API
	// 文档：https://cloud.google.com/vertex-ai/docs/reference/rest
	vertexaiRouter := router.Group("/vertexai")
	vertexaiRouter.Use(middlewares...)
	{
		vertexaiRouter.Any("/*path", controller.RelayNative)
	}
}
