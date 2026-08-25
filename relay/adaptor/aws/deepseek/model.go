package aws

import (
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/entity"
)

type DeepSeekStreamResponseChoice struct {
	Index        int            `json:"index"`
	Message      entity.Message `json:"message"`
	FinishReason *string        `json:"finish_reason,omitempty"`
}

type DeepSeekStreamResponse struct {
	Id      string                         `json:"id"`
	Object  string                         `json:"object"`
	Created int64                          `json:"created"`
	Model   string                         `json:"model"`
	Choices []DeepSeekStreamResponseChoice `json:"choices"`
	Usage   *entity.Usage                  `json:"usage,omitempty"`
}

func (r *DeepSeekStreamResponse) ConvertToOpenAI() *openai.ChatCompletionsStreamResponse {
	resp := &openai.ChatCompletionsStreamResponse{
		Id:      r.Id,
		Object:  r.Object,
		Created: r.Created,
		Model:   r.Model,
		Choices: make([]openai.ChatCompletionsStreamResponseChoice, 0),
		Usage:   r.Usage,
	}
	for _, choice := range r.Choices {
		resp.Choices = append(resp.Choices, openai.ChatCompletionsStreamResponseChoice{
			Index: choice.Index,
			Delta: choice.Message,
		})
	}
	return resp
}
