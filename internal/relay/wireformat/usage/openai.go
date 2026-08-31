package usage

import (
	"bytes"
	"encoding/json"

	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

type openaiShape struct {
	Usage *struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		TotalTokens         int `json:"total_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	} `json:"usage"`
}

func (s openaiShape) toUsage() *entity.Usage {
	u := buildUsage(s.Usage.PromptTokens, s.Usage.CompletionTokens, s.Usage.PromptTokensDetails.CachedTokens)
	// 上游给出的 total_tokens 可能包含 reasoning 等未单列的部分，优先采信
	if s.Usage.TotalTokens > 0 {
		u.TotalTokens = s.Usage.TotalTokens
	}
	return u
}

// OpenAI 解析 OpenAI 兼容响应中的 usage。
//
// 流式响应仅在客户端指定 stream_options.include_usage 时携带 usage，
// 未携带时返回 false，由调用方按 0 结算并告警。
func OpenAI(body []byte, isStream bool) (*entity.Usage, bool) {
	if !isStream {
		var s openaiShape
		if err := json.Unmarshal(body, &s); err == nil && s.Usage != nil {
			return s.toUsage(), true
		}
		return nil, false
	}
	var last *entity.Usage
	for _, line := range bytes.Split(body, []byte("\n")) {
		payload, isData := sseData(line)
		if !isData {
			continue
		}
		var s openaiShape
		if err := json.Unmarshal(payload, &s); err == nil && s.Usage != nil {
			last = s.toUsage()
		}
	}
	return last, last != nil
}
