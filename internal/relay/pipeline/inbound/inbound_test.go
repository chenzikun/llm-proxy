package inbound

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/zicorn/llm-proxy/internal/relay/pipeline"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
)

func ctxWithRequest(method, path, body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c
}

func TestResolveGemini(t *testing.T) {
	cases := []struct {
		path       string
		wantModel  string
		wantKind   pipeline.Kind
		wantStream bool
	}{
		{"/gemini/v1beta/models/gemini-2.5-flash:generateContent",
			"gemini-2.5-flash", pipeline.KindGenerate, false},
		{"/gemini/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse",
			"gemini-2.5-flash", pipeline.KindGenerate, true},
		{"/gemini/v1beta/models/gemini-2.5-flash:countTokens",
			"gemini-2.5-flash", pipeline.KindMetadata, false},
		{"/gemini/v1beta/models/text-embedding-004:embedContent",
			"text-embedding-004", pipeline.KindUnsupported, false},
		{"/gemini/v1beta/models", "", pipeline.KindMetadata, false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			op, err := resolveGemini(ctxWithRequest(http.MethodPost, tc.path, ""))
			assert.NoError(t, err)
			assert.Equal(t, tc.wantModel, op.Model)
			assert.Equal(t, tc.wantKind, op.Kind)
			assert.Equal(t, tc.wantStream, op.IsStream)
			assert.Equal(t, wireformat.Gemini, op.InboundWire)
		})
	}
}

func TestResolveGeminiBillableOnlyForGeneration(t *testing.T) {
	op, _ := resolveGemini(ctxWithRequest(http.MethodPost,
		"/gemini/v1beta/models/gemini-2.5-flash:generateContent", ""))
	assert.True(t, op.Billable())

	op, _ = resolveGemini(ctxWithRequest(http.MethodPost,
		"/gemini/v1beta/models/gemini-2.5-flash:countTokens", ""))
	assert.False(t, op.Billable())
}

func TestResolveVertexAIWireFormatFollowsModel(t *testing.T) {
	op, err := resolveVertexAI(ctxWithRequest(http.MethodPost,
		"/vertexai/v1/models/gemini-2.5-flash:generateContent", ""))
	assert.NoError(t, err)
	assert.Equal(t, wireformat.Gemini, op.InboundWire)
	assert.Equal(t, pipeline.KindGenerate, op.Kind)

	op, err = resolveVertexAI(ctxWithRequest(http.MethodPost,
		"/vertexai/v1/models/claude-sonnet-4:rawPredict", ""))
	assert.NoError(t, err)
	assert.Equal(t, wireformat.Anthropic, op.InboundWire,
		"Vertex 上的 Claude 请求体是 Anthropic 格式")
	assert.Equal(t, pipeline.KindGenerate, op.Kind)

	op, _ = resolveVertexAI(ctxWithRequest(http.MethodPost,
		"/vertexai/v1/models/claude-sonnet-4:streamRawPredict", ""))
	assert.True(t, op.IsStream)
}

func TestResolveAnthropic(t *testing.T) {
	op, err := resolveAnthropic(ctxWithRequest(http.MethodPost, "/anthropic/v1/messages",
		`{"model":"claude-sonnet-4","stream":false,"messages":[]}`))
	assert.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4", op.Model)
	assert.Equal(t, pipeline.KindGenerate, op.Kind)
	assert.False(t, op.IsStream)
	assert.Equal(t, wireformat.Anthropic, op.InboundWire)

	op, _ = resolveAnthropic(ctxWithRequest(http.MethodPost, "/anthropic/v1/messages",
		`{"model":"claude-sonnet-4","stream":true,"messages":[]}`))
	assert.True(t, op.IsStream)
}

func TestResolveAnthropicCountTokensNotBillable(t *testing.T) {
	op, err := resolveAnthropic(ctxWithRequest(http.MethodPost,
		"/anthropic/v1/messages/count_tokens", `{"model":"claude-sonnet-4"}`))
	assert.NoError(t, err)
	assert.False(t, op.Billable())
}

// 透传必须尊重客户端在路径上选的 API 版本：渠道默认是 v1，而原生 SDK 打 v1beta，
// 用默认值覆盖会让新模型的特性不可用。
func TestInboundAPIVersionPreserved(t *testing.T) {
	op, _ := resolveGemini(ctxWithRequest(http.MethodPost,
		"/gemini/v1beta/models/gemini-2.5-flash:generateContent", ""))
	assert.Equal(t, "v1beta", op.APIVersion)

	op, _ = resolveGemini(ctxWithRequest(http.MethodPost,
		"/gemini/v1/models/gemini-2.5-flash:generateContent", ""))
	assert.Equal(t, "v1", op.APIVersion)

	op, _ = resolveVertexAI(ctxWithRequest(http.MethodPost,
		"/vertexai/v1beta1/models/gemini-2.5-flash:generateContent", ""))
	assert.Equal(t, "v1beta1", op.APIVersion)

	// 路径里没有版本段时留空，由渠道配置决定
	op, _ = resolveGemini(ctxWithRequest(http.MethodPost, "/gemini/models", ""))
	assert.Equal(t, "", op.APIVersion)
}

