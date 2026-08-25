package router

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/zicorn/llm-proxy/pkg/common/logger"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/static"
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/config"
	"github.com/zicorn/llm-proxy/internal/handler"
	"github.com/zicorn/llm-proxy/internal/middleware"
)

func SetWebRouter(router *gin.Engine, buildFS embed.FS) {
	finetuneIndexPage, err := buildFS.ReadFile("doc/api/finetune/index.html")
	if err != nil {
		log.Fatal("doc files not found")
	}
	indexPageData, err := buildFS.ReadFile(fmt.Sprintf("build/%s/index.html", config.Theme))
	if err != nil {
		log.Fatal("web build files not found")
	}
	router.Use(gzip.Gzip(gzip.DefaultCompression))
	router.Use(middleware.GlobalWebRateLimit())
	router.Use(middleware.Cache())

	// Serve static files
	docFS := common.EmbedFolder(buildFS, "doc")
	docServer := static.Serve("/doc", docFS)
	staticServer := static.Serve("/", common.EmbedFolder(buildFS, fmt.Sprintf("build/%s", config.Theme)))
	router.Use(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") {
			logger.Infof(c.Request.Context(), "api request: %s", c.Request.RequestURI)
			c.Next()
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/doc") {
			logger.Infof(c.Request.Context(), "doc request: %s", c.Request.URL.Path)
			if c.Request.RequestURI == "/doc" {
				c.Redirect(http.StatusMovedPermanently, "/doc/")
				c.Abort()
				return
			}
			if c.Request.RequestURI == "/doc/api/finetune/" || c.Request.RequestURI == "/doc/api/finetune/index.html" {
				c.Data(http.StatusOK, "text/html; charset=utf-8", finetuneIndexPage)
				c.Abort()
				return
			}
			docServer(c)
			return
		}
		logger.Infof(c.Request.Context(), "static request: %s", c.Request.URL.Path)
		staticServer(c)
	})

	// NoRoute handler
	router.NoRoute(func(c *gin.Context) {
		logger.Infof(c.Request.Context(), "no route: %s", c.Request.RequestURI)
		if strings.HasPrefix(c.Request.RequestURI, "/v1") || strings.HasPrefix(c.Request.RequestURI, "/api") {
			controller.RelayNotFound(c)
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPageData)
	})
}
