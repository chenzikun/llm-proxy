package openai

import (
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

type TextContent struct {
	Type string `json:"type,omitempty"`
	Text string `json:"text,omitempty"`
}

type ImageContent struct {
	Type     string           `json:"type,omitempty"`
	ImageURL *entity.ImageURL `json:"image_url,omitempty"`
}

type ChatRequest struct {
	Model     string           `json:"model"`
	Messages  []entity.Message `json:"messages"`
	MaxTokens int              `json:"max_tokens"`
}

type TextRequest struct {
	Model     string           `json:"model"`
	Messages  []entity.Message `json:"messages"`
	Prompt    string           `json:"prompt"`
	MaxTokens int              `json:"max_tokens"`
	//Stream   bool      `json:"stream"`
}

// ImageRequest docs: https://platform.openai.com/docs/api-reference/images/create
type ImageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt" binding:"required"`
	N              int    `json:"n,omitempty"`
	Size           string `json:"size,omitempty"`
	Quality        string `json:"quality,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
	Style          string `json:"style,omitempty"`
	User           string `json:"user,omitempty"`
}

type WhisperJSONResponse struct {
	Text string `json:"text,omitempty"`
}

type WhisperVerboseJSONResponse struct {
	Task     string    `json:"task,omitempty"`
	Language string    `json:"language,omitempty"`
	Duration float64   `json:"duration,omitempty"`
	Text     string    `json:"text,omitempty"`
	Segments []Segment `json:"segments,omitempty"`
}

type Segment struct {
	Id               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

type TextToSpeechRequest struct {
	Model          string  `json:"model" binding:"required"`
	Input          string  `json:"input" binding:"required"`
	Voice          string  `json:"voice" binding:"required"`
	Speed          float64 `json:"speed"`
	ResponseFormat string  `json:"response_format"`
}

type UsageOrResponseText struct {
	*entity.Usage
	ResponseText string
}

type SlimTextResponse struct {
	Choices      []TextResponseChoice `json:"choices"`
	entity.Usage `json:"usage"`
	Error        objects.Error `json:"error"`
}

type TextResponseChoice struct {
	Index          int `json:"index"`
	entity.Message `json:"message"`
	FinishReason   string `json:"finish_reason"`
}

type TextResponse struct {
	Id           string               `json:"id"`
	Model        string               `json:"model,omitempty"`
	Object       string               `json:"object"`
	Created      int64                `json:"created"`
	Choices      []TextResponseChoice `json:"choices"`
	entity.Usage `json:"usage"`
}

type EmbeddingRequest struct {
	Input          any    `json:"input"`
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format"`
	// cohere embed 需要的参数，详见 https://docs.cohere.com/reference/embed
	InputType string `json:"input_type"`
}

type EmbeddingResponseItem struct {
	Object    string        `json:"object"`
	Index     int           `json:"index"`
	Embedding []interface{} `json:"embedding"`
}

type EmbeddingResponse struct {
	Object       string                  `json:"object"`
	Data         []EmbeddingResponseItem `json:"data"`
	Model        string                  `json:"model"`
	entity.Usage `json:"usage"`
}

type ImageData struct {
	Url           string `json:"url,omitempty"`
	B64Json       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

type ImageResponse struct {
	Created int64       `json:"created"`
	Data    []ImageData `json:"data"`
	//model.Usage `json:"usage"`
}

type ChatCompletionsStreamResponseChoice struct {
	Index        int            `json:"index"`
	Delta        entity.Message `json:"delta"`
	FinishReason *string        `json:"finish_reason,omitempty"`
}

type ChatCompletionsStreamResponse struct {
	Id      string                                `json:"id"`
	Object  string                                `json:"object"`
	Created int64                                 `json:"created"`
	Model   string                                `json:"model"`
	Choices []ChatCompletionsStreamResponseChoice `json:"choices"`
	Usage   *entity.Usage                         `json:"usage,omitempty"`
}

type CompletionsStreamResponse struct {
	Choices []struct {
		Text         string `json:"text"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
}
