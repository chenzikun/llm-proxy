package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/base"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

var _ base.AwsProviderAdapter = new(Llama3ProviderAdaptor)

type Llama3ProviderAdaptor struct {
	AwsBedrockClient      *bedrockruntime.Client
	AwsBedrockAgentClient *bedrockagentruntime.Client
}

func NewLlama3ProviderAdaptor(channelConfig *model.ChannelConfig) (*Llama3ProviderAdaptor, error) {
	a := &Llama3ProviderAdaptor{}
	awsClient, err := base.GetOrCreateAwsClient(channelConfig)
	if err != nil {
		return nil, err
	}
	a.AwsBedrockClient = awsClient.AwsBedrockClient
	a.AwsBedrockAgentClient = awsClient.AwsBedrockAgentClient
	return a, nil
}

func (a *Llama3ProviderAdaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	llamaReq := ConvertRequest(*request)
	c.Set(ctxkey.RequestModel, request.Model)
	c.Set(ctxkey.ConvertedRequest, llamaReq)
	return llamaReq, nil
}

func (a *Llama3ProviderAdaptor) DoResponse(c *gin.Context, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	if meta.IsStream {
		err, usage, responseText = StreamHandler(c, a.AwsBedrockClient)
	} else {
		err, usage, responseText = Handler(c, a.AwsBedrockClient, meta.ActualModelName)
	}
	return
}
