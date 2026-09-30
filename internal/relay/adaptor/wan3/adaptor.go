// Package wan3 适配阿里云百炼 Wan3.0 视频生成 API。
//
// 与 Seedance 同属「异步视频生成」，因此走同一套：入站前缀 + 透传管线 + Billing
// 接管计费。差别都在包内：端点形状（services/aigc/... 与 tasks/{id}）、
// 建单必须带的 X-DashScope-Async 头、以及**计费单位是秒而不是 token**。
package wan3

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	channelhelper "github.com/zicorn/llm-proxy/internal/relay/adaptor"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
)

// PathPrefix 是本代理暴露的路由前缀。客户端把 base URL 从
// https://{WorkspaceId}.{region}.maas.aliyuncs.com 换成 https://<proxy>/wan3 即可。
const PathPrefix = "/wan3"

// CreatePathSuffix 是建单路径的结尾段。
//
// 导出是因为两处都要认它：本包用它决定要不要加异步头，inbound 用它决定这次
// 请求要不要计费。各写一份字面量，改一处漏一处时的症状分别是"建单缺头直接报错"
// 与**"建单被当成只读请求，静默不计费"** —— 后者不报错，只是不扣钱。
const CreatePathSuffix = "/services/aigc/video-generation/video-synthesis"

const channelName = "wan3"

// Adaptor 实现 adaptor.RelayAdaptor。
type Adaptor struct{}

var _ channelhelper.RelayAdaptor = (*Adaptor)(nil)

func (a *Adaptor) Init(meta *objects.Meta) error {
	return nil
}

// GetRequestURL 把入站路径原样接到渠道 BaseURL 后面。
//
//	入站 /wan3/api/v1/services/aigc/video-generation/video-synthesis
//	上游 {BaseURL}/api/v1/services/aigc/video-generation/video-synthesis
//
// 查询串整体丢掉：入站的 ?model= 是本代理用来选渠道的（见 inbound.resolveWan3），
// 百炼没有这个参数。
func (a *Adaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	if meta.BaseURL == "" {
		// 百炼端点是按 Workspace 与 Region 拼的，没有通用默认值。宁可在这里
		// 报一句能看懂的错，也不要把请求发到一个猜出来的域名上。
		return "", errors.New("wan3: 渠道未配置 BaseURL（形如 https://{WorkspaceId}.{region}.maas.aliyuncs.com）")
	}
	path := meta.RequestURLPath
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	// 前缀必须显式校验：TrimPrefix 在前缀不存在时是恒等操作，于是别的路径会被
	// 原样拼到百炼上 —— 请求发得出去、回来一个 404，看不出是路由绑错了。
	if !strings.HasPrefix(path, PathPrefix) {
		return "", fmt.Errorf("wan3: 入站路径 %q 缺少 %s 前缀", meta.RequestURLPath, PathPrefix)
	}
	path = strings.TrimPrefix(path, PathPrefix)
	if path == "" {
		return "", fmt.Errorf("wan3: 入站路径 %q 只有前缀，没有上游路径", meta.RequestURLPath)
	}
	return strings.TrimRight(meta.BaseURL, "/") + path, nil
}

// SetupRequestHeader 设置上游要求的头。
//
// X-DashScope-Async 只在**建单**时加：缺了它建单会报
// "current user api does not support synchronous calls"；而查询接口不需要它，
// 无差别地加上只会让查询带上一个没意义的头。
func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	channelhelper.SetupCommonRequestHeader(c, req, meta)
	req.Header.Set("Authorization", "Bearer "+meta.APIKey)
	if IsCreatePath(c.Request.URL.Path) {
		req.Header.Set("X-DashScope-Async", asyncEnable)
	}
	return nil
}

// IsCreatePath 判断入站路径是不是建单。
func IsCreatePath(path string) bool {
	return strings.HasSuffix(strings.TrimRight(path, "/"), CreatePathSuffix)
}

// ConvertRequest 不会被调用：本渠道走透传管线，请求体不做转换。
func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("wan3: 本渠道走透传管线，不支持请求体转换")
}

// ConvertImageRequest 同 ConvertRequest，不会被调用。
func (a *Adaptor) ConvertImageRequest(request *entity.ImageRequest) (any, error) {
	return nil, errors.New("wan3: 不支持图片生成接口")
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error) {
	return channelhelper.DoRequestHelper(a, c, meta, requestBody)
}

// DoResponse 原样回传上游响应。
//
// 透传管线自己转发响应体（pipeline.relayResponse），不走这里；本方法只服务于
// 渠道测试等直接调用适配器的路径。计费由 pipeline.Billing 在建单时刻完成，
// 不依赖响应体。
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (*entity.Usage, string, *objects.ErrorWithStatusCode) {
	if resp == nil {
		return nil, "", objects.ErrorWrapper(errors.New("wan3: 上游响应为空"),
			"upstream_response_nil", http.StatusBadGateway)
	}
	return nil, "", channelhelper.ProxyHandler(c, resp)
}

func (a *Adaptor) GetChannelName() string {
	return channelName
}
