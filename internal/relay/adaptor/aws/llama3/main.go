// Package aws provides the AWS adaptor for the relay service.
package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/helper"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/pkg/common/random"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/utils"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
	relaymodel "github.com/zicorn/llm-proxy/internal/relay/entity"
)

// Only support llama-3-8b and llama-3-70b instruction models
// https://docs.aws.amazon.com/bedrock/latest/userguide/model-ids.html
var Llama3ModelIDMap = map[string]string{
	"aws-llama3:8b":     "meta.llama3-8b-instruct-v1:0",
	"aws-llama3:70b":    "meta.llama3-70b-instruct-v1:0",
	"aws-llama3.1:8b":   "meta.llama3-1-8b-instruct-v1:0",
	"aws-llama3.1:70b":  "meta.llama3-1-70b-instruct-v1:0",
	"aws-llama3.1:405b": "meta.llama3-1-405b-instruct-v1:0",
	"aws-llama3.2:1b":   "us.meta.llama3-2-1b-instruct-v1:0",
	"aws-llama3.2:3b":   "us.meta.llama3-2-3b-instruct-v1:0",
	"aws-llama3.2:11b":  "us.meta.llama3-2-11b-instruct-v1:0",
	"aws-llama3.2:90b":  "us.meta.llama3-2-90b-instruct-v1:0",

	"meta.llama3-8b-instruct-v1:0":  "meta.llama3-8b-instruct-v1:0",
	"meta.llama3-70b-instruct-v1:0": "meta.llama3-70b-instruct-v1:0",

	"meta.llama3-1-8b-instruct-v1:0":   "meta.llama3-1-8b-instruct-v1:0",
	"meta.llama3-1-70b-instruct-v1:0":  "meta.llama3-1-70b-instruct-v1:0",
	"meta.llama3-1-405b-instruct-v1:0": "meta.llama3-1-405b-instruct-v1:0",

	"meta.llama3-2-1b-instruct-v1:0":  "us.meta.llama3-2-1b-instruct-v1:0",
	"meta.llama3-2-3b-instruct-v1:0":  "us.meta.llama3-2-3b-instruct-v1:0",
	"meta.llama3-2-11b-instruct-v1:0": "us.meta.llama3-2-11b-instruct-v1:0",
	"meta.llama3-2-90b-instruct-v1:0": "us.meta.llama3-2-90b-instruct-v1:0",
}

func awsModelID(requestModel string) (string, error) {
	if awsModelID, ok := Llama3ModelIDMap[requestModel]; ok {
		return awsModelID, nil
	}
	// 对于未在静态映射表中注册的模型，若请求名本身符合 Bedrock Llama 模型 ID 格式，则直接透传。
	if strings.HasPrefix(requestModel, "meta.llama3-") ||
		strings.HasPrefix(requestModel, "us.meta.llama3-") ||
		strings.HasPrefix(requestModel, "eu.meta.llama3-") ||
		strings.HasPrefix(requestModel, "ap.meta.llama3-") {
		return requestModel, nil
	}
	return "", errors.Errorf("model %s not found", requestModel)
}

// promptTemplate with range
const promptTemplate = `<|begin_of_text|>{{range .Messages}}<|start_header_id|>{{.Role}}<|end_header_id|>{{.StringContent}}<|eot_id|>{{end}}<|start_header_id|>assistant<|end_header_id|>
`

var promptTpl = template.Must(template.New("llama3-chat").Parse(promptTemplate))

func RenderPrompt(messages []relaymodel.Message) string {
	var buf bytes.Buffer
	err := promptTpl.Execute(&buf, struct{ Messages []relaymodel.Message }{messages})
	if err != nil {
		logger.SysError("error rendering prompt messages: " + err.Error())
	}
	return buf.String()
}

func ConvertRequest(textRequest relaymodel.GeneralOpenAIRequest) *Request {
	llamaRequest := Request{
		MaxGenLen:   textRequest.MaxTokens,
		Temperature: textRequest.Temperature,
		TopP:        textRequest.TopP,
	}
	if llamaRequest.MaxGenLen == 0 {
		llamaRequest.MaxGenLen = 2048
	}
	prompt := RenderPrompt(textRequest.Messages)
	llamaRequest.Prompt = prompt
	return &llamaRequest
}

