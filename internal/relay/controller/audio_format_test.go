package controller

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
)

func sampleVerbose() *openai.WhisperVerboseJSONResponse {
	return &openai.WhisperVerboseJSONResponse{
		Task:     "transcribe",
		Language: "chinese",
		Duration: 3.5,
		Text:     "你好 世界",
		Segments: []openai.Segment{
			{Id: 0, Start: 0, End: 1.5, Text: " 你好"},
			{Id: 1, Start: 1.5, End: 3.5, Text: " 世界"},
		},
	}
}

// sampleRaw 是与 sampleVerbose 对应的上游原始响应体。
func sampleRaw(t *testing.T) []byte {
	t.Helper()
	raw, err := json.Marshal(sampleVerbose())
	if err != nil {
		t.Fatalf("构造 rawBody 失败: %v", err)
	}
	return raw
}

func TestConvertVerboseJSONToText(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), sampleRaw(t), "text")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	// OpenAI 官方 text 响应以换行结尾
	if body != "你好 世界\n" {
		t.Errorf("text 格式得到 %q，期望 %q", body, "你好 世界\n")
	}
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q，期望 text/plain", ct)
	}
}

func TestConvertVerboseJSONToTextDoesNotDoubleNewline(t *testing.T) {
	resp := sampleVerbose()
	resp.Text = "已带换行\n"
	body, _, err := convertVerboseJSON(resp, sampleRaw(t), "text")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if body != "已带换行\n" {
		t.Errorf("上游文本已带换行时不应重复追加，得到 %q", body)
	}
}

func TestConvertVerboseJSONToSRT(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), sampleRaw(t), "srt")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	want := "1\n00:00:00,000 --> 00:00:01,500\n你好\n\n2\n00:00:01,500 --> 00:00:03,500\n世界\n\n"
	if body != want {
		t.Errorf("srt 格式得到:\n%q\n期望:\n%q", body, want)
	}
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q，期望 text/plain", ct)
	}
}

func TestConvertVerboseJSONToVTT(t *testing.T) {
	body, _, err := convertVerboseJSON(sampleVerbose(), sampleRaw(t), "vtt")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !strings.HasPrefix(body, "WEBVTT\n\n") {
		t.Errorf("vtt 缺少 WEBVTT 头: %q", body)
	}
	if !strings.Contains(body, "00:00:01.500 --> 00:00:03.500") {
		t.Errorf("vtt 时间戳格式错误（应用点号分隔毫秒）: %q", body)
	}
}

// 不少 whisper 兼容服务只返回 text 而不返回 segments，此时不能给出空字幕。
func TestConvertVerboseJSONSRTFallsBackWhenSegmentsMissing(t *testing.T) {
	resp := &openai.WhisperVerboseJSONResponse{Duration: 2.25, Text: "整段文本"}
	body, _, err := convertVerboseJSON(resp, nil, "srt")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	want := "1\n00:00:00,000 --> 00:00:02,250\n整段文本\n\n"
	if body != want {
		t.Errorf("segments 缺失时 srt 得到:\n%q\n期望:\n%q", body, want)
	}
}

func TestConvertVerboseJSONVTTFallsBackWhenSegmentsMissing(t *testing.T) {
	resp := &openai.WhisperVerboseJSONResponse{Duration: 2.25, Text: "整段文本"}
	body, _, err := convertVerboseJSON(resp, nil, "vtt")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	want := "WEBVTT\n\n00:00:00.000 --> 00:00:02.250\n整段文本\n\n"
	if body != want {
		t.Errorf("segments 缺失时 vtt 得到:\n%q\n期望:\n%q", body, want)
	}
}

// duration 也缺失时降级成 [0, 0] 的单条字幕，仍要带上文本。
func TestConvertVerboseJSONFallbackWithZeroDuration(t *testing.T) {
	resp := &openai.WhisperVerboseJSONResponse{Text: "无时长"}
	body, _, err := convertVerboseJSON(resp, nil, "srt")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	want := "1\n00:00:00,000 --> 00:00:00,000\n无时长\n\n"
	if body != want {
		t.Errorf("duration 缺失时 srt 得到:\n%q\n期望:\n%q", body, want)
	}
}

