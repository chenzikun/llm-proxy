package router

import (
	"github.com/chenjiandongx/ginprom"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

func SetHealthRouter(router *gin.Engine) {
	healthRouter := router.Group("/actuator")
	healthRouter.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	healthRouter.GET("/stable", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	healthRouter.GET("/prometheus", ginprom.PromHandler(promhttp.Handler()))
}
