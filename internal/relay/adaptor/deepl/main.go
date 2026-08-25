package deepl

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/helper"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
	"github.com/zicorn/llm-proxy/internal/relay/constant"
	"github.com/zicorn/llm-proxy/internal/relay/constant/finishreason"
	"github.com/zicorn/llm-proxy/internal/relay/constant/role"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
	"io"
	"net/http"
)

// https://developers.deepl.com/docs/getting-started/your-first-api-request

func ConvertRequest(textRequest entity.GeneralOpenAIRequest) (*Request, string) {
	var text string
	if len(textRequest.Messages) != 0 {
		text = textRequest.Messages[len(textRequest.Messages)-1].StringContent()
	}
	deeplRequest := Request{
		TargetLang: parseLangFromModelName(textRequest.Model),
		Text:       []string{text},
	}
	return &deeplRequest, text
}

func StreamResponseDeepL2OpenAI(deeplResponse *Response) *openai.ChatCompletionsStreamResponse {
	var choice openai.ChatCompletionsStreamResponseChoice
	if len(deeplResponse.Translations) != 0 {
		choice.Delta.Content = deeplResponse.Translations[0].Text
	}
	choice.Delta.Role = role.Assistant
	choice.FinishReason = &constant.StopFinishReason
	openaiResponse := openai.ChatCompletionsStreamResponse{
		Object:  constant.StreamObject,
		Created: helper.GetTimestamp(),
		Choices: []openai.ChatCompletionsStreamResponseChoice{choice},
	}
	return &openaiResponse
}

func ResponseDeepL2OpenAI(deeplResponse *Response) *openai.TextResponse {
	var responseText string
	if len(deeplResponse.Translations) != 0 {
		responseText = deeplResponse.Translations[0].Text
	}
	choice := openai.TextResponseChoice{
		Index: 0,
		Message: entity.Message{
			Role:    role.Assistant,
			Content: responseText,
			Name:    nil,
		},
		FinishReason: finishreason.Stop,
	}
	fullTextResponse := openai.TextResponse{
		Object:  constant.NonStreamObject,
		Created: helper.GetTimestamp(),
		Choices: []openai.TextResponseChoice{choice},
	}
	return &fullTextResponse
}

func StreamHandler(c *gin.Context, resp *http.Response, modelName string) *objects.ErrorWithStatusCode {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return objects.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	err = resp.Body.Close()
	if err != nil {
		return objects.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}
	var deeplResponse Response
	err = json.Unmarshal(responseBody, &deeplResponse)
	if err != nil {
		return objects.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	fullTextResponse := StreamResponseDeepL2OpenAI(&deeplResponse)
	fullTextResponse.Model = modelName
	fullTextResponse.Id = helper.GetResponseID(c)
	jsonData, err := json.Marshal(fullTextResponse)
	if err != nil {
		return objects.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	common.SetEventStreamHeaders(c)
	c.Stream(func(w io.Writer) bool {
		if jsonData != nil {
			c.Render(-1, common.CustomEvent{Data: "data: " + string(jsonData)})
			jsonData = nil
			return true
		}
		c.Render(-1, common.CustomEvent{Data: "data: [DONE]"})
		return false
	})
	_ = resp.Body.Close()
	return nil
}

func Handler(c *gin.Context, resp *http.Response, modelName string) *objects.ErrorWithStatusCode {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return objects.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	err = resp.Body.Close()
	if err != nil {
		return objects.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}
	var deeplResponse Response
	err = json.Unmarshal(responseBody, &deeplResponse)
	if err != nil {
		return objects.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if deeplResponse.Message != "" {
		return &objects.ErrorWithStatusCode{
			Error: objects.Error{
				Message: deeplResponse.Message,
				Code:    "deepl_error",
			},
			StatusCode: resp.StatusCode,
		}
	}
	fullTextResponse := ResponseDeepL2OpenAI(&deeplResponse)
	fullTextResponse.Model = modelName
	fullTextResponse.Id = helper.GetResponseID(c)
	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return objects.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError)
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = c.Writer.Write(jsonResponse)
	return nil
}
