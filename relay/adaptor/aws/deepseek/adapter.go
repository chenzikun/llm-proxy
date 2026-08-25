package aws

import (
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor/aws/base"
	"github.com/songquanpeng/one-api/relay/entity"
)

var _ base.AwsProviderAdapter = new(DeepSeekProviderAdaptor)

type DeepSeekProviderAdaptor struct {
	AwsBedrockClient      *bedrockruntime.Client
	AwsBedrockAgentClient *bedrockagentruntime.Client
}

func NewDeepSeekProviderAdaptor(channelConfig *model.ChannelConfig) (*DeepSeekProviderAdaptor, error) {
	a := &DeepSeekProviderAdaptor{}
	awsClient, err := base.GetOrCreateAwsClient(channelConfig)
	if err != nil {
		return nil, err
	}
	a.AwsBedrockClient = awsClient.AwsBedrockClient
	a.AwsBedrockAgentClient = awsClient.AwsBedrockAgentClient
	return a, nil
}

func (a *DeepSeekProviderAdaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	c.Set(ctxkey.RequestModel, request.Model)
	request.Model = ""
	c.Set(ctxkey.ConvertedRequest, request)
	return request, nil
}

func (a *DeepSeekProviderAdaptor) DoResponse(c *gin.Context, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	if meta.IsStream {
		err, usage, responseText = StreamHandler(c, a.AwsBedrockClient, meta.ActualModelName)
	} else {
		err, usage, responseText = Handler(c, a.AwsBedrockClient, meta.ActualModelName)
	}
	return
}
