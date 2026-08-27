package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/zicorn/llm-proxy/pkg/common/env"

	"github.com/google/uuid"
)

var SystemName = "One API"
var ServerAddress = env.String("SERVER_ADDRESS", "")
var Footer = ""
var Logo = ""
var TopUpLink = ""
var ChatLink = ""
var QuotaPerUnit = 1000 * 1000.0 // 1,000,000 额度单位 = ¥1（1元人民币）
var ExchangeRate = 7.2             // 汇率：1 USD = ? CNY，用于将 USD 计价的模型价格换算成 CNY
var DisplayInCurrencyEnabled = true
var DisplayTokenStatEnabled = true

// Any options with "Secret", "Token" in its key won't be return by GetOptions

var SessionSecret = uuid.New().String()

var OptionMap map[string]string
var OptionMapRWMutex sync.RWMutex

var ItemsPerPage = 10
var MaxRecentItems = 100

var PasswordLoginEnabled = true
var PasswordRegisterEnabled = true
var EmailVerificationEnabled = false
var GitHubOAuthEnabled = false
var OidcEnabled = false
var WeChatAuthEnabled = false
var TurnstileCheckEnabled = false
var RegisterEnabled = true

var EmailDomainRestrictionEnabled = false
var EmailDomainWhitelist = []string{
	"gmail.com",
	"163.com",
	"126.com",
	"qq.com",
	"outlook.com",
	"hotmail.com",
	"icloud.com",
	"yahoo.com",
	"foxmail.com",
}

var DebugEnabled = strings.ToLower(os.Getenv("DEBUG")) == "true"
var DebugSQLEnabled = strings.ToLower(os.Getenv("DEBUG_SQL")) == "true"

// MemoryCacheEnabled 是否启用内存缓存，启用缓存会导致用户额度的更新存在一定的延迟
var MemoryCacheEnabled = strings.ToLower(os.Getenv("MEMORY_CACHE_ENABLED")) == "true"

// SyncFrequency 在启用缓存的情况下与数据库同步配置的频率，单位为秒，默认为 600 秒
var SyncFrequency = env.Int("SYNC_FREQUENCY", 10*60) // unit is second

var LogConsumeEnabled = true

var SMTPServer = ""
var SMTPPort = 587
var SMTPAccount = ""
var SMTPFrom = ""
var SMTPToken = ""

var GitHubClientId = ""
var GitHubClientSecret = ""

var LarkClientId = ""
var LarkClientSecret = ""

var OidcClientId = ""
var OidcClientSecret = ""
var OidcWellKnown = ""
var OidcAuthorizationEndpoint = ""
var OidcTokenEndpoint = ""
var OidcUserinfoEndpoint = ""

var WeChatServerAddress = ""
var WeChatServerToken = ""
var WeChatAccountQRCodeImageURL = ""

var MessagePusherAddress = ""
var MessagePusherToken = ""

var TurnstileSiteKey = ""
var TurnstileSecretKey = ""

var QuotaForNewUser int64 = 2500000
var QuotaForInviter int64 = 0
var QuotaForInvitee int64 = 0
var ChannelDisableThreshold = 5.0
var AutomaticDisableChannelEnabled = false
var AutomaticEnableChannelEnabled = false
var QuotaRemindThreshold int64 = 1000
var PreConsumedQuota int64 = 500
var ApproximateTokenEnabled = false
var RetryTimes = 0

var RootUserEmail = ""

var IsMasterNode = os.Getenv("NODE_TYPE") != "slave"

var requestInterval, _ = strconv.Atoi(os.Getenv("POLLING_INTERVAL"))
var RequestInterval = time.Duration(requestInterval) * time.Second

// BatchUpdateEnabled 是否启用数据库批量更新聚合，启用会导致用户额度的更新存在一定的延迟
var BatchUpdateEnabled = os.Getenv("BATCH_UPDATE_ENABLED") == "true"

// BatchUpdateInterval 批量更新聚合的时间间隔，单位为秒
var BatchUpdateInterval = env.Int("BATCH_UPDATE_INTERVAL", 5)

var GeminiSafetySetting = env.String("GEMINI_SAFETY_SETTING", "BLOCK_NONE")

const Theme = "berry"

