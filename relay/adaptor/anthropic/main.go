package anthropic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/image"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/common/render"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/entity"
)

func stopReasonClaude2OpenAI(reason *string) string {
	if reason == nil {
		return ""
	}
	switch *reason {
	case "end_turn":
		return "stop"
	case "stop_sequence":
		return "stop"
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return *reason
	}
}

func ConvertRequest(textRequest entity.GeneralOpenAIRequest) *Request {
	claudeTools := make([]Tool, 0, len(textRequest.Tools))

	for _, tool := range textRequest.Tools {
		if params, ok := tool.Function.Parameters.(map[string]any); ok {
			claudeTool := Tool{
				Name:        tool.Function.Name,
				Description: tool.Function.Description,
				InputSchema: InputSchema{
					Type:       params["type"].(string),
					Properties: params["properties"],
					Required:   params["required"],
				},
			}
			if tool.CacheControl != nil {
				claudeTool.CacheControl = &entity.CacheControl{Type: tool.CacheControl.Type}
			}
			claudeTools = append(claudeTools, claudeTool)
		}
	}

	claudeRequest := Request{
		Model:       textRequest.Model,
		MaxTokens:   textRequest.MaxCompletionTokens,
		Temperature: textRequest.Temperature,
		TopP:        textRequest.TopP,
		TopK:        textRequest.TopK,
		Stream:      textRequest.Stream,
		Tools:       claudeTools,
	}
	if len(claudeTools) > 0 {
		claudeToolChoice := struct {
			Type string `json:"type"`
			Name string `json:"name,omitempty"`
		}{Type: "auto"} // default value https://docs.anthropic.com/en/docs/build-with-claude/tool-use#controlling-claudes-output
		if choice, ok := textRequest.ToolChoice.(map[string]any); ok {
			if function, ok := choice["function"].(map[string]any); ok {
				claudeToolChoice.Type = "tool"
				claudeToolChoice.Name = function["name"].(string)
			}
		} else if toolChoiceType, ok := textRequest.ToolChoice.(string); ok {
			if toolChoiceType == "any" {
				claudeToolChoice.Type = toolChoiceType
			}
		}
		claudeRequest.ToolChoice = claudeToolChoice
	}
	if claudeRequest.MaxTokens == 0 {
		claudeRequest.MaxTokens = textRequest.MaxTokens
	}
	if textRequest.ReasoningEffort != "" {
		if claudeRequest.MaxTokens == 0 {
			claudeRequest.MaxTokens = 24000
		}
		claudeRequest.Thinking = &Thinking{
			Type:         "enabled",
			BudgetTokens: claudeRequest.MaxTokens * 2 / 3,
		}
	} else {
		claudeRequest.Thinking = nil
	}
	if claudeRequest.MaxTokens == 0 {
		claudeRequest.MaxTokens = 4096
	}
	// legacy model name mapping
	if claudeRequest.Model == "claude-instant-1" {
		claudeRequest.Model = "claude-instant-1.1"
	} else if claudeRequest.Model == "claude-2" {
		claudeRequest.Model = "claude-2.1"
	}
	for _, message := range textRequest.Messages {
		if message.Role == "system" && claudeRequest.System == nil {
			// System can be either a plain string or a []Content array. Use the
			// array form whenever the source content is structured so any
			// cache_control markers survive — Anthropic accepts both shapes.
			if message.IsStringContent() {
				claudeRequest.System = message.StringContent()
			} else {
				var systemBlocks []Content
				for _, part := range message.ParseContent() {
					if part.Type != entity.ContentTypeText {
						continue
					}
					block := Content{
						Type: "text",
						Text: part.Text,
					}
					if part.CacheControl != nil {
						block.CacheControl = &entity.CacheControl{Type: part.CacheControl.Type}
					}
					systemBlocks = append(systemBlocks, block)
				}
				if len(systemBlocks) > 0 {
					claudeRequest.System = systemBlocks
				}
			}
			continue
		}
		claudeMessage := Message{
			Role: message.Role,
		}
		var content Content
		if message.IsStringContent() {
			content.Type = "text"
			content.Text = message.StringContent()
			if message.Role == "tool" {
				claudeMessage.Role = "user"
				content.Type = "tool_result"
				content.Content = content.Text
				content.Text = ""
				content.ToolUseId = message.ToolCallId
			}
			claudeMessage.Content = append(claudeMessage.Content, content)
			for i := range message.ToolCalls {
				inputParam := make(map[string]any)
				_ = json.Unmarshal([]byte(message.ToolCalls[i].Function.Arguments.(string)), &inputParam)
				toolUseBlock := Content{
					Type:  "tool_use",
					Id:    message.ToolCalls[i].Id,
					Name:  message.ToolCalls[i].Function.Name,
					Input: inputParam,
				}
				if message.ToolCalls[i].CacheControl != nil {
					toolUseBlock.CacheControl = &entity.CacheControl{Type: message.ToolCalls[i].CacheControl.Type}
				}
				claudeMessage.Content = append(claudeMessage.Content, toolUseBlock)
			}
			claudeRequest.Messages = append(claudeRequest.Messages, claudeMessage)
			continue
		}
		// Tool-role messages with structured content: emit a single tool_result
		// block; lift any cache_control found on inner text parts up to the
		// tool_result block (Anthropic carries cache_control on the result
		// block, not on its inner content).
		if message.Role == "tool" {
			toolResult := Content{
				Type:      "tool_result",
				ToolUseId: message.ToolCallId,
			}
			var textBuilder strings.Builder
			for _, part := range message.ParseContent() {
				if part.Type == entity.ContentTypeText {
					textBuilder.WriteString(part.Text)
					if part.CacheControl != nil && toolResult.CacheControl == nil {
						toolResult.CacheControl = &entity.CacheControl{Type: part.CacheControl.Type}
					}
				}
			}
			toolResult.Content = textBuilder.String()
			claudeMessage.Role = "user"
			claudeMessage.Content = append(claudeMessage.Content, toolResult)
			claudeRequest.Messages = append(claudeRequest.Messages, claudeMessage)
			continue
		}
		// Always start with a non-nil slice — Anthropic/Bedrock require content
		// to be a JSON array, and a nil []Content marshals to `null`, which
		// fails validation ("messages.N.content: Input should be a valid list").
		contents := make([]Content, 0)
		openaiContent := message.ParseContent()
		for _, part := range openaiContent {
			var content Content
			if part.Type == entity.ContentTypeText {
				content.Type = "text"
				content.Text = part.Text
				if part.CacheControl != nil {
					content.CacheControl = &entity.CacheControl{Type: part.CacheControl.Type}
				}
			} else if part.Type == entity.ContentTypeImageURL {
				content.Type = "image"
				content.Source = &ImageSource{
					Type: "base64",
				}
				mimeType, data, _ := image.GetImageFromUrl(part.ImageURL.Url)
				content.Source.MediaType = mimeType
				content.Source.Data = data
				if part.CacheControl != nil {
					content.CacheControl = &entity.CacheControl{Type: part.CacheControl.Type}
				}
			} else {
				continue
			}
			contents = append(contents, content)
		}
		// An assistant turn carrying tool_calls may arrive with structured (or
		// null) content alongside the calls; emit the tool_use blocks here too,
		// otherwise they'd be silently dropped on this branch.
		for i := range message.ToolCalls {
			inputParam := make(map[string]any)
			if argStr, ok := message.ToolCalls[i].Function.Arguments.(string); ok && argStr != "" {
				_ = json.Unmarshal([]byte(argStr), &inputParam)
			}
			toolUseBlock := Content{
				Type:  "tool_use",
				Id:    message.ToolCalls[i].Id,
				Name:  message.ToolCalls[i].Function.Name,
				Input: inputParam,
			}
			if message.ToolCalls[i].CacheControl != nil {
				toolUseBlock.CacheControl = &entity.CacheControl{Type: message.ToolCalls[i].CacheControl.Type}
			}
			contents = append(contents, toolUseBlock)
		}
		claudeMessage.Content = contents
		claudeRequest.Messages = append(claudeRequest.Messages, claudeMessage)
	}
	return &claudeRequest
}