// segments 与 text 都为空说明上游响应不可用，必须报错而不是给 200 空字幕。
func TestConvertVerboseJSONErrorsWhenSegmentsAndTextEmpty(t *testing.T) {
	for _, format := range []string{"srt", "vtt"} {
		resp := &openai.WhisperVerboseJSONResponse{Duration: 1.0, Text: "   "}
		if _, _, err := convertVerboseJSON(resp, nil, format); err == nil {
			t.Errorf("%s: segments 与 text 都为空时应返回错误", format)
		}
	}
}

func TestConvertVerboseJSONToJSON(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), sampleRaw(t), "json")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if body != `{"text":"你好 世界"}` {
		t.Errorf("json 格式得到 %q", body)
	}
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q，期望 application/json", ct)
	}
}

// text 为空时也必须保留 text 字段，openai-python 的 Transcription 声明它必填。
func TestConvertVerboseJSONToJSONKeepsEmptyText(t *testing.T) {
	resp := &openai.WhisperVerboseJSONResponse{Duration: 1.0}
	body, _, err := convertVerboseJSON(resp, nil, "json")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if body != `{"text":""}` {
		t.Errorf("空转写得到 %q，期望 %q", body, `{"text":""}`)
	}
}

func TestConvertVerboseJSONPassthrough(t *testing.T) {
	raw := sampleRaw(t)
	body, ct, err := convertVerboseJSON(sampleVerbose(), raw, "verbose_json")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if !strings.Contains(body, `"duration":3.5`) {
		t.Errorf("verbose_json 应保留 duration: %q", body)
	}
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q，期望 application/json", ct)
	}
}

// verbose_json 必须原样透传上游 body：反序列化再序列化会因 omitempty 丢掉
// duration:0 / text:"" 等必填字段，也会吃掉 words 这类结构体未声明的扩展字段。
func TestConvertVerboseJSONPassthroughPreservesRawFields(t *testing.T) {
	raw := []byte(`{"task":"transcribe","language":"english","duration":0,"text":"",` +
		`"segments":[],"words":[{"word":"hi","start":0,"end":0.4}],"x_provider":"faster-whisper"}`)
	var parsed openai.WhisperVerboseJSONResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("构造用例失败: %v", err)
	}

	body, _, err := convertVerboseJSON(&parsed, raw, "verbose_json")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if body != string(raw) {
		t.Errorf("verbose_json 应原样透传，得到:\n%q\n期望:\n%q", body, string(raw))
	}
	for _, want := range []string{`"duration":0`, `"text":""`, `"words"`, `"x_provider"`} {
		if !strings.Contains(body, want) {
			t.Errorf("透传结果丢失字段 %s: %q", want, body)
		}
	}
}

func TestIsValidTranscriptionFormat(t *testing.T) {
	for _, valid := range []string{"json", "text", "srt", "verbose_json", "vtt"} {
		if !isValidTranscriptionFormat(valid) {
			t.Errorf("isValidTranscriptionFormat(%q) = false，期望 true", valid)
		}
	}
	for _, invalid := range []string{"", "foo", "JSON", "verbose", "vtt "} {
		if isValidTranscriptionFormat(invalid) {
			t.Errorf("isValidTranscriptionFormat(%q) = true，期望 false", invalid)
		}
	}
}

func TestFormatTimestamp(t *testing.T) {
	cases := []struct {
		in       float64
		sep      string
		expected string
	}{
		{0, ",", "00:00:00,000"},
		{1.5, ",", "00:00:01,500"},
		{61.25, ".", "00:01:01.250"},
		{3661.001, ".", "01:01:01.001"},
		{1.9995, ",", "00:00:02,000"},
	}
	for _, c := range cases {
		if got := formatTimestamp(c.in, c.sep); got != c.expected {
			t.Errorf("formatTimestamp(%v, %q) = %q，期望 %q", c.in, c.sep, got, c.expected)
		}
	}
}