// All duration's unit is seconds
// Shouldn't larger then RateLimitKeyExpirationDuration
var (
	GlobalApiRateLimitNum            = env.Int("GLOBAL_API_RATE_LIMIT", 240)
	GlobalApiRateLimitDuration int64 = 3 * 60

	GlobalWebRateLimitNum            = env.Int("GLOBAL_WEB_RATE_LIMIT", 120)
	GlobalWebRateLimitDuration int64 = 3 * 60

	UploadRateLimitNum            = 10
	UploadRateLimitDuration int64 = 60

	DownloadRateLimitNum            = 10
	DownloadRateLimitDuration int64 = 60

	CriticalRateLimitNum            = 20
	CriticalRateLimitDuration int64 = 20 * 60
)

var RateLimitKeyExpirationDuration = 20 * time.Minute

// EnableMetric 是否根据请求成功率禁用渠道。开启后，如果某个渠道的成功率很低，则会禁用该渠道，默认不开启
var EnableMetric = env.Bool("ENABLE_METRIC", false)

// MetricQueueSize 请求成功率统计队列大小，默认为 10
var MetricQueueSize = env.Int("METRIC_QUEUE_SIZE", 10)

// MetricSuccessRateThreshold 请求成功率阈值，默认为 0.8，渠道成功率低于该阈值会被禁用
var MetricSuccessRateThreshold = env.Float64("METRIC_SUCCESS_RATE_THRESHOLD", 0.8)

var MetricSuccessChanSize = env.Int("METRIC_SUCCESS_CHAN_SIZE", 1024)

var MetricFailChanSize = env.Int("METRIC_FAIL_CHAN_SIZE", 128)

// InitialRootToken 如果设置了该值，则在系统首次启动时会自动创建一个值为该环境变量值的 root 用户令牌
var InitialRootToken = os.Getenv("INITIAL_ROOT_TOKEN")

// InitialRootAccessToken 如果设置了该值，则在系统首次启动时会自动创建一个值为该环境变量的 root 用户创建系统管理令牌
var InitialRootAccessToken = os.Getenv("INITIAL_ROOT_ACCESS_TOKEN")

// GeminiVersion One API 所使用的 Gemini 版本，默认为 v1
var GeminiVersion = env.String("GEMINI_VERSION", "v1")

var OnlyOneLogFile = env.Bool("ONLY_ONE_LOG_FILE", false)

// RelayProxy 设置后使用该代理来请求 API
var RelayProxy = env.String("RELAY_PROXY", "")

// RelayTimeout 中继超时设置，单位为秒，默认不设置超时时间
var RelayTimeout = env.Int("RELAY_TIMEOUT", 0) // unit is second

// UserContentRequestProxy 设置后使用该代理来请求用户上传的内容，例如图片
var UserContentRequestProxy = env.String("USER_CONTENT_REQUEST_PROXY", "")

// UserContentRequestTimeout 用户上传内容下载超时时间，单位为秒。
var UserContentRequestTimeout = env.Int("USER_CONTENT_REQUEST_TIMEOUT", 30)

// ChannelTestFrequency 设置之后将定期检查渠道，单位为分钟，未设置则不进行检查
var ChannelTestFrequency = env.Int("CHANNEL_TEST_FREQUENCY", 0)

// RedisMode Redis 模式，支持 single、cluster，默认为 single
var RedisMode = env.String("REDIS_MODE", "cluster")
var RedisServer = os.Getenv("REDIS_SERVER")
var RedisPassword = os.Getenv("REDIS_PASSWORD")

var MQServerAddr = env.String("RABBITMQ310_SERVER", "10.240.3.153:5672")
var MQVhost = env.String("RABBITMQ_VHOST", "test")
var MQUsername = env.String("RABBITMQ_USERNAME", "admin")
var MQPassword = env.String("RABBITMQ_PASSWORD", "admin")

// var MQUrl = fmt.Sprintf("amqp://%s:%s@%s/%s", MQUsername, MQPassword, MQServerAddr, MQVhost)

func GetMQUrl() string {
	if MQServerAddr == "" {
		return ""
	}
	return fmt.Sprintf("amqp://%s:%s@%s/%s", MQUsername, MQPassword, MQServerAddr, MQVhost)
}

// var MQQueueName = env.String("RABBITMQ_QUEUE_NAME", "LLM_AI_SYNC_DATA")
const MQQueueName = "LLM_PROXY_QUEUE"