func Handler(c *gin.Context, awsCli *bedrockruntime.Client, modelName string) (*objects.ErrorWithStatusCode, *relaymodel.Usage, string) {
	awsModelId, err := awsModelID(c.GetString(ctxkey.RequestModel))
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "awsModelID")), nil, ""
	}

	awsReq := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(awsModelId),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	llamaReq, ok := c.Get(ctxkey.ConvertedRequest)
	if !ok {
		return utils.WrapErr(errors.New("request not found")), nil, ""
	}

	awsReq.Body, err = json.Marshal(llamaReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "marshal request")), nil, ""
	}

	awsResp, err := awsCli.InvokeModel(c.Request.Context(), awsReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "InvokeModel")), nil, ""
	}

	var llamaResponse Response
	err = json.Unmarshal(awsResp.Body, &llamaResponse)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "unmarshal response")), nil, ""
	}

	openaiResp := ResponseLlama2OpenAI(&llamaResponse)
	openaiResp.Model = modelName
	usage := relaymodel.Usage{
		PromptTokens:     llamaResponse.PromptTokenCount,
		CompletionTokens: llamaResponse.GenerationTokenCount,
		TotalTokens:      llamaResponse.PromptTokenCount + llamaResponse.GenerationTokenCount,
	}
	openaiResp.Usage = usage

	c.JSON(http.StatusOK, openaiResp)
	responseText := openaiResp.Choices[0].Message.StringContent()
	return nil, &usage, responseText
}

func ResponseLlama2OpenAI(llamaResponse *Response) *openai.TextResponse {
	var responseText string
	if len(llamaResponse.Generation) > 0 {
		responseText = llamaResponse.Generation
	}
	choice := openai.TextResponseChoice{
		Index: 0,
		Message: relaymodel.Message{
			Role:    "assistant",
			Content: responseText,
			Name:    nil,
		},
		FinishReason: llamaResponse.StopReason,
	}
	fullTextResponse := openai.TextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", random.GetUUID()),
		Object:  "chat.completion",
		Created: helper.GetTimestamp(),
		Choices: []openai.TextResponseChoice{choice},
	}
	return &fullTextResponse
}

func StreamHandler(c *gin.Context, awsCli *bedrockruntime.Client) (*objects.ErrorWithStatusCode, *relaymodel.Usage, string) {
	responseText := ""
	createdTime := helper.GetTimestamp()
	awsModelId, err := awsModelID(c.GetString(ctxkey.RequestModel))
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "awsModelID")), nil, ""
	}

	awsReq := &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(awsModelId),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	llamaReq, ok := c.Get(ctxkey.ConvertedRequest)
	if !ok {
		return utils.WrapErr(errors.New("request not found")), nil, ""
	}

	awsReq.Body, err = json.Marshal(llamaReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "marshal request")), nil, ""
	}

	awsResp, err := awsCli.InvokeModelWithResponseStream(c.Request.Context(), awsReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "InvokeModelWithResponseStream")), nil, ""
	}
	stream := awsResp.GetStream()
	defer stream.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	var usage relaymodel.Usage
	c.Stream(func(w io.Writer) bool {
		event, ok := <-stream.Events()
		if !ok {
			c.Render(-1, common.CustomEvent{Data: "data: [DONE]"})
			return false
		}

		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			var llamaResp StreamResponse
			err := json.NewDecoder(bytes.NewReader(v.Value.Bytes)).Decode(&llamaResp)
			if err != nil {
				logger.SysError("error unmarshalling stream response: " + err.Error())
				return false
			}

			if llamaResp.PromptTokenCount > 0 {
				usage.PromptTokens = llamaResp.PromptTokenCount
			}
			if llamaResp.StopReason == "stop" {
				usage.CompletionTokens = llamaResp.GenerationTokenCount
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
			response := StreamResponseLlama2OpenAI(&llamaResp)
			response.Id = fmt.Sprintf("chatcmpl-%s", random.GetUUID())
			response.Model = c.GetString(ctxkey.OriginalModel)
			response.Created = createdTime
			jsonStr, err := json.Marshal(response)
			if err != nil {
				logger.SysError("error marshalling stream response: " + err.Error())
				return true
			}
			responseText += response.Choices[0].Delta.StringContent()
			c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonStr)})
			return true
		case *types.UnknownUnionMember:
			fmt.Println("unknown tag:", v.Tag)
			return false
		default:
			fmt.Println("union is nil or unknown type")
			return false
		}
	})

	return nil, &usage, responseText
}

func StreamResponseLlama2OpenAI(llamaResponse *StreamResponse) *openai.ChatCompletionsStreamResponse {
	var choice openai.ChatCompletionsStreamResponseChoice
	choice.Delta.Content = llamaResponse.Generation
	choice.Delta.Role = "assistant"
	finishReason := llamaResponse.StopReason
	if finishReason != "null" {
		choice.FinishReason = &finishReason
	}
	var openaiResponse openai.ChatCompletionsStreamResponse
	openaiResponse.Object = "chat.completion.chunk"
	openaiResponse.Choices = []openai.ChatCompletionsStreamResponseChoice{choice}
	return &openaiResponse
}
