package aws

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/utils"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

// CohereModelIDMap 将代理模型名映射到 Bedrock 模型 ID（目前均为恒等映射）。
// 对于未在此映射表中的模型，getRerankOrEmbedModelID 会通过前缀检测自动透传 Bedrock 原生模型 ID，
// 无需修改代码即可使用新发布的 Cohere/Amazon 向量/重排模型。
var CohereModelIDMap = map[string]string{
	// TODO: amazon 模型应该分离出去
	"amazon.rerank-v1:0":           "amazon.rerank-v1:0",
	"cohere.rerank-v3-5:0":         "cohere.rerank-v3-5:0",
	"cohere.embed-english-v3":      "cohere.embed-english-v3",
	"cohere.embed-multilingual-v3": "cohere.embed-multilingual-v3",
	"amazon.titan-embed-text-v1":   "amazon.titan-embed-text-v1",
	"amazon.titan-embed-text-v2:0": "amazon.titan-embed-text-v2:0",
	//"amazon.titan-embed-image-v1":  "amazon.titan-embed-image-v1",
}

// getRerankOrEmbedModelID 将模型名解析为 Bedrock 模型 ID。
// 优先查静态映射表，未命中时对符合 Bedrock 格式前缀的模型 ID 直接透传。
func getRerankOrEmbedModelID(modelName string) (string, bool) {
	if id, ok := CohereModelIDMap[modelName]; ok {
		return id, true
	}
	if strings.HasPrefix(modelName, "cohere.") ||
		strings.HasPrefix(modelName, "amazon.rerank-") ||
		strings.HasPrefix(modelName, "amazon.titan-embed-") {
		return modelName, true
	}
	return "", false
}

func RerankHandler(c *gin.Context, awsClient *bedrockagentruntime.Client, modelName string) (*objects.ErrorWithStatusCode, *entity.Usage) {
	modeId, ok := getRerankOrEmbedModelID(modelName)
	if !ok {
		return utils.WrapErr(errors.New("[cohere RerankHandler] model not found")), nil
	}

	param := &CohereRerankRequest{}
	err := common.UnmarshalBodyReusable(c, param)
	if err != nil {
		return utils.WrapErr(errors.New("invalid request params")), nil
	}

	r, err := utils.RerankText(awsClient, param.Query, param.Documents, int32(param.TopN), modeId)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "RerankText")), nil
	}
	var rs []utils.RerankResultItem
	for _, item := range r.Results {
		rs = append(rs, utils.RerankResultItem{
			Index:          *item.Index,
			RelevanceScore: *item.RelevanceScore,
		})
	}
	result := utils.RerankResult{
		Results: rs,
	}
	c.JSON(http.StatusOK, result)
	return nil, nil
}

func EmbedHandler(c *gin.Context, awsCli *bedrockruntime.Client, modelName string) (*objects.ErrorWithStatusCode, *entity.Usage) {
	logger.Infof(c.Request.Context(), "EmbedHandler: modelName=%s", modelName)
	awsModelID, ok := getRerankOrEmbedModelID(modelName)
	if !ok {
		return utils.WrapErr(errors.New("model not found")), nil
	}

	awsReq := &bedrockruntime.InvokeModelInput{
		ModelId:     &awsModelID,
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	var p openai.EmbeddingRequest
	err := c.ShouldBindJSON(&p)
	if err != nil {
		return utils.WrapErr(errors.New("invalid request params")), nil
	}
	var texts []string
	switch tp := p.Input.(type) {
	case string:
		texts = []string{tp}
	case []string:
		texts = tp
	case []interface{}:
		for _, item := range tp {
			if s, ok := item.(string); ok {
				texts = append(texts, s)
			}
		}
	default:
		logger.Errorf(c.Request.Context(), "invalid input type: %T", tp)
		return utils.WrapErr(errors.New("invalid input type")), nil
	}

	if strings.HasPrefix(awsModelID, "amazon.titan-embed") {
		resp := openai.EmbeddingResponse{
			Object: "list",
			Model:  p.Model,
			Data:   []openai.EmbeddingResponseItem{},
		}
		for i, text := range texts {
			body, err := json.Marshal(TitanEmbedRequest{InputText: text})
			if err != nil {
				return utils.WrapErr(errors.Wrap(err, "marshal titan request")), nil
			}
			logger.Infof(c.Request.Context(), "Embedding Params: %s", string(body))
			titanResp, err := awsCli.InvokeModel(c.Request.Context(), &bedrockruntime.InvokeModelInput{
				ModelId:     &awsModelID,
				Accept:      aws.String("application/json"),
				ContentType: aws.String("application/json"),
				Body:        body,
			})
			if err != nil {
				return utils.WrapErr(errors.Wrap(err, "InvokeModel")), nil
			}
			var tResp TitanEmbedResponse
			if err := json.Unmarshal(titanResp.Body, &tResp); err != nil {
				return utils.WrapErr(errors.Wrap(err, "unmarshal titan response")), nil
			}
			embedding := make([]interface{}, len(tResp.Embedding))
			for j, v := range tResp.Embedding {
				embedding[j] = v
			}
			resp.Data = append(resp.Data, openai.EmbeddingResponseItem{
				Object:    "embedding",
				Index:     i,
				Embedding: embedding,
			})
		}
		c.JSON(http.StatusOK, resp)
		return nil, nil
	}

	var param = CohereEmbedRequest{
		Texts:          texts,
		EmbeddingTypes: []string{p.EncodingFormat},
		InputType:      p.InputType,
	}

	awsReq.Body, err = json.Marshal(param)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "marshal request")), nil
	}

	logger.Infof(c.Request.Context(), "Embedding Params: %s", string(awsReq.Body))

	awsResp, err := awsCli.InvokeModel(c.Request.Context(), awsReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "InvokeModel")), nil
	}

	var chResp CohereEmbedResponse
	err = json.Unmarshal(awsResp.Body, &chResp)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "unmarshal response")), nil
	}

	embeddings := chResp.Embeddings[p.EncodingFormat]

	var resp = openai.EmbeddingResponse{
		Object: "list",
		Model:  p.Model,
		Data:   []openai.EmbeddingResponseItem{},
	}
	for i, embedding := range embeddings {
		item := openai.EmbeddingResponseItem{
			Object:    "embedding",
			Index:     i,
			Embedding: embedding,
		}
		resp.Data = append(resp.Data, item)
	}
	c.JSON(http.StatusOK, resp)
	return nil, nil
}
