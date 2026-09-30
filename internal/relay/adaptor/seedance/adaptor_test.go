package seedance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zicorn/llm-proxy/internal/objects"
)

func TestGetRequestURL(t *testing.T) {
	cases := []struct {
		name    string
		baseURL string
		inbound string
		want    string
	}{
		{
			name:    "创建任务",
			baseURL: "https://ark.ap-southeast.bytepluses.com",
			inbound: "/seedance/api/v3/contents/generations/tasks",
			want:    "https://ark.ap-southeast.bytepluses.com/api/v3/contents/generations/tasks",
		},
		{
			name:    "查询任务",
			baseURL: "https://ark.ap-southeast.bytepluses.com",
			inbound: "/seedance/api/v3/contents/generations/tasks/task_abc",
			want:    "https://ark.ap-southeast.bytepluses.com/api/v3/contents/generations/tasks/task_abc",
		},
		{
			// 渠道里把 BaseURL 填成带尾斜杠时不能拼出双斜杠
			name:    "BaseURL 带尾斜杠",
			baseURL: "https://ark.ap-southeast.bytepluses.com/",
			inbound: "/seedance/api/v3/contents/generations/tasks",
			want:    "https://ark.ap-southeast.bytepluses.com/api/v3/contents/generations/tasks",
		},
		{
			// 自建的代理地址（渠道 BaseURL 常被指向反代）
			name:    "自定义 BaseURL",
			baseURL: "http://127.0.0.1:9999",
			inbound: "/seedance/api/v3/contents/generations/tasks",
			want:    "http://127.0.0.1:9999/api/v3/contents/generations/tasks",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := &Adaptor{}
			got, err := a.GetRequestURL(&objects.Meta{
				BaseURL:        tc.baseURL,
				RequestURLPath: tc.inbound,
			})
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// 上游没有 ?model= 这个参数，它只用于本代理选渠道，绝不能出现在上游 URL 里。
func TestGetRequestURLEdropsQueryString(t *testing.T) {
	a := &Adaptor{}
	got, err := a.GetRequestURL(&objects.Meta{
		BaseURL:        "https://ark.ap-southeast.bytepluses.com",
		RequestURLPath: "/seedance/api/v3/contents/generations/tasks?model=doubao-seedance-2-5-pro-250528",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://ark.ap-southeast.bytepluses.com/api/v3/contents/generations/tasks", got)
}

func TestGetRequestURLRejectsUnusableMeta(t *testing.T) {
	a := &Adaptor{}
	_, err := a.GetRequestURL(&objects.Meta{RequestURLPath: "/seedance/x"})
	assert.Error(t, err, "BaseURL 为空时必须报错，而不是发到相对路径")

	_, err = a.GetRequestURL(&objects.Meta{
		BaseURL:        "https://ark.ap-southeast.bytepluses.com",
		RequestURLPath: "/v1/chat/completions",
	})
	assert.Error(t, err, "没有 /seedance 前缀说明路由绑错了")
}
