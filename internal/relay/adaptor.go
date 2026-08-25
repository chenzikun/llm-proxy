package relay

import (
	"github.com/zicorn/llm-proxy/internal/relay/adaptor"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aiproxy"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/ali"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/anthropic"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/aws"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/baidu"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/cloudflare"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/cohere"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/coze"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/deepl"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/gemini"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/ollama"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/palm"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/proxy"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/tencent"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/vertexai"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/xunfei"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/zhipu"
	"github.com/zicorn/llm-proxy/internal/relay/apitype"
)

func GetAdaptor(apiType int) adaptor.RelayAdaptor {
	switch apiType {
	case apitype.AIProxyLibrary:
		return &aiproxy.Adaptor{}
	case apitype.Ali:
		return &ali.Adaptor{}
	case apitype.Anthropic:
		return &anthropic.Adaptor{}
	case apitype.AwsClaude:
		return &aws.AwsRelayAdaptor{}
	case apitype.Baidu:
		return &baidu.Adaptor{}
	case apitype.Gemini:
		return &gemini.Adaptor{}
	case apitype.OpenAI:
		return &openai.OpenAIAdaptor{}
	case apitype.PaLM:
		return &palm.Adaptor{}
	case apitype.Tencent:
		return &tencent.Adaptor{}
	case apitype.Xunfei:
		return &xunfei.Adaptor{}
	case apitype.Zhipu:
		return &zhipu.Adaptor{}
	case apitype.Ollama:
		return &ollama.Adaptor{}
	case apitype.Coze:
		return &coze.Adaptor{}
	case apitype.Cohere:
		return &cohere.Adaptor{}
	case apitype.Cloudflare:
		return &cloudflare.Adaptor{}
	case apitype.DeepL:
		return &deepl.Adaptor{}
	case apitype.VertexAI:
		return &vertexai.Adaptor{}
	case apitype.Proxy:
		return &proxy.ProxyAdaptor{}
	}
	return nil
}
