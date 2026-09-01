package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common/client"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
	"github.com/zicorn/llm-proxy/internal/relay/relaymode"
)

func RelayAudioHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := objects.GetRequestMeta(c)
	// 计费用 ActualModelName 查 model_meta，发给上游的 model 字段必须是同一个值，
	// 否则会出现"按 A 计费、实际调用 B"。GetRequestMeta 不设 ActualModelName，需在此补。
	meta.ActualModelName, _ = objects.ResolveModelName(meta.OriginModelName, meta.ModelMapping)
	audioModel := meta.ActualModelName

	tokenId := c.GetInt(ctxkey.TokenId)
	channelType := c.GetInt(ctxkey.ChannelType)

	modelMeta, err := model.GetModelMetaByModel(meta.ActualModelName)
	if err != nil {
		return objects.ErrorWrapper(
			fmt.Errorf("模型 %s 未配置，请联系管理员在模型管理中添加", meta.ActualModelName),
			"model_not_configured", http.StatusBadRequest)
	}
	// 价格字段恒为"每 100 万个计量单位的价格"，音频按秒计量。billing_unit 的数据库默认值是
	// token，若管理员建模型时漏选 second，计费会拿按 token 定价的数字去乘秒数，静默算错金额。
	if modelMeta.BillingUnit != model.BillingUnitSecond {
		return objects.ErrorWrapper(
			fmt.Errorf("模型 %s 的计量单位为 %s，音频转写/翻译仅支持 second",
				meta.ActualModelName, modelMeta.BillingUnit),
			"billing_unit_mismatch", http.StatusBadRequest)
	}

	preConsumedQuota, bizErr := objects.PreConsumeTranscriptionQuota(ctx, meta)
	if bizErr != nil {
		return bizErr
	}

	succeed := false
	defer func() {
		if succeed {
			return
		}
		if preConsumedQuota > 0 {
			// we need to roll back the pre-consumed quota
			defer func(ctx context.Context) {
				go func() {
					// negative means add quota back for token & user
					err := model.PostConsumeTokenQuota(tokenId, -preConsumedQuota)
					if err != nil {
						logger.Error(ctx, fmt.Sprintf("error rollback pre-consumed quota: %s", err.Error()))
					}
				}()
			}(c.Request.Context())
		}
	}()

	baseURL := channeltype.ChannelBaseURLs[channelType]
	requestURL := c.Request.URL.String()
	if c.GetString(ctxkey.BaseURL) != "" {
		baseURL = c.GetString(ctxkey.BaseURL)
	}

	fullRequestURL := openai.GetFullRequestURL(baseURL, requestURL, channelType)
	if channelType == channeltype.Azure {
		apiVersion := meta.Config.APIVersion
		if relayMode == relaymode.AudioTranscription {
			// https://learn.microsoft.com/en-us/azure/ai-services/openai/whisper-quickstart?tabs=command-line#rest-api
			fullRequestURL = fmt.Sprintf("%s/openai/deployments/%s/audio/transcriptions?api-version=%s", baseURL, audioModel, apiVersion)
		} else if relayMode == relaymode.AudioSpeech {
			// https://learn.microsoft.com/en-us/azure/ai-services/openai/text-to-speech-quickstart?tabs=command-line#rest-api
			fullRequestURL = fmt.Sprintf("%s/openai/deployments/%s/audio/speech?api-version=%s", baseURL, audioModel, apiVersion)
		}
	}

	requestBody := &bytes.Buffer{}

	if relayMode == relaymode.AudioTranscription || relayMode == relaymode.AudioTranslation {
		file, err := c.FormFile("file")
		if err != nil {
			return objects.ErrorWrapper(err, "get_form_file_failed", http.StatusBadRequest)
		}
		fileContent, err := file.Open()
		if err != nil {
			return objects.ErrorWrapper(err, "open_file_failed", http.StatusInternalServerError)
		}
		defer fileContent.Close()

		// 创建一个新的multipart writer
		writer := multipart.NewWriter(requestBody)

		// 添加文件
		part, err := writer.CreateFormFile("file", file.Filename)
		if err != nil {
			return objects.ErrorWrapper(err, "create_form_file_failed", http.StatusInternalServerError)
		}

		// 复制文件内容
		_, err = io.Copy(part, fileContent)
		if err != nil {
			return objects.ErrorWrapper(err, "copy_file_content_failed", http.StatusInternalServerError)
		}

		// 添加其他表单字段
		err = writer.WriteField("model", audioModel)
		if err != nil {
			return objects.ErrorWrapper(err, "write_field_failed", http.StatusInternalServerError)
		}

		// 固定以 verbose_json 请求上游：只有该格式的响应带 duration 字段，
		// 而按秒计费必须知道时长。客户端要的格式在响应阶段再转换。
		err = writer.WriteField("response_format", "verbose_json")
		if err != nil {
			return objects.ErrorWrapper(err, "write_field_failed", http.StatusInternalServerError)
		}

		// 这些字段此前被整体丢弃，导致 language 等参数失效、识别质量下降
		fields := []string{"prompt", "temperature"}
		if relayMode == relaymode.AudioTranscription {
			// 翻译端点固定输出英文，不接受 language
			fields = append(fields, "language")
		}
		for _, field := range fields {
			if v := c.PostForm(field); v != "" {
				if err := writer.WriteField(field, v); err != nil {
					return objects.ErrorWrapper(err, "write_field_failed", http.StatusInternalServerError)
				}
			}
		}

		err = writer.Close()
		if err != nil {
			return objects.ErrorWrapper(err, "close_multipart_writer_failed", http.StatusInternalServerError)
		}

		// 更新Content-Type
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	} else {
		_, err := io.Copy(requestBody, c.Request.Body)
		if err != nil {
			return objects.ErrorWrapper(err, "new_request_body_failed", http.StatusInternalServerError)
		}
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody.Bytes()))
	// 客户端要求的格式，与发给上游的 verbose_json 无关。
	// 校验放在发起上游请求之前：非法取值应像 OpenAI 一样直接 400，而不是先把钱花出去。
	clientResponseFormat := c.DefaultPostForm("response_format", "json")
	if !isValidTranscriptionFormat(clientResponseFormat) {
		return objects.ErrorWrapper(
			fmt.Errorf("response_format 取值 %s 非法，可选：json / text / srt / verbose_json / vtt", clientResponseFormat),
			"invalid_response_format", http.StatusBadRequest)
	}

	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return objects.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}

	if (relayMode == relaymode.AudioTranscription || relayMode == relaymode.AudioSpeech) && meta.ChannelType == channeltype.Azure {
		// https://learn.microsoft.com/en-us/azure/ai-services/openai/whisper-quickstart?tabs=command-line#rest-api
		apiKey := c.Request.Header.Get("Authorization")
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		req.Header.Set("api-key", apiKey)
	} else {
		req.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
	}
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	req.Header.Set("Accept", c.Request.Header.Get("Accept"))

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return objects.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}

	err = req.Body.Close()
	if err != nil {
		return objects.ErrorWrapper(err, "close_request_body_failed", http.StatusInternalServerError)
	}
	err = c.Request.Body.Close()
	if err != nil {
		return objects.ErrorWrapper(err, "close_request_body_failed", http.StatusInternalServerError)
	}

	// 状态码检查必须在读取 body 之前：RelayErrorHandler 需要未被消费的 resp.Body，
	// 且上游返回 HTML / 纯文本错误时不应被后面的 JSON 解析失败掩盖成 500。
	if resp.StatusCode != http.StatusOK {
		return RelayErrorHandler(resp)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return objects.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	err = resp.Body.Close()
	if err != nil {
		return objects.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}

	// 200 但 body 内含 error 字段的上游
	var openAIErr openai.SlimTextResponse
	if err = json.Unmarshal(responseBody, &openAIErr); err == nil {
		if openAIErr.Error.Message != "" {
			return objects.ErrorWrapper(fmt.Errorf("type %s, code %v, message %s", openAIErr.Error.Type, openAIErr.Error.Code, openAIErr.Error.Message), "request_error", http.StatusInternalServerError)
		}
	}

	// 上游固定按 verbose_json 返回：先取 duration 用于按秒计费，
	// 再降级成客户端原本请求的格式写回 resp.Body，供尾部统一拷贝。
	//
	// 解析不了则原样透传：contentType 为空表示未做转换，沿用上游的响应头。
	var audioDuration float64
	convertedBody := string(responseBody)
	var contentType string

	var verbose openai.WhisperVerboseJSONResponse
	if err = json.Unmarshal(responseBody, &verbose); err != nil {
		// 很多 whisper 兼容服务会忽略 response_format=verbose_json 而返回纯文本或 SRT。
		// 按既定策略降级：按 0 结算并告警，而不是让整个请求失败——500 既丢掉了上游本来
		// 可用的响应，又会命中 shouldRetry 把同一段音频重投到所有渠道。
		logger.Errorf(ctx, "[转写计费] 模型 %s 的上游响应不是 verbose_json，无法解析时长，按 0 结算并原样透传，需补适配: %s",
			meta.ActualModelName, err.Error())
	} else {
		audioDuration = verbose.Duration
		convertedBody, contentType, err = convertVerboseJSON(&verbose, responseBody, clientResponseFormat)
		if err != nil {
			return objects.ErrorWrapper(err, "convert_response_format_failed", http.StatusInternalServerError)
		}
	}
	resp.Body = io.NopCloser(bytes.NewBufferString(convertedBody))

	succeed = true
	defer func(ctx context.Context) {
		go objects.PostConsumeTranscriptionQuota(ctx, meta, audioDuration, preConsumedQuota)
	}(c.Request.Context())

	for k, v := range resp.Header {
		c.Writer.Header().Set(k, v[0])
	}
	// 上游返回的是 verbose_json 的类型与长度，必须按转换后的实际内容覆盖。
	// contentType 为空表示上游响应无法解析、正在原样透传，此时上游的头部本就是对的。
	if contentType != "" {
		c.Writer.Header().Set("Content-Type", contentType)
		c.Writer.Header().Set("Content-Length", strconv.Itoa(len(convertedBody)))
	}
	c.Writer.WriteHeader(resp.StatusCode)

	_, err = io.Copy(c.Writer, resp.Body)
	if err != nil {
		return objects.ErrorWrapper(err, "copy_response_body_failed", http.StatusInternalServerError)
	}
	err = resp.Body.Close()
	if err != nil {
		return objects.ErrorWrapper(err, "close_response_body_failed", http.StatusInternalServerError)
	}
	return nil
}
