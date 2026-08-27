package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/chenjiandongx/ginprom"
	"github.com/gin-gonic/gin"
	gorillasessions "github.com/gorilla/sessions"
	_ "github.com/joho/godotenv/autoload"
	"github.com/rbcervilla/redisstore/v9"
	"github.com/redis/go-redis/v9"
	"github.com/zicorn/llm-proxy/internal/webstatic"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/client"
	"github.com/zicorn/llm-proxy/pkg/common/config"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/handler"
	"github.com/zicorn/llm-proxy/internal/middleware"
	"github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/router"
)

func main() {
	common.Init()
	logger.SetupLogger()
	logger.SysLogf("One API %s started", common.Version)

	if os.Getenv("GIN_MODE") != gin.DebugMode {
		gin.SetMode(gin.ReleaseMode)
	}
	if config.DebugEnabled {
		logger.SysLog("running in debug mode")
	}

	// Initialize SQL Database
	// model.InitDB()
	// model.InitLogDB()

	// 初始化模型元数据，临时措施
	model.InitModelMetaFromMap()

	var err error
	err = model.CreateRootAccountIfNeed()
	if err != nil {
		logger.FatalLog("database init error: " + err.Error())
	}
	defer func() {
		err := model.CloseDB()
		if err != nil {
			logger.FatalLog("failed to close database: " + err.Error())
		}
	}()

	// Initialize Redis
	err = common.InitRedisClient()
	if err != nil {
		logger.FatalLog("failed to initialize Redis: " + err.Error())
	}

	// Initialize options
	model.InitOptionMap()
	logger.SysLog(fmt.Sprintf("using theme %s", config.Theme))
	if common.RedisEnabled {
		// for compatibility with old versions
		config.MemoryCacheEnabled = true
	}
	if config.MemoryCacheEnabled {
		logger.SysLog("memory cache enabled")
		logger.SysLog(fmt.Sprintf("sync frequency: %d seconds", config.SyncFrequency))
		model.InitChannelCache()
	}
	if config.MemoryCacheEnabled {
		go model.SyncOptions(config.SyncFrequency)
		go model.SyncChannelCache(config.SyncFrequency)
	}
	if config.ChannelTestFrequency > 0 {
		go controller.AutomaticallyTestChannels(config.ChannelTestFrequency)
	}
	if config.BatchUpdateEnabled {
		logger.SysLog("batch update enabled with interval " + strconv.Itoa(config.BatchUpdateInterval) + "s")
		model.InitBatchUpdater()
	}
	if config.EnableMetric {
		logger.SysLog("metric enabled, will disable channel if too much request failed")
	}
	objects.InitTokenEncoders()
	client.Init()

	// Initialize HTTP server
	server := gin.New()
	server.Use(gin.Recovery())
	// This will cause SSE not to work!!!
	//server.Use(gzip.Gzip(gzip.DefaultCompression))
	// yyf add for request & response logs
	//server.Use(RequestLogger())
	// yyf add end
	// use prometheus
	server.Use(ginprom.PromMiddleware(nil))

	server.Use(middleware.RequestId())
	middleware.SetUpLogger(server)
	// Initialize session store
	//store := cookie.NewStore([]byte(config.SessionSecret))
	// Initialize a Redis cluster client
	//store, err := redis.NewStore(10, "tcp", config.RedisServer, config.RedisPassword, []byte(config.SessionSecret))
	//if err != nil {
	//	logger.SysLog(fmt.Sprintf("redisServer=%s, redisPassword=%s", config.RedisServer, config.RedisPassword))
	//	logger.FatalLog("failed to initialize Redis session store: " + err.Error())
	//}
	var sessionStore gorillasessions.Store
	if config.RedisServer == "" {
		logger.SysLog("REDIS_SERVER not set, using cookie-based session store (single-node mode)")
		sessionStore = gorillasessions.NewCookieStore([]byte(config.SessionSecret))
	} else {
		ctx := context.Background()
		var redisStore *redisstore.RedisStore
		if config.RedisMode == "cluster" {
			redisStore, err = redisstore.NewRedisStore(ctx, redis.NewClusterClient(&redis.ClusterOptions{
				Addrs:    []string{config.RedisServer},
				Password: config.RedisPassword,
			}))
		} else if config.RedisMode == "single" {
			redisStore, err = redisstore.NewRedisStore(ctx, redis.NewClient(&redis.Options{
				Addr:     config.RedisServer,
				Password: config.RedisPassword,
			}))
		} else {
			logger.FatalLog("invalid Redis mode: " + config.RedisMode)
		}
		if err != nil {
			log.Fatal("failed to create redis store: ", err)
		}
		sessionStore = redisStore
	}
	server.Use(common.SessionMiddleware("session", sessionStore))

	router.SetRouter(server, webstatic.BuildFS)
	var port = os.Getenv("PORT")
	if port == "" {
		port = strconv.Itoa(*common.Port)
	}
	logger.SysLogf("server started on http://localhost:%s", port)
	err = server.Run(":" + port)
	if err != nil {
		logger.FatalLog("failed to start HTTP server: " + err.Error())
	}
}
