package base

import (
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

// AwsProviderAdapter 每个供应商需要实现的接口
type AwsProviderAdapter interface {
	ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error)
	DoResponse(c *gin.Context, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode)
}
