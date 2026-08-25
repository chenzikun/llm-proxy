package openai

import (
	"github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
)

var CompatibleChannels = []int{
	// todo: zikun 重要；当直接使用proxy访问呢endpoint的时候需要取消掉这个
	channeltype.Azure,
	channeltype.AI360,
	channeltype.Moonshot,
	channeltype.Baichuan,
	channeltype.Minimax,
	channeltype.Doubao,
	channeltype.Mistral,
	channeltype.Groq,
	channeltype.LingYiWanWu,
	channeltype.StepFun,
	channeltype.DeepSeek,
	channeltype.TogetherAI,
	channeltype.Novita,
	channeltype.SiliconFlow,
	channeltype.SagemakerEndpoint,
}

func GetCompatibleChannelMeta(channelType int) (string, []string) {
	// todo:zikun 动态添加模型
	modelList, err := model.GetModelListByChannelType(channelType)
	if err != nil {
		return "", nil
	}
	switch channelType {
	case channeltype.Azure:
		return "azure", modelList
	case channeltype.AI360:
		return "360", modelList
	case channeltype.Moonshot:
		return "moonshot", modelList
	case channeltype.Baichuan:
		return "baichuan", modelList
	case channeltype.Minimax:
		return "minimax", modelList
	case channeltype.Mistral:
		return "mistralai", modelList
	case channeltype.Groq:
		return "groq", modelList
	case channeltype.LingYiWanWu:
		return "lingyiwanwu", modelList
	case channeltype.StepFun:
		return "stepfun", modelList
	case channeltype.DeepSeek:
		return "deepseek", modelList
	case channeltype.TogetherAI:
		return "together.ai", modelList
	case channeltype.Doubao:
		return "doubao", modelList
	case channeltype.Novita:
		return "novita", modelList
	case channeltype.SiliconFlow:
		return "siliconflow", modelList
	case channeltype.SagemakerEndpoint:
		return "sagemaker-endpoint", modelList
	default:
		return "openai", modelList
	}
}
