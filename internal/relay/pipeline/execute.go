package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay"
	"github.com/zicorn/llm-proxy/internal/relay/entity"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat"
	"github.com/zicorn/llm-proxy/internal/relay/wireformat/usage"
	model "github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/pkg/common"
	"github.com/zicorn/llm-proxy/pkg/common/client"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
)

// Execute 执行一次转发并强制结算。
//
// 计费写在这里而不是各 Controller 里：新增一条转发路由必须经由 Handler 绑定
// spec，绑不上服务就起不来，因此"忘记接计费"从运行期问题变成了启动期问题。
func Execute(c *gin.Context, spec *RelaySpec) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)

	op, err := spec.Resolve(c)
	if err != nil {
		return objects.ErrorWrapper(err, "resolve_request_failed", http.StatusBadRequest)
	}

	meta.OriginModelName = op.Model
	meta.ActualModelName, _ = objects.ResolveModelName(op.Model, meta.ModelMapping)
	meta.IsStream = op.IsStream

	if op.Kind == KindUnsupported {
		return objects.ErrorWrapper(
			fmt.Errorf("原生透传暂不支持 %s 操作，请改用 OpenAI 兼容接口，否则无法正确计费", op.Action),
			"unsupported_native_operation", http.StatusBadRequest)
	}

	upstreamWire := wireformat.Resolve(meta.ChannelType, meta.ActualModelName)

	if spec.Mode == ModePassthrough && op.Billable() &&
		op.InboundWire != wireformat.Unspecified && op.InboundWire != upstreamWire {
		return objects.ErrorWrapper(
			fmt.Errorf("该接口需要 %s 格式的渠道，当前渠道是 %s 格式，请求体无法被上游理解",
				op.InboundWire, upstreamWire),
			"inbound_channel_mismatch", http.StatusBadRequest)
	}

	var preConsumed int64
	if op.Billable() {
		if meta.ActualModelName == "" {
			return objects.ErrorWrapper(
				fmt.Errorf("无法从请求中识别模型名，拒绝转发以避免漏计费"),
				"model_name_unresolved", http.StatusBadRequest)
		}
		// 模型未配置定价即拒绝：宁可拒绝也不放行一笔无法计费的请求
		if _, err := model.GetModelMetaByModel(meta.ActualModelName); err != nil {
			return objects.ErrorWrapper(
				fmt.Errorf("模型 %s 未配置，请联系管理员在模型管理中添加", meta.ActualModelName),
				"model_not_configured", http.StatusBadRequest)
		}
		var bizErr *objects.ErrorWithStatusCode
		preConsumed, bizErr = objects.PreConsumeQuotaByTokens(ctx, 0, meta)
		if bizErr != nil {
			return bizErr
		}
	}

	if spec.Mode == ModeNormalize {
		return executeNormalize(c, meta, op, preConsumed)
	}
	return executePassthrough(c, spec, meta, op, preConsumed, upstreamWire)
}

func executePassthrough(c *gin.Context, spec *RelaySpec, meta *objects.Meta, op *Operation,
	preConsumed int64, upstreamWire wireformat.Format) *objects.ErrorWithStatusCode {

	ctx := c.Request.Context()

	resp, bizErr := sendUpstream(c, spec, meta, op)
	if bizErr != nil {
		refund(ctx, meta, preConsumed)
		return bizErr
	}
	defer resp.Body.Close()

	body, copyErr := relayResponse(c, resp)
	if copyErr != nil {
		logger.Warnf(ctx, "[pipeline] 转发响应体失败: %v", copyErr)
	}

	if resp.StatusCode != http.StatusOK {
		refund(ctx, meta, preConsumed)
		return nil
	}
	if !op.Billable() {
		return nil
	}

	settle(ctx, meta, op, preConsumed, body, upstreamWire)
	return nil
}

