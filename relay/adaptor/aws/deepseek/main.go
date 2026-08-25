package aws

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor/aws/utils"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/entity"
)

var DeepSeekModelIDMap = map[string]string{
	"deepseek.r1-v1:0": "us.deepseek.r1-v1:0",
}

func Handler(c *gin.Context, awsCli *bedrockruntime.Client, modelName string) (*objects.ErrorWithStatusCode, *entity.Usage, string) {
	modelId, ok := DeepSeekModelIDMap[modelName]
	if !ok {
		return utils.WrapErr(errors.New("[deepseek Handler] model not found")), nil, ""
	}
	awsReq := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(modelId),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	request, ok := c.Get(ctxkey.ConvertedRequest)
	if !ok {
		return utils.WrapErr(errors.New("request not found")), nil, ""
	}

	var err error
	awsReq.Body, err = json.Marshal(request)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "marshal request")), nil, ""
	}

	logger.Infof(c.Request.Context(), "Handler：aws model_id: %s", *awsReq.ModelId)
	logger.Infof(c.Request.Context(), "Handler：aws request: %s", string(awsReq.Body))
	awsResp, err := awsCli.InvokeModel(c.Request.Context(), awsReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "InvokeModel")), nil, ""
	}
	var response openai.TextResponse
	err = json.Unmarshal(awsResp.Body, &response)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "unmarshal response")), nil, ""
	}

	response.Model = modelName

	c.JSON(http.StatusOK, response)
	responseText := response.Choices[0].Message.StringContent()
	return nil, &response.Usage, responseText
}

func StreamHandler(c *gin.Context, awsCli *bedrockruntime.Client, modelName string) (*objects.ErrorWithStatusCode, *entity.Usage, string) {
	responseText := ""
	// createdTime := helper.GetTimestamp()
	modelId, ok := DeepSeekModelIDMap[modelName]
	if !ok {
		return utils.WrapErr(errors.New("[deepseek Handler] model not found")), nil, ""
	}

	awsReq := &bedrockruntime.InvokeModelWithResponseStreamInput{
		ModelId:     aws.String(modelId),
		Accept:      aws.String("application/json"),
		ContentType: aws.String("application/json"),
	}

	request, ok := c.Get(ctxkey.ConvertedRequest)
	if !ok {
		return utils.WrapErr(errors.New("request not found")), nil, ""
	}
	var err error
	awsReq.Body, err = json.Marshal(request)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "marshal request")), nil, ""
	}

	logger.Infof(c.Request.Context(), "StreamHandler：aws model_id: %s", *awsReq.ModelId)
	logger.Infof(c.Request.Context(), "StreamHandler：aws request: %s", string(awsReq.Body))
	awsResp, err := awsCli.InvokeModelWithResponseStream(c.Request.Context(), awsReq)
	if err != nil {
		return utils.WrapErr(errors.Wrap(err, "InvokeModelWithResponseStream")), nil, ""
	}
	stream := awsResp.GetStream()
	defer stream.Close()

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	var usage entity.Usage
	var id string
	// var lastToolCallChoice openai.ChatCompletionsStreamResponseChoice

	c.Stream(func(w io.Writer) bool {
		event, ok := <-stream.Events()
		if !ok {
			c.Render(-1, common.CustomEvent{Data: "data: [DONE]"})
			return false
		}

		switch v := event.(type) {
		case *types.ResponseStreamMemberChunk:
			// logger.Infof(c.Request.Context(), "StreamHandler：aws response: %s", string(v.Value.Bytes))
			var response *DeepSeekStreamResponse
			err := json.NewDecoder(bytes.NewReader(v.Value.Bytes)).Decode(&response)
			if err != nil {
				logger.SysError("error unmarshalling stream response: " + err.Error())
				return false
			}
			// claudeResp := new(anthropic.StreamResponse)
			// err := json.NewDecoder(bytes.NewReader(v.Value.Bytes)).Decode(claudeResp)
			// if err != nil {
			// 	logger.SysError("error unmarshalling stream response: " + err.Error())
			// 	return false
			// }

			// response, meta := anthropic.StreamResponseClaude2OpenAI(claudeResp)
			// if meta != nil {
			// 	usage.PromptTokens += meta.Usage.InputTokens
			// 	usage.CompletionTokens += meta.Usage.OutputTokens
			// 	if len(meta.Id) > 0 { // only message_start has an id, otherwise it's a finish_reason event.
			// 		id = fmt.Sprintf("chatcmpl-%s", meta.Id)
			// 		return true
			// 	} else { // finish_reason case
			// 		if len(lastToolCallChoice.Delta.ToolCalls) > 0 {
			// 			lastArgs := &lastToolCallChoice.Delta.ToolCalls[len(lastToolCallChoice.Delta.ToolCalls)-1].Function
			// 			if len(lastArgs.Arguments.(string)) == 0 { // compatible with OpenAI sending an empty object `{}` when no arguments.
			// 				lastArgs.Arguments = "{}"
			// 				response.Choices[len(response.Choices)-1].Delta.Content = nil
			// 				response.Choices[len(response.Choices)-1].Delta.ToolCalls = lastToolCallChoice.Delta.ToolCalls
			// 			}
			// 		}
			// 	}
			// }
			if response == nil {
				return true
			}
			response.Id = id
			response.Model = c.GetString(ctxkey.OriginalModel)
			openaiResponse := response.ConvertToOpenAI()
			// response.Created = createdTime

			responseText += openaiResponse.Choices[0].Delta.StringContent()

			jsonStr, err := json.Marshal(openaiResponse)
			if err != nil {
				logger.SysError("error marshalling stream response: " + err.Error())
				return true
			}
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
