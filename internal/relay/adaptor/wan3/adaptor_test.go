package wan3

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zicorn/llm-proxy/internal/objects"
)

const testBase = "https://ws-abc123.cn-beijing.maas.aliyuncs.com"

func TestGetRequestURL(t *testing.T) {
	cases := []struct {
		name    string
		inbound string
		want    string
	}{
		{
			name:    "建单",
			inbound: "/wan3/api/v1/services/aigc/video-generation/video-synthesis",
			want:    testBase + "/api/v1/services/aigc/video-generation/video-synthesis",
		},
		{
			name:    "查询任务",
			inbound: "/wan3/api/v1/tasks/0385dc79-5ff8-4d82-bcb6",
			want:    testBase + "/api/v1/tasks/0385dc79-5ff8-4d82-bcb6",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &Adaptor{}
			got, err := a.GetRequestURL(&objects.Meta{BaseURL: testBase, RequestURLPath: tc.inbound})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestGetRequestURLDropsQueryString(t *testing.T) {
	a := &Adaptor{}
	got, err := a.GetRequestURL(&objects.Meta{
		BaseURL:        testBase + "/",
		RequestURLPath: "/wan3/api/v1/tasks/abc?model=wan3.0-video",
	})
	require.NoError(t, err)
	assert.Equal(t, testBase+"/api/v1/tasks/abc", got)
}

// 百炼端点是按 Workspace + Region 拼的，没有通用默认值。没配就得报错，
// 而不是把请求发到一个猜出来的域名上。
func TestGetRequestURLRequiresBaseURL(t *testing.T) {
	a := &Adaptor{}
	_, err := a.GetRequestURL(&objects.Meta{RequestURLPath: "/wan3/api/v1/tasks/abc"})
	assert.Error(t, err)
}

func TestGetRequestURLRejectsMissingPrefix(t *testing.T) {
	a := &Adaptor{}
	_, err := a.GetRequestURL(&objects.Meta{
		BaseURL:        testBase,
		RequestURLPath: "/v1/chat/completions",
	})
	assert.Error(t, err, "没有 /wan3 前缀说明路由绑错了")
}

// X-DashScope-Async 只在建单时加：查询带上它没有意义，
// 而建单缺了它会直接报 "does not support synchronous calls"。
func TestSetupRequestHeaderAddsAsyncOnlyForCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		path      string
		wantAsync bool
	}{
		{"/wan3/api/v1/services/aigc/video-generation/video-synthesis", true},
		{"/wan3/api/v1/tasks/abc", false},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, tc.path, nil)
			c.Request.Header.Set("Content-Type", "application/json")

			req := httptest.NewRequest(http.MethodPost, "https://upstream/x", nil)
			a := &Adaptor{}
			require.NoError(t, a.SetupRequestHeader(c, req, &objects.Meta{APIKey: "dashscope-key"}))

			assert.Equal(t, "Bearer dashscope-key", req.Header.Get("Authorization"))
			if tc.wantAsync {
				assert.Equal(t, "enable", req.Header.Get("X-DashScope-Async"))
			} else {
				assert.Empty(t, req.Header.Get("X-DashScope-Async"))
			}
		})
	}
}
