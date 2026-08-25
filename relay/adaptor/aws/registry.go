package aws

import (
	"errors"
	"fmt"

	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay/adaptor/aws/base"
	claude "github.com/songquanpeng/one-api/relay/adaptor/aws/claude"
	cohere "github.com/songquanpeng/one-api/relay/adaptor/aws/cohere"
	deepseek "github.com/songquanpeng/one-api/relay/adaptor/aws/deepseek"
	llama3 "github.com/songquanpeng/one-api/relay/adaptor/aws/llama3"
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

func GetProviderAdaptor(model string, channelConfig *model.ChannelConfig) (base.AwsProviderAdapter, error) {
	adaptorType := adaptors[model]
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
		return nil, errors.New(fmt.Sprintf("adaptor not found, model: %s, adaptorType: %d", model, adaptorType))
	}
}
