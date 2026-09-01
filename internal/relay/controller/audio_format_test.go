package controller

import (
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

func TestConvertVerboseJSONToText(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), "text")
	if err != nil {
		t.Fatalf("意外错误: %v", err)
	}
	if body != "你好 世界" {
		t.Errorf("text 格式得到 %q，期望 %q", body, "你好 世界")
	}
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q，期望 text/plain", ct)
	}
}

func TestConvertVerboseJSONToSRT(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), "srt")
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
	body, _, err := convertVerboseJSON(sampleVerbose(), "vtt")
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

func TestConvertVerboseJSONToJSON(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), "json")
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

func TestConvertVerboseJSONPassthrough(t *testing.T) {
	body, ct, err := convertVerboseJSON(sampleVerbose(), "verbose_json")
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