// https://docs.anthropic.com/claude/reference/messages-streaming
//
// toolIdByBlockIndex maps a Claude content_block index to the tool_use id that
// arrived on its content_block_start event. It must be supplied (and reused
// across calls within the same stream) so input_json_delta chunks can keep
// emitting a non-empty tool_calls[].id — OpenAI clients (e.g. LangChain) drop
// the id when a later chunk omits it, which then breaks ToolMessage
// construction with `tool_call_id: Input should be a valid string`.
func StreamResponseClaude2OpenAI(claudeResponse *StreamResponse, toolIdByBlockIndex map[int]string) (*openai.ChatCompletionsStreamResponse, *Response) {
	var response *Response
	var responseText string
	var stopReason string
	var thinkingText string
	tools := make([]entity.Tool, 0)

	switch claudeResponse.Type {
	case "message_start":
		return nil, claudeResponse.Message
	case "content_block_start":
		if claudeResponse.ContentBlock != nil {
			responseText = claudeResponse.ContentBlock.Text
			if claudeResponse.ContentBlock.Type == "tool_use" {
				if toolIdByBlockIndex != nil {
					toolIdByBlockIndex[claudeResponse.Index] = claudeResponse.ContentBlock.Id
				}
				idx := claudeResponse.Index
				tools = append(tools, entity.Tool{
					Id:    claudeResponse.ContentBlock.Id,
					Index: &idx,
					Type:  "function",
					Function: entity.Function{
						Name:      claudeResponse.ContentBlock.Name,
						Arguments: "",
					},
				})
			} else if claudeResponse.ContentBlock.Type == "thinking" {
				thinkingText += claudeResponse.ContentBlock.Thinking
			}
		}
	case "content_block_delta":
		if claudeResponse.Delta != nil {
			responseText = claudeResponse.Delta.Text
			if claudeResponse.Delta.Type == "input_json_delta" {
				var toolId string
				if toolIdByBlockIndex != nil {
					toolId = toolIdByBlockIndex[claudeResponse.Index]
				}
				idx := claudeResponse.Index
				tools = append(tools, entity.Tool{
					Id:    toolId,
					Index: &idx,
					Type:  "function",
					Function: entity.Function{
						Arguments: claudeResponse.Delta.PartialJson,
					},
				})
			} else if claudeResponse.Delta.Type == "thinking_delta" {
				thinkingText += claudeResponse.Delta.Thinking
			}
		}
	case "message_delta":
		if claudeResponse.Usage != nil {
			response = &Response{
				Usage: *claudeResponse.Usage,
			}
		}
		if claudeResponse.Delta != nil && claudeResponse.Delta.StopReason != nil {
			stopReason = *claudeResponse.Delta.StopReason
		}
	}
	var choice openai.ChatCompletionsStreamResponseChoice
	choice.Delta.Content = responseText
	choice.Delta.ReasoningContent = thinkingText
	if len(tools) > 0 {
		choice.Delta.Content = nil // compatible with other OpenAI derivative applications, like LobeOpenAICompatibleFactory ...
		choice.Delta.ToolCalls = tools
	}
	choice.Delta.Role = "assistant"
	finishReason := stopReasonClaude2OpenAI(&stopReason)
	if finishReason != "null" {
		choice.FinishReason = &finishReason
	}
	var openaiResponse openai.ChatCompletionsStreamResponse
	openaiResponse.Object = "chat.completion.chunk"
	openaiResponse.Choices = []openai.ChatCompletionsStreamResponseChoice{choice}
	return &openaiResponse, response
}

