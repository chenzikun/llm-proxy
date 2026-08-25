package aws

// todo 添加Endpoint模型
import (
	"errors"
	"io"
	"net/http"

	_ "github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/base"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/utils"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

var _ adaptor.RelayAdaptor = new(AwsRelayAdaptor)

type AwsRelayAdaptor struct {
	awsProviderAdapter base.AwsProviderAdapter
	Meta               *objects.Meta
}

func (a *AwsRelayAdaptor) Init(meta *objects.Meta) error {
	a.Meta = meta

	providerAdaptor, err := GetProviderAdaptor(meta.ActualModelName, &meta.Config)
	if err != nil {
		return err
	}
	if providerAdaptor == nil {
		return errors.New("adaptor not found, ActualModelName: " + meta.ActualModelName)
	}

	a.awsProviderAdapter = providerAdaptor
	return nil
}

func (a *AwsRelayAdaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}

	return a.awsProviderAdapter.ConvertRequest(c, relayMode, request)
}

func (a *AwsRelayAdaptor) DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error) {
	return nil, nil
}

func (a *AwsRelayAdaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode) {
	if a.awsProviderAdapter == nil {
		return nil, "", utils.WrapErr(errors.New("awsAdapter is nil"))
	}
	return a.awsProviderAdapter.DoResponse(c, meta)
}

func (a *AwsRelayAdaptor) GetModelList() (models []string) {
	for model_ := range adaptors {
		models = append(models, model_)
	}
	return
}

func (a *AwsRelayAdaptor) GetChannelName() string {
	return "aws"
}

func (a *AwsRelayAdaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	return "", nil
}

func (a *AwsRelayAdaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	return nil
}

func (a *AwsRelayAdaptor) ConvertImageRequest(request *entity.ImageRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}
