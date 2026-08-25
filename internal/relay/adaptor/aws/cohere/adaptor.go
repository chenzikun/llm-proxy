package aws

import (
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/base"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/utils"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

var _ base.AwsProviderAdapter = new(CohereProviderAdaptor)

type CohereProviderAdaptor struct {
	AwsBedrockClient      *bedrockruntime.Client
	AwsBedrockAgentClient *bedrockagentruntime.Client
}

func NewCohereProviderAdaptor(channelConfig *model.ChannelConfig) (*CohereProviderAdaptor, error) {
	a := &CohereProviderAdaptor{}
	awsClient, err := base.GetOrCreateAwsClient(channelConfig)
	if err != nil {
		return nil, err
	}
	a.AwsBedrockClient = awsClient.AwsBedrockClient
	a.AwsBedrockAgentClient = awsClient.AwsBedrockAgentClient
	return a, nil
}

func (a *CohereProviderAdaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	c.Set(ctxkey.RequestModel, request.Model)
	return nil, nil
}

func (a *CohereProviderAdaptor) DoResponse(c *gin.Context, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	modelId := c.GetString(ctxkey.RequestModel)
	if strings.Contains(modelId, "rerank") {
		err, usage = RerankHandler(c, a.AwsBedrockAgentClient, meta.ActualModelName)
	} else if strings.Contains(modelId, "embed") {
		err, usage = EmbedHandler(c, a.AwsBedrockClient, meta.ActualModelName)
	} else {
		err = utils.WrapErr(errors.New("[cohere DoResponse] model not found"))
	}
	return nil, "", err
}
