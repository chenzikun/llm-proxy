package aws

import (
	"errors"
	"fmt"
	"strings"

	"github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/base"
	claude "github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/claude"
	cohere "github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/cohere"
	deepseek "github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/deepseek"
	llama3 "github.com/zicorn/llm-proxy/internal/relay/adaptor/aws/llama3"
)

type AwsModelType int

const (
	AwsClaude AwsModelType = iota + 1
	AwsLlama3
	AwsCohere
	AwsDeepSeek
)

var (
	adaptors = map[string]AwsModelType{}
)

func init() {
	for model_ := range claude.ClaudeModelIDMap {
		adaptors[model_] = AwsClaude
	}
	for model_ := range llama3.Llama3ModelIDMap {
		adaptors[model_] = AwsLlama3
	}
	for model_ := range cohere.CohereModelIDMap {
		adaptors[model_] = AwsCohere
	}
	for model_ := range deepseek.DeepSeekModelIDMap {
		adaptors[model_] = AwsDeepSeek
	}
}

// detectAdaptorTypeByPrefix 通过 Bedrock 模型 ID 前缀推断 adaptor 类型，
// 用于支持未在静态映射表中注册的新模型，无需修改代码即可使用。
func detectAdaptorTypeByPrefix(modelID string) AwsModelType {
	switch {
	case strings.HasPrefix(modelID, "anthropic.claude-"),
		strings.HasPrefix(modelID, "us.anthropic.claude-"),
		strings.HasPrefix(modelID, "eu.anthropic.claude-"),
		strings.HasPrefix(modelID, "ap.anthropic.claude-"):
		return AwsClaude
	case strings.HasPrefix(modelID, "meta.llama3-"),
		strings.HasPrefix(modelID, "us.meta.llama3-"),
		strings.HasPrefix(modelID, "eu.meta.llama3-"),
		strings.HasPrefix(modelID, "ap.meta.llama3-"):
		return AwsLlama3
	case strings.HasPrefix(modelID, "cohere."),
		strings.HasPrefix(modelID, "amazon.rerank-"),
		strings.HasPrefix(modelID, "amazon.titan-embed-"):
		return AwsCohere
	case strings.HasPrefix(modelID, "us.deepseek."),
		strings.HasPrefix(modelID, "deepseek."):
		return AwsDeepSeek
	default:
		return 0
	}
}

func GetProviderAdaptor(modelName string, channelConfig *model.ChannelConfig) (base.AwsProviderAdapter, error) {
	adaptorType, ok := adaptors[modelName]
	if !ok {
		adaptorType = detectAdaptorTypeByPrefix(modelName)
	}
	switch adaptorType {
	case AwsClaude:
		return claude.NewClaudeProviderAdaptor(channelConfig)
	case AwsLlama3:
		return llama3.NewLlama3ProviderAdaptor(channelConfig)
	case AwsCohere:
		return cohere.NewCohereProviderAdaptor(channelConfig)
	case AwsDeepSeek:
		return deepseek.NewDeepSeekProviderAdaptor(channelConfig)
	default:
		return nil, errors.New(fmt.Sprintf("adaptor not found, model: %s, adaptorType: %d", modelName, adaptorType))
	}
}