func ResponseClaude2OpenAI(claudeResponse *Response) *openai.TextResponse {
	var responseText string
	var thinkingText string
	if len(claudeResponse.Content) > 0 {
		// thinkingText = claudeResponse.Content[0].Thinking
		// responseText = claudeResponse.Content[1].Text
		for _, i := range claudeResponse.Content {
			if i.Type == "text" {
				responseText = i.Text
			} else if i.Type == "thinking" {
				thinkingText = i.Thinking
			} else if i.Type == "redacted_thinking" {
				// TODO
			}
		}
	}
	tools := make([]entity.Tool, 0)
	for _, v := range claudeResponse.Content {
		if v.Type == "tool_use" {
			args, _ := json.Marshal(v.Input)
			tools = append(tools, entity.Tool{
				Id:   v.Id,
				Type: "function", // compatible with other OpenAI derivative applications
				Function: entity.Function{
					Name:      v.Name,
					Arguments: string(args),
				},
			})
		}
	}
	choice := openai.TextResponseChoice{
		Index: 0,
		Message: entity.Message{
			Role:             "assistant",
			Content:          responseText,
			ReasoningContent: thinkingText,
			Name:             nil,
			ToolCalls:        tools,
		},
		FinishReason: stopReasonClaude2OpenAI(claudeResponse.StopReason),
	}
	fullTextResponse := openai.TextResponse{
		Id:      fmt.Sprintf("chatcmpl-%s", claudeResponse.Id),
		Model:   claudeResponse.Model,
		Object:  "chat.completion",
		Created: helper.GetTimestamp(),
		Choices: []openai.TextResponseChoice{choice},
	}
	return &fullTextResponse
}

