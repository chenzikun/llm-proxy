package usage

import (
	"bytes"
	"encoding/json"

	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

type geminiShape struct {
	UsageMetadata struct {
		PromptTokenCount        int `json:"promptTokenCount"`
		CandidatesTokenCount    int `json:"candidatesTokenCount"`
		CachedContentTokenCount int `json:"cachedContentTokenCount"`
	} `json:"usageMetadata"`
}

func (s geminiShape) ok() bool {
	return s.UsageMetadata.PromptTokenCount > 0 || s.UsageMetadata.CandidatesTokenCount > 0
}

func (s geminiShape) toUsage() *entity.Usage {
	m := s.UsageMetadata
	return buildUsage(m.PromptTokenCount, m.CandidatesTokenCount, m.CachedContentTokenCount)
}

// Gemini 解析 Gemini API 与 Vertex-Gemini 响应中的 usageMetadata。
func Gemini(body []byte, isStream bool) (*entity.Usage, bool) {
	if !isStream {
		var s geminiShape
		if err := json.Unmarshal(body, &s); err == nil && s.ok() {
			return s.toUsage(), true
		}
		return nil, false
	}
	// 流式下每个 chunk 的 usageMetadata 都是累计值，取最后一个有效的
	var last *entity.Usage
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, isData := sseData(line)
		if !isData {
			continue
		}
		var s geminiShape
		if err := json.Unmarshal(payload, &s); err == nil && s.ok() {
			last = s.toUsage()
		}
	}
	return last, last != nil
}
