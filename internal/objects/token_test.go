package objects

import (
	"testing"

	"github.com/zicorn/llm-proxy/internal/relay/relaymode"
)

func TestPredictAudioPromptTokenCountCountsRunes(t *testing.T) {
	// TTS 上游按字符计价。中文在 UTF-8 下每字 3 字节，按字节计会超收 3 倍。
	if got := PredictAudioPromptTokenCount("你好世界", relaymode.AudioSpeech); got != 4 {
		t.Errorf("中文 4 字得到 %d，期望 4（按字符而非字节）", got)
	}
	if got := PredictAudioPromptTokenCount("hello", relaymode.AudioSpeech); got != 5 {
		t.Errorf("英文 5 字母得到 %d，期望 5", got)
	}
	if got := PredictAudioPromptTokenCount("", relaymode.AudioSpeech); got != 0 {
		t.Errorf("空串得到 %d，期望 0", got)
	}
}
