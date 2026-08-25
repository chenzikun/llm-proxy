package config

// 以下常量定义了各端点在请求体未指定 model 时使用的默认模型名。
// 修改这些常量即可全局调整默认行为，无需改动业务逻辑代码。
// 如需运行时配置，可在后续版本中从系统 Option 读取并覆盖。
const (
	// DefaultChatModel 渠道健康检测（test channel）的默认模型
	DefaultChatModel = "gpt-3.5-turbo"

	// DefaultModerationModel /v1/moderations 端点的默认模型
	DefaultModerationModel = "text-moderation-stable"

	// DefaultImageModel /v1/images/generations 端点的默认模型
	DefaultImageModel = "dall-e-2"

	// DefaultAudioModel /v1/audio/transcriptions 和 /v1/audio/translations 端点的默认模型
	DefaultAudioModel = "whisper-1"

	// DefaultFilesModel /v1/files 端点路由时使用的模型（用于渠道选择）
	DefaultFilesModel = "gpt-4o-mini"

	// DefaultFineTuningModel /v1/fine_tuning/jobs 端点路由时使用的模型（用于渠道选择）
	DefaultFineTuningModel = "gpt-4o-mini"
)
