package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
)

const (
	contentTypeJSON = "application/json; charset=utf-8"
	contentTypeText = "text/plain; charset=utf-8"
)

// validTranscriptionFormats 是转写 / 翻译接口接受的 response_format 取值，
// 与 convertVerboseJSON 的分支一一对应。
var validTranscriptionFormats = map[string]bool{
	"json":         true,
	"text":         true,
	"srt":          true,
	"verbose_json": true,
	"vtt":          true,
}

func isValidTranscriptionFormat(format string) bool {
	return validTranscriptionFormats[format]
}

// transcriptionJSON 是 json 格式的响应体。不复用 openai.WhisperJSONResponse：
// 后者的 text 带 omitempty，空转写会序列化成 {}，而 openai-python 的
// Transcription 把 text 声明为必填，客户端会抛校验错误。
type transcriptionJSON struct {
	Text string `json:"text"`
}

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

// subtitleSegments 返回用于生成字幕的分段。
//
// 很多 whisper 兼容服务（faster-whisper / vLLM-whisper / 自建）只返回 text 而不返回
// segments。若直接按空 segments 渲染，客户端会拿到一个语法合法但空无一物的字幕文件，
// 且因为状态码是 200 而无从察觉。因此降级为一条覆盖 [0, Duration] 的整段字幕；
// 连 text 都没有时说明上游响应不可用，报错而不是给 200。
func subtitleSegments(resp *openai.WhisperVerboseJSONResponse) ([]openai.Segment, error) {
	if len(resp.Segments) > 0 {
		return resp.Segments, nil
	}
	if strings.TrimSpace(resp.Text) == "" {
		return nil, errors.New("upstream response contains neither segments nor text")
	}
	end := resp.Duration
	if end < 0 {
		end = 0
	}
	return []openai.Segment{{Start: 0, End: end, Text: resp.Text}}, nil
}

// convertVerboseJSON 把上游的 verbose_json 响应转换为客户端要求的格式。
//
// 代理固定以 verbose_json 请求上游以获得 duration 用于计费，因此必须在这里
// 降级回客户端原本要求的格式，否则客户端会收到与其请求不符的响应体。
//
// rawBody 是上游未经处理的响应体，供 verbose_json 原样透传使用。
func convertVerboseJSON(resp *openai.WhisperVerboseJSONResponse, rawBody []byte, format string) (string, string, error) {
	switch format {
	case "text":
		// OpenAI 官方 text 响应以换行结尾
		if strings.HasSuffix(resp.Text, "\n") {
			return resp.Text, contentTypeText, nil
		}
		return resp.Text + "\n", contentTypeText, nil

	case "srt":
		segments, err := subtitleSegments(resp)
		if err != nil {
			return "", "", err
		}
		var b strings.Builder
		for i, seg := range segments {
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n",
				i+1,
				formatTimestamp(seg.Start, ","),
				formatTimestamp(seg.End, ","),
				strings.TrimSpace(seg.Text))
		}
		return b.String(), contentTypeText, nil

	case "vtt":
		segments, err := subtitleSegments(resp)
		if err != nil {
			return "", "", err
		}
		var b strings.Builder
		b.WriteString("WEBVTT\n\n")
		for _, seg := range segments {
			fmt.Fprintf(&b, "%s --> %s\n%s\n\n",
				formatTimestamp(seg.Start, "."),
				formatTimestamp(seg.End, "."),
				strings.TrimSpace(seg.Text))
		}
		return b.String(), contentTypeText, nil

	case "verbose_json":
		// 原样透传：反序列化再序列化会因 omitempty 丢掉 duration:0 / text:"" 等
		// 客户端声明为必填的字段，也会吃掉 words 这类结构体未覆盖的扩展字段。
		return string(rawBody), contentTypeJSON, nil

	default: // json
		payload, err := json.Marshal(transcriptionJSON{Text: resp.Text})
		if err != nil {
			return "", "", fmt.Errorf("marshal json failed: %w", err)
		}
		return string(payload), contentTypeJSON, nil
	}
}
