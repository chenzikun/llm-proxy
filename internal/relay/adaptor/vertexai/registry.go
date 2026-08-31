package vertexai

import (
	"github.com/zicorn/llm-proxy/internal/objects"
	"net/http"

	"github.com/gin-gonic/gin"
	claude "github.com/zicorn/llm-proxy/internal/relay/adaptor/vertexai/claude"
	gemini "github.com/zicorn/llm-proxy/internal/relay/adaptor/vertexai/gemini"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
)

type VertexAIModelType int

const (
	VerterAIClaude VertexAIModelType = iota + 1
	VerterAIGemini
)

var modelMapping = map[string]VertexAIModelType{}
var modelList = []string{}

func init() {
	modelList = append(modelList, claude.ModelList...)
	for _, model := range claude.ModelList {
		modelMapping[model] = VerterAIClaude
	}

	modelList = append(modelList, gemini.ModelList...)
	for _, model := range gemini.ModelList {
		modelMapping[model] = VerterAIGemini
	}
}

type innerAIAdapter interface {
	ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error)
	DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (usage *entity.Usage, responseText string, err *objects.ErrorWithStatusCode)
}

// GetAdaptor 按模型名分派到对应厂商的适配器。
//
// Vertex AI 同时托管 Google 与 Anthropic 的模型，两者 wire 格式不同：
// gemini-* 走 :generateContent，claude-* 走 :rawPredict。分派条件与
// Adaptor.GetRequestURL 共用 wireformat.IsVertexGeminiModel 保持一致，
// 否则会出现 URL 按 Claude 发出、响应按 Gemini 解析的错配。
//
// 这里按前缀而非 modelMapping 查表：modelList 是过时的硬编码清单，
// 新模型（claude-sonnet-4、gemini-2.5-flash）都不在其中，查表会落到 nil。
func GetAdaptor(model string) innerAIAdapter {
	if wireformat.IsVertexGeminiModel(model) {
		return &gemini.Adaptor{}
	}
	return &claude.Adaptor{}
}
