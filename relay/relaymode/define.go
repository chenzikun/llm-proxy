package relaymode

const (
	Unknown = iota
	ChatCompletions
	Completions
	Embeddings
	Rerank
	Moderations
	ImagesGenerations
	Edits
	AudioSpeech        // 语音合成 tts
	AudioTranscription // 音频转录 stt
	AudioTranslation   // 音频翻译
	// Proxy is a special relay mode for proxying requests to custom upstream
	Proxy
)
