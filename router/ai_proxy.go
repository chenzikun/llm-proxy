package router

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/middleware"
)

// RouteConfig 定义路由配置
type RouteConfig struct {
	Prefix string
	Target string // 仅当该路由使用代理时需要
}

const maxUploadSize = 1 << 20 // 1MB

var routeConfigs = []RouteConfig{
	{Prefix: "/ok", Target: ""},
	{Prefix: "/voice-sync-data/", Target: "http://voice-sync-data-server-default:8000/"},
	{Prefix: "/llm-nlu/", Target: "http://llm-nlu-default:8000/"},
	{Prefix: "/llm-nlu-app/", Target: "http://llm-nlu-app-default:8000/"},
	{Prefix: "/llm-nlu-csms/", Target: "http://llm-nlu-csms-default:8000/"},
	{Prefix: "/llm-nlu-home/", Target: "http://llm-nlu-home-default:8000/"},
	{Prefix: "/test-tools/", Target: "http://test-tools-default:8000/"},
	{Prefix: "/ai-cron/", Target: "http://ai-cron-default:8000/user-ai-proxy/ai-cron"},
	{Prefix: "/ai-public-basic-services/", Target: "http://ai-public-basic-services-default:8000/"},
	{Prefix: "/ai-books-server/", Target: "http://ai-books-server-default:8000/"},
	{Prefix: "/logsearchtool/", Target: "http://logsearchtool-default:8000/"},
	{Prefix: "/pricing-analyst/", Target: "http://pricing-analyst-default:8000/"},
	{Prefix: "/pricing-analyst-worker/", Target: "http://pricing-analyst-worker-default:8000/"},
}

func baseProxy(aiProxyRouter *gin.RouterGroup) {
	aiProxyRouter.Any("/*proxyPath", func(c *gin.Context) {
		proxyPath := c.Param("proxyPath")
		proxyPathWithoutPrefix := strings.TrimPrefix(proxyPath, "/ai_proxy")

		for _, config := range routeConfigs {
			if strings.HasPrefix(proxyPathWithoutPrefix, config.Prefix) {
				if config.Target != "" {
					handleProxy(config.Target, proxyPathWithoutPrefix, config.Prefix, c)
				} else {
					handleInternal(proxyPathWithoutPrefix, c)
				}
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "No matching route"})
	})
}

func setTokenRouter(router *gin.Engine) {
	aiProxyRouter := router.Group("/ai-proxy")
	aiProxyRouter.Use(middleware.TokenAuth())
	baseProxy(aiProxyRouter)
}

func setUserRouter(router *gin.Engine) {
	aiProxyRouter := router.Group("/user-ai-proxy")
	aiProxyRouter.Use(middleware.UserAuth())
	baseProxy(aiProxyRouter)
}

func SetAIProxyRouter(router *gin.Engine) {
	setTokenRouter(router)
	setUserRouter(router)
}

func handleProxy(targetURL, proxyPath, prefix string, c *gin.Context) {
	// 解析目标服务的 URL
	fmt.Printf("proxyPath: %s\n", proxyPath)
	target, err := url.Parse(targetURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid target URL"})
		return
	}

	// 创建反向代理
	proxy := httputil.NewSingleHostReverseProxy(target)

	//// 如果是文件上传请求，确保不会缓存整个请求体
	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 200*maxUploadSize)
	}

	// 设置请求的 Host
	c.Request.Host = target.Host

	// 设置重写请求头部
	c.Request.Header.Set("Authorization", "")
	c.Request.Header.Set("X-Forwarded-Host", c.Request.Host)
	c.Request.Header.Set("X-Forwarded-For", c.ClientIP())

	// 重写请求路径
	c.Request.URL.Path = strings.TrimPrefix(proxyPath, prefix)

	// 转发请求
	proxy.ServeHTTP(c.Writer, c.Request)
}

// handleInternal 用于处理内部请求
func handleInternal(proxyPath string, c *gin.Context) {
	// 根据 proxyPath 进行不同的内部处理
	if proxyPath == "/ok" {
		c.JSON(http.StatusOK, gin.H{"message": "This is an internal response!"})
		return
	}
	// 添加更多内部处理逻辑
	c.JSON(http.StatusNotFound, gin.H{"error": "No matching internal route"})
}