func StreamHandler(c *gin.Context, resp *http.Response) (*objects.ErrorWithStatusCode, *entity.Usage) {
	createdTime := helper.GetTimestamp()
	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := strings.Index(string(data), "\n"); i >= 0 {
			return i + 1, data[0:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		return 0, nil, nil
	})

	common.SetEventStreamHeaders(c)

	var usage entity.Usage
	var modelName string
	var id string
	var lastToolCallChoice openai.ChatCompletionsStreamResponseChoice
	toolIdByBlockIndex := make(map[int]string)

	for scanner.Scan() {
		data := scanner.Text()
		if len(data) < 6 || !strings.HasPrefix(data, "data:") {
			continue
		}
		data = strings.TrimPrefix(data, "data:")
		data = strings.TrimSpace(data)

		var claudeResponse StreamResponse
		err := json.Unmarshal([]byte(data), &claudeResponse)
		if err != nil {
			logger.SysError("error unmarshalling stream response: " + err.Error())
			continue
		}

		response, meta := StreamResponseClaude2OpenAI(&claudeResponse, toolIdByBlockIndex)
		if meta != nil {
			usage.PromptTokens += meta.Usage.InputTokens
			usage.CompletionTokens += meta.Usage.OutputTokens
			if len(meta.Id) > 0 { // only message_start has an id, otherwise it's a finish_reason event.
				modelName = meta.Model
				id = fmt.Sprintf("chatcmpl-%s", meta.Id)
				continue
			} else { // finish_reason case
				if len(lastToolCallChoice.Delta.ToolCalls) > 0 {
					lastArgs := &lastToolCallChoice.Delta.ToolCalls[len(lastToolCallChoice.Delta.ToolCalls)-1].Function
					if len(lastArgs.Arguments.(string)) == 0 { // compatible with OpenAI sending an empty object `{}` when no arguments.
						lastArgs.Arguments = "{}"
						response.Choices[len(response.Choices)-1].Delta.Content = nil
						response.Choices[len(response.Choices)-1].Delta.ToolCalls = lastToolCallChoice.Delta.ToolCalls
					}
				}
			}
		}
		if response == nil {
			continue
		}

		response.Id = id
		response.Model = modelName
		response.Created = createdTime

		for _, choice := range response.Choices {
			if len(choice.Delta.ToolCalls) > 0 {
				lastToolCallChoice = choice
			}
		}
		err = render.ObjectData(c, response)
		if err != nil {
			logger.SysError(err.Error())
		}
	}

	if err := scanner.Err(); err != nil {
		logger.SysError("error reading stream: " + err.Error())
	}

	render.Done(c)

	err := resp.Body.Close()
	if err != nil {
		return objects.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}
	return nil, &usage
}

func Handler(c *gin.Context, resp *http.Response, promptTokens int, modelName string) (*objects.ErrorWithStatusCode, *entity.Usage) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return objects.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError), nil
	}
	err = resp.Body.Close()
	if err != nil {
		return objects.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError), nil
	}
	var claudeResponse Response
	err = json.Unmarshal(responseBody, &claudeResponse)
	if err != nil {
		return objects.ErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError), nil
	}
	if claudeResponse.Error.Type != "" {
		return &objects.ErrorWithStatusCode{
			Error: objects.Error{
				Message: claudeResponse.Error.Message,
				Type:    claudeResponse.Error.Type,
				Param:   "",
				Code:    claudeResponse.Error.Type,
			},
			StatusCode: resp.StatusCode,
		}, nil
	}
	fullTextResponse := ResponseClaude2OpenAI(&claudeResponse)
	fullTextResponse.Model = modelName
	usage := entity.Usage{
		PromptTokens:     claudeResponse.Usage.InputTokens,
		CompletionTokens: claudeResponse.Usage.OutputTokens,
		TotalTokens:      claudeResponse.Usage.InputTokens + claudeResponse.Usage.OutputTokens,
	}
	fullTextResponse.Usage = usage
	jsonResponse, err := json.Marshal(fullTextResponse)
	if err != nil {
		return objects.ErrorWrapper(err, "marshal_response_body_failed", http.StatusInternalServerError), nil
	}
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = c.Writer.Write(jsonResponse)
	return nil, &usage
}
