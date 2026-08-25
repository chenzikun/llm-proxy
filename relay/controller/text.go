package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/common/mq"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/apitype"
	"github.com/songquanpeng/one-api/relay/billing"
	"github.com/songquanpeng/one-api/relay/channeltype"
	"github.com/songquanpeng/one-api/relay/entity"
)

func RelayTextHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)
	// 验证 X-Session-ID 格式（必须是 UUID v4）
	if meta.SessionId != "" && !common.IsValidUUIDv4(meta.SessionId) {
		return objects.ErrorWrapper(fmt.Errorf("invalid sessionId: %s, must be UUID v4 format", meta.SessionId), "invalid_session_id", http.StatusBadRequest)
	}
	// get & validate textRequest
	textRequest, err := getAndValidateTextRequest(c, meta.Mode)
	if err != nil {
		logger.Errorf(ctx, "getAndValidateTextRequest failed: %s", err.Error())
		return objects.ErrorWrapper(err, "invalid_text_request", http.StatusBadRequest)
	}
	meta.IsStream = textRequest.Stream

	// map model name
	meta.OriginModelName = textRequest.Model
	textRequest.Model, _ = getMappedModelName(textRequest.Model, meta.ModelMapping)
	meta.ActualModelName = textRequest.Model
	// 预先扣除费用
	preConsumedQuota, bizErr := objects.PreConsumeQuota(ctx, textRequest, meta)
	if bizErr != nil {
		logger.Warnf(ctx, "preConsumeQuota failed: %+v", *bizErr)
		return bizErr
	}

	// 获取每个渠道对应的适配器
	adaptor_ := relay.GetAdaptor(meta.APIType)
	if adaptor_ == nil {
		return objects.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType), "invalid_api_type", http.StatusBadRequest)
	}
	if err := adaptor_.Init(meta); err != nil {
		logger.Errorf(ctx, "Init failed: %s", err.Error())
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		return objects.ErrorWrapper(err, "init_failed", http.StatusInternalServerError)
	}

	// 创建每个渠道对应的 request 对象
	requestBody, err := getTextRequestBody(c, meta, textRequest, adaptor_)
	if err != nil {
		return objects.ErrorWrapper(err, "convert_request_failed", http.StatusInternalServerError)
	}

	// do request
	resp, err := adaptor_.DoRequest(c, meta, requestBody)
	if err != nil {
		logger.Errorf(ctx, "DoRequest failed: %s", err.Error())
		return objects.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}
	if isErrorHappened(meta, resp) {
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		return RelayErrorHandler(resp)
	}

	// do response
	usage, responseText, respErr := adaptor_.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "respErr is not nil: %+v", respErr)
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		return respErr
	}

	// 数据回流
	if len(textRequest.Messages) > 0 {
		logger.Infof(ctx, "数据回流 sessionId: %s", meta.SessionId)
		var messages []entity.Message
		for i := len(textRequest.Messages) - 1; i >= 0; i-- {
			message := textRequest.Messages[i]
			if message.Role == "assistant" {
				break
			}
			messages = append([]entity.Message{
				{
					Role:    message.Role,
					Content: message.Content,
				},
			}, messages...)
		}

		msg := entity.AutelMessage{
			ID:        strings.ReplaceAll(uuid.New().String(), "-", ""),
			SessionID: meta.SessionId,
			TokenName: meta.TokenName,
			Messages:  messages,
			Model:     meta.OriginModelName,
			CreatedAt: time.Now().Unix(),
		}
		msg.Messages = append(msg.Messages, entity.Message{
			Role:    "assistant",
			Content: responseText,
		})
		data, err := json.Marshal(msg)
		if err == nil {
			go mq.Push(ctx, data)
		} else {
			logger.Errorf(ctx, "Failed to marshal request.Messages: %s", err.Error())
		}
	}

	// post-consume quota
	go objects.PostConsumeQuota(ctx, usage, meta, preConsumedQuota)
	return nil
}

// 创建每个渠道对应的 request 对象
func getTextRequestBody(c *gin.Context, meta *objects.Meta, textRequest *entity.GeneralOpenAIRequest, adaptor_ adaptor.RelayAdaptor) (io.Reader, error) {
	if meta.APIType == apitype.OpenAI &&
		meta.OriginModelName == meta.ActualModelName &&
		meta.ChannelType != channeltype.Baichuan {
		// no need to convert request for openai
		return c.Request.Body, nil
	}

	// get request body
	var requestBody io.Reader
	convertedRequest, err := adaptor_.ConvertRequest(c, meta.Mode, textRequest)
	if err != nil {
		logger.Debugf(c.Request.Context(), "converted request failed: %s\n", err.Error())
		return nil, err
	}
	jsonData, err := json.Marshal(convertedRequest)
	if err != nil {
		logger.Debugf(c.Request.Context(), "converted request json_marshal_failed: %s\n", err.Error())
		return nil, err
	}
	logger.Debugf(c.Request.Context(), "converted request: \n%s", string(jsonData))
	requestBody = bytes.NewBuffer(jsonData)
	return requestBody, nil
}
