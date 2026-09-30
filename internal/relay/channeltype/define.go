package channeltype

const (
	Unknown = iota
	OpenAI
	API2D
	Azure
	CloseAI
	OpenAISB
	OpenAIMax
	OhMyGPT
	Custom
	Ails
	AIProxy
	PaLM
	API2GPT
	AIGC2D
	Anthropic
	Baidu
	Zhipu
	Ali
	Xunfei
	AI360
	OpenRouter
	AIProxyLibrary
	FastGPT
	Tencent
	Gemini
	Moonshot
	Baichuan
	Minimax
	Mistral
	Groq
	Ollama
	LingYiWanWu
	StepFun
	AwsClaude
	Coze
	Cohere
	DeepSeek
	Cloudflare
	DeepL
	TogetherAI
	Doubao
	Novita
	VertextAI
	Proxy
	SiliconFlow
	// Seedance 占的是原先的空位 Dummy0：它的取值 45 已经在库里被用掉了，
	// 在末尾追加会让 SagemakerEndpoint / Dummy 整体后移，既有 channel.type 全部错位。
	Seedance
	SagemakerEndpoint
	Dummy
)