// 四份 spec 必须都在 init 中注册，否则路由绑定时服务会 panic。
func TestSpecsRegistered(t *testing.T) {
	for _, name := range []string{
		"gemini.native", "anthropic.native", "vertexai.native", "seedance.native",
	} {
		_, ok := pipeline.Lookup(name)
		assert.True(t, ok, "spec %s 未注册", name)
	}
}

func TestResolveSeedance(t *testing.T) {
	cases := []struct {
		method   string
		path     string
		wantKind pipeline.Kind
	}{
		// 创建任务计费
		{http.MethodPost, "/seedance/api/v3/contents/generations/tasks", pipeline.KindGenerate},
		// 版本段变化不能让它掉进不计费的分支
		{http.MethodPost, "/seedance/v3/contents/generations/tasks", pipeline.KindGenerate},
		{http.MethodPost, "/seedance/api/v3/contents/generations/tasks/", pipeline.KindGenerate},
		// 查询任务、查询列表都不产生新用量
		{http.MethodGet, "/seedance/api/v3/contents/generations/tasks/task_abc", pipeline.KindMetadata},
		{http.MethodGet, "/seedance/api/v3/contents/generations/tasks", pipeline.KindMetadata},
		// 不带 /contents/generations/tasks 的路径（KYC 资产库那批）一律透传不计费
		{http.MethodPost, "/seedance/CreateAsset", pipeline.KindMetadata},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			op, err := resolveSeedance(ctxWithRequest(tc.method, tc.path, ""))
			assert.NoError(t, err)
			assert.Equal(t, tc.wantKind, op.Kind)
			assert.Equal(t, wireformat.Unspecified, op.InboundWire,
				"入站请求体就是上游格式，不做 wire 兼容校验")
		})
	}
}

// 查询串上的 ?model= 只用于本代理选渠道，转发前必须摘掉，否则会一路发到 BytePlus。
func TestResolveSeedanceStripsModelQueryParam(t *testing.T) {
	c := ctxWithRequest(http.MethodGet,
		"/seedance/api/v3/contents/generations/tasks/task_abc?model=doubao-seedance-2-5-pro-250528&page_size=1", "")
	op, err := resolveSeedance(c)
	assert.NoError(t, err)
	assert.Equal(t, pipeline.KindMetadata, op.Kind)
	assert.Equal(t, "page_size=1", c.Request.URL.RawQuery)
}

// input_video_duration 是可选参数：不传就退回标准行为，不该因此报错。
func TestResolveSeedanceInputVideoDurationOptional(t *testing.T) {
	c := ctxWithRequest(http.MethodPost, "/seedance/api/v3/contents/generations/tasks", `{}`)
	op, err := resolveSeedance(c)
	assert.NoError(t, err)
	assert.Equal(t, pipeline.KindGenerate, op.Kind)
	assert.Equal(t, 0.0, seedanceInputVideoSeconds(c), "不传时按 0（未声明）")
}

// 声明了就要能读到，并且从查询串里摘掉 —— 上游不认识这个参数。
func TestResolveSeedanceReadsAndStripsInputVideoDuration(t *testing.T) {
	c := ctxWithRequest(http.MethodPost,
		"/seedance/api/v3/contents/generations/tasks?input_video_duration=5.5&model=m", `{}`)
	_, err := resolveSeedance(c)
	assert.NoError(t, err)
	assert.Equal(t, 5.5, seedanceInputVideoSeconds(c))
	assert.Equal(t, "", c.Request.URL.RawQuery, "两个代理参数都要摘掉")
}

// 填错的时长必须报错：静默忽略等于按错误口径扣钱，账单上看不出来。
func TestResolveSeedanceRejectsBadInputVideoDuration(t *testing.T) {
	for _, raw := range []string{"abc", "-1", "NaN", "601"} {
		c := ctxWithRequest(http.MethodPost,
			"/seedance/api/v3/contents/generations/tasks?input_video_duration="+raw, `{}`)
		_, err := resolveSeedance(c)
		assert.Error(t, err, "input_video_duration=%s 应被拒绝", raw)
	}
}

// 上限要放得过正常值：10 分钟是 Seedance 出片上限（30s）的 20 倍，够用。
func TestResolveSeedanceAcceptsInputDurationAtCap(t *testing.T) {
	c := ctxWithRequest(http.MethodPost,
		"/seedance/api/v3/contents/generations/tasks?input_video_duration=600", `{}`)
	_, err := resolveSeedance(c)
	assert.NoError(t, err)
	assert.Equal(t, 600.0, seedanceInputVideoSeconds(c))
}

// 创建任务不带 model 时由 nativeformat 从请求体取，不依赖查询串。
func TestResolveSeedanceKeepsBodyModel(t *testing.T) {
	c := ctxWithRequest(http.MethodPost,
		"/seedance/api/v3/contents/generations/tasks", `{"model":"doubao-seedance-2-5-pro-250528"}`)
	c.Set(ctxkey.RequestModel, "doubao-seedance-2-5-pro-250528")
	op, err := resolveSeedance(c)
	assert.NoError(t, err)
	assert.Equal(t, "doubao-seedance-2-5-pro-250528", op.Model)
	assert.Equal(t, pipeline.KindGenerate, op.Kind)
}
