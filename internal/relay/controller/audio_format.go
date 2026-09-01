package controller

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
)

const (
	contentTypeJSON = "application/json; charset=utf-8"
	contentTypeText = "text/plain; charset=utf-8"
)

// formatTimestamp 把秒数格式化为字幕时间戳。sep 为毫秒分隔符：SRT 用逗号，WebVTT 用点号。
//
// 先整体换算成毫秒再拆分，避免"秒取整 + 单独算毫秒"在 1.9995 这类值上得到
// 00:00:01,1000 的越界结果。
func formatTimestamp(seconds float64, sep string) string {
	if seconds < 0 {
		seconds = 0
	}
	totalMs := int64(math.Round(seconds * 1000))
	h := totalMs / 3600000
	m := (totalMs % 3600000) / 60000
	s := (totalMs % 60000) / 1000
	ms := totalMs % 1000
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", h, m, s, sep, ms)
}

// convertVerboseJSON 把上游的 verbose_json 响应转换为客户端要求的格式。
//
// 代理固定以 verbose_json 请求上游以获得 duration 用于计费，因此必须在这里
// 降级回客户端原本要求的格式，否则客户端会收到与其请求不符的响应体。
func convertVerboseJSON(resp *openai.WhisperVerboseJSONResponse, format string) (string, string, error) {
	switch format {
	case "text":
		return resp.Text, contentTypeText, nil

	case "srt":
		var b strings.Builder
		for i, seg := range resp.Segments {
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
				i+1,
				formatTimestamp(seg.Start, ","),
				formatTimestamp(seg.End, ","),
				strings.TrimSpace(seg.Text))
		}
		return b.String(), contentTypeText, nil

	case "vtt":
		var b strings.Builder
		b.WriteString("WEBVTT\n\n")
		for _, seg := range resp.Segments {
			fmt.Fprintf(&b, "%s --> %s\n%s\n\n",
				formatTimestamp(seg.Start, "."),
				formatTimestamp(seg.End, "."),
				strings.TrimSpace(seg.Text))
		}
		return b.String(), contentTypeText, nil

	case "verbose_json":
		payload, err := json.Marshal(resp)
		if err != nil {
			return "", "", fmt.Errorf("marshal verbose_json failed: %w", err)
		}
		return string(payload), contentTypeJSON, nil

	default: // json
		payload, err := json.Marshal(openai.WhisperJSONResponse{Text: resp.Text})
		if err != nil {
			return "", "", fmt.Errorf("marshal json failed: %w", err)
		}
		return string(payload), contentTypeJSON, nil
	}
}