// sendUpstream 把请求发往上游。
//
// KindGenerate 委托渠道适配器构造 URL 与鉴权头，这样 Vertex AI 这类路径含
// project/location、鉴权需 OAuth token 的渠道也能正确寻址；被删掉的 native.go
// 自行拼接 baseURL+path，在 Vertex 渠道下发到的是不存在的地址。
func sendUpstream(c *gin.Context, spec *RelaySpec, meta *objects.Meta,
	op *Operation) (*http.Response, *objects.ErrorWithStatusCode) {

	reqBody, err := requestBodyReader(c)
	if err != nil {
		return nil, objects.ErrorWrapper(err, "read_request_body_failed", http.StatusBadRequest)
	}

	if op.Kind == KindMetadata {
		return sendRawUpstream(c, spec, meta, reqBody)
	}

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return nil, objects.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType),
			"invalid_api_type", http.StatusBadRequest)
	}
	if err := adaptor.Init(meta); err != nil {
		return nil, objects.ErrorWrapper(err, "init_adaptor_failed", http.StatusInternalServerError)
	}
	resp, err := adaptor.DoRequest(c, meta, reqBody)
	if err != nil {
		return nil, objects.ErrorWrapper(err, "do_request_failed", http.StatusBadGateway)
	}
	return resp, nil
}

// sendRawUpstream 按「渠道 BaseURL + 入站路径」直接转发，仅用于不计费的元数据操作。
// 鉴权头仍由渠道适配器设置，避免在此处重复实现各渠道的鉴权方式。
func sendRawUpstream(c *gin.Context, spec *RelaySpec, meta *objects.Meta,
	reqBody io.Reader) (*http.Response, *objects.ErrorWithStatusCode) {

	upstreamPath := strings.TrimPrefix(c.Request.URL.Path, spec.PathPrefix)
	if upstreamPath == "" {
		upstreamPath = "/"
	}
	url := strings.TrimRight(meta.BaseURL, "/") + upstreamPath
	if c.Request.URL.RawQuery != "" {
		url += "?" + c.Request.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, url, reqBody)
	if err != nil {
		return nil, objects.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return nil, objects.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType),
			"invalid_api_type", http.StatusBadRequest)
	}
	if err := adaptor.SetupRequestHeader(c, req, meta); err != nil {
		return nil, objects.ErrorWrapper(err, "setup_request_header_failed", http.StatusInternalServerError)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, objects.ErrorWrapper(err, "do_request_failed", http.StatusBadGateway)
	}
	return resp, nil
}

// requestBodyReader 返回可重复读取的请求体。
// 中间件（TokenAuth 解析模型名）已消费过原始 Body，必须走缓存副本。
func requestBodyReader(c *gin.Context) (io.Reader, error) {
	raw, err := common.GetRequestBody(c)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(raw), nil
}

// relayResponse 把上游响应转发给客户端，同时返回响应体副本用于解析用量。
// 每写一段就 Flush，否则 SSE 会被缓冲住直到响应结束。
func relayResponse(c *gin.Context, resp *http.Response) ([]byte, error) {
	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Writer.WriteHeader(resp.StatusCode)

	var buf bytes.Buffer
	flusher, canFlush := c.Writer.(http.Flusher)
	chunk := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(chunk)
		if n > 0 {
			buf.Write(chunk[:n])
			if _, writeErr := c.Writer.Write(chunk[:n]); writeErr != nil {
				return buf.Bytes(), writeErr
			}
			if canFlush {
				flusher.Flush()
			}
		}
		if readErr == io.EOF {
			return buf.Bytes(), nil
		}
		if readErr != nil {
			return buf.Bytes(), readErr
		}
	}
}

// settle 提取用量并结算。
//
// 解析失败时传零值 usage：PostConsumeQuota 会按 0 结算并把预扣退回，
// 同时写一条 error 级日志，便于发现需要补适配的响应格式。
func settle(ctx context.Context, meta *objects.Meta, op *Operation,
	preConsumed int64, body []byte, wire wireformat.Format) {

	var u *entity.Usage
	if extractor := usage.Get(wire); extractor == nil {
		logger.Errorf(ctx, "[pipeline] 用量解析异常：wire 格式 %s 无提取器，模型 %s 按 0 结算，需补适配",
			wire, meta.ActualModelName)
	} else if parsed, ok := extractor(body, op.IsStream); ok {
		u = parsed
	} else {
		logger.Errorf(ctx, "[pipeline] 用量解析异常：%s 格式响应未解析出 usage，模型 %s 按 0 结算，需补适配",
			wire, meta.ActualModelName)
	}
	if u == nil {
		u = &entity.Usage{}
	}
	objects.PostConsumeQuota(ctx, u, meta, preConsumed)
}

func refund(ctx context.Context, meta *objects.Meta, preConsumed int64) {
	if preConsumed <= 0 {
		return
	}
	if err := model.PostConsumeTokenQuota(meta.TokenId, -preConsumed); err != nil {
		logger.Errorf(ctx, "[pipeline] 回滚预扣额度失败: %v", err)
	}
}
