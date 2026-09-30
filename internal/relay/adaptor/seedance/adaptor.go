// Package seedance 适配 BytePlus ModelArk 的 Dreamina Seedance 视频生成 API。
//
// 与其它渠道最大的不同：**请求体不做任何转换**。入站的
// /seedance/api/v3/contents/generations/tasks 与 BytePlus 原生接口同形，
// 所以走 pipeline.ModePassthrough，请求体原样转发、响应体原样回给客户端。
// 客户端只需把 base URL 从 https://ark.ap-southeast.bytepluses.com/api/v3
// 换成 https://<proxy>/seedance/api/v3，其余（路径、请求体、响应）都不用改。
package seedance

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

// PathPrefix 是本代理暴露的路由前缀，同时也是渠道 BaseURL 之外的所有路径的来源。
const PathPrefix = "/seedance"

const channelName = "seedance"

// Adaptor 实现 adaptor.RelayAdaptor。
type Adaptor struct{}

var _ channelhelper.RelayAdaptor = (*Adaptor)(nil)

func (a *Adaptor) Init(meta *objects.Meta) error {
	return nil
}

// GetRequestURL 把入站路径原样接到渠道 BaseURL 后面。
//
//	入站 /seedance/api/v3/contents/generations/tasks
//	上游 {BaseURL}/api/v3/contents/generations/tasks
//
// 查询串整体丢掉：入站的 ?model= 是本代理用来选渠道的（见 inbound.resolveSeedance），
// 上游没有这个参数，带过去没有意义。其余查询参数本渠道当前用不到。
func (a *Adaptor) GetRequestURL(meta *objects.Meta) (string, error) {
	if meta.BaseURL == "" {
		return "", errors.New("seedance: 渠道 BaseURL 为空")
	}
	path := meta.RequestURLPath
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	// 前缀必须显式校验：TrimPrefix 在前缀不存在时是恒等操作，于是 /v1/chat/completions
	// 会被原样拼到 BytePlus 上 —— 请求发得出去、回来一个 404，看不出是路由绑错了。
	if !strings.HasPrefix(path, PathPrefix) {
		return "", fmt.Errorf("seedance: 入站路径 %q 缺少 %s 前缀", meta.RequestURLPath, PathPrefix)
	}
	path = strings.TrimPrefix(path, PathPrefix)
	if path == "" {
		return "", fmt.Errorf("seedance: 入站路径 %q 只有前缀，没有上游路径", meta.RequestURLPath)
	}
	return strings.TrimRight(meta.BaseURL, "/") + path, nil
}

// SetupRequestHeader 设置上游要求的头。BytePlus 用 Bearer API Key 鉴权。
//
// meta.APIKey 是**渠道**的 Key 而不是用户的令牌：Distribute 中间件已经把
// Authorization 换成了渠道 Key，GetRequestMeta 再把它读进 meta。
func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Request, meta *objects.Meta) error {
	channelhelper.SetupCommonRequestHeader(c, req, meta)
	req.Header.Set("Authorization", "Bearer "+meta.APIKey)
	return nil
}

// ConvertRequest 不会被调用：本渠道走透传管线，请求体不做转换。
func (a *Adaptor) ConvertRequest(c *gin.Context, relayMode int, request *entity.GeneralOpenAIRequest) (any, error) {
	return nil, errors.New("seedance: 本渠道走透传管线，不支持请求体转换")
}

// ConvertImageRequest 同 ConvertRequest，不会被调用。
func (a *Adaptor) ConvertImageRequest(request *entity.ImageRequest) (any, error) {
	return nil, errors.New("seedance: 不支持图片生成接口")
}

func (a *Adaptor) DoRequest(c *gin.Context, meta *objects.Meta, requestBody io.Reader) (*http.Response, error) {
	return channelhelper.DoRequestHelper(a, c, meta, requestBody)
}

// DoResponse 原样回传上游响应。
//
// 透传管线自己转发响应体（pipeline.relayResponse），不走这里；本方法只服务于
// 渠道测试等直接调用适配器的路径，因此不解析用量 —— 本渠道的用量结算由
// pipeline.Billing 在提交时刻完成，不依赖响应体。
func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, meta *objects.Meta) (*entity.Usage, string, *objects.ErrorWithStatusCode) {
	if resp == nil {
		return nil, "", objects.ErrorWrapper(errors.New("seedance: 上游响应为空"),
			"upstream_response_nil", http.StatusBadGateway)
	}
	return nil, "", channelhelper.ProxyHandler(c, resp)
}

func (a *Adaptor) GetChannelName() string {
	return channelName
}
