package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/client"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/objects"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/channeltype"
)

// RelayAudioSpeechHelper is a helper function for audio TTS(channelType.AudioSpeech) relay
func RelayAudioSpeechHelper(c *gin.Context, relayMode int) *objects.ErrorWithStatusCode {
	ctx := c.Request.Context()
	//if err := c.BindJSON(&TextToSpeechParam{}); err != nil {
	//	return objects.ErrorWrapper(err, "invalid_json", http.StatusBadRequest)
	//}
	meta := objects.GetRequestMeta(c)
	meta.OriginModelName = c.GetString(ctxkey.RequestModel)
	meta.ActualModelName = c.GetString(ctxkey.RequestModel)
	// audioModel := "whisper-1"

	// tokenId := c.GetInt(ctxkey.TokenId)
	// channelType := c.GetInt(ctxkey.ChannelType)
	// channelId := c.GetInt(ctxkey.ChannelId)
	// userId := c.GetInt(ctxkey.UserId)
	// group := c.GetString(ctxkey.Group)
	// tokenName := c.GetString(ctxkey.TokenName)

	var ttsRequest openai.TextToSpeechRequest
	{
		// Read JSON
		err := common.UnmarshalBodyReusable(c, &ttsRequest)
		// Check if JSON is valid
		if err != nil {
			return objects.ErrorWrapper(err, "invalid_json", http.StatusBadRequest)
		}
		// audioModel = ttsRequest.Model
		// Check if text is too long 4096
		if len(ttsRequest.Input) > 4096 {
			return objects.ErrorWrapper(errors.New("input is too long (over 4096 characters)"), "text_too_long", http.StatusBadRequest)
		}
	}

	preConsumedQuota, bizErr := objects.PreConsumeQuotaForAudio(ctx, ttsRequest.Input, meta)
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
					err := model.PostConsumeTokenQuota(meta.TokenId, -preConsumedQuota)
					if err != nil {
						logger.Error(ctx, fmt.Sprintf("error rollback pre-consumed quota: %s", err.Error()))
					}
				}()
			}(c.Request.Context())
		}
	}()

	// map model name
	// modelMapping := c.GetString(ctxkey.ModelMapping)
	// if modelMapping != "" {
	// 	modelMap := make(map[string]string)
	// 	err := json.Unmarshal([]byte(modelMapping), &modelMap)
	// 	if err != nil {
	// 		return objects.ErrorWrapper(err, "unmarshal_model_mapping_failed", http.StatusInternalServerError)
	// 	}
	// 	if modelMap[audioModel] != "" {
	// 		audioModel = modelMap[audioModel]
	// 	}
	// }

	// baseURL := channeltype.ChannelBaseURLs[meta.ChannelType]
	// requestURL := c.Request.URL.String()
	// if c.GetString(ctxkey.BaseURL) != "" {
	// 	baseURL = c.GetString(ctxkey.BaseURL)
	// }

	// fullRequestURL := openai.GetFullRequestURL(baseURL, requestURL, meta.ChannelType)
	// if meta.ChannelType == channeltype.Azure {
	// 	apiVersion := meta.Config.APIVersion
	// 	if relayMode == relaymode.AudioTranscription {
	// 		// https://learn.microsoft.com/en-us/azure/ai-services/openai/whisper-quickstart?tabs=command-line#rest-api
	// 		fullRequestURL = fmt.Sprintf("%s/openai/deployments/%s/audio/transcriptions?api-version=%s", baseURL, audioModel, apiVersion)
	// 	} else if relayMode == relaymode.AudioSpeech {
	// 		// https://learn.microsoft.com/en-us/azure/ai-services/openai/text-to-speech-quickstart?tabs=command-line#rest-api
	// 		fullRequestURL = fmt.Sprintf("%s/openai/deployments/%s/audio/speech?api-version=%s", baseURL, audioModel, apiVersion)
	// 	}
	// }

	fullRequestURL := GetRequestFullUrl(c, meta, relayMode)

	requestBody := &bytes.Buffer{}

	_, err := io.Copy(requestBody, c.Request.Body)
	if err != nil {
		return objects.ErrorWrapper(err, "new_request_body_failed", http.StatusInternalServerError)
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody.Bytes()))

	req, err := http.NewRequest(c.Request.Method, fullRequestURL, requestBody)
	if err != nil {
		return objects.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}

	if meta.ChannelType == channeltype.Azure {
		// https://learn.microsoft.com/en-us/azure/ai-services/openai/whisper-quickstart?tabs=command-line#rest-api
		apiKey := c.Request.Header.Get("Authorization")
		apiKey = strings.TrimPrefix(apiKey, "Bearer ")
		req.Header.Set("api-key", apiKey)
		req.ContentLength = c.Request.ContentLength
	} else {
		req.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
	}
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	req.Header.Set("Accept", c.Request.Header.Get("Accept"))

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		succeed = false
		return objects.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Errorf(ctx, "Failed to close response body: %s", err.Error())
		}
	}(c.Request.Body)
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logger.Errorf(ctx, "Failed to close request body: %s", err.Error())
		}
	}(req.Body)

	if resp.StatusCode != http.StatusOK {
		return RelayErrorHandler(resp)
	}

	//io.ReadAll(req.Body)
	//json.Unmarshal()
	//quotaDelta := quota - preConsumedQuota
	//defer func(ctx context.Context) {
	//	go objects.PostConsumeQuota(ctx, usage, meta, preConsumedQuota)
	//	go billing.PostConsumeQuota(ctx, meta.TokenId, quotaDelta, quota, meta.UserId, meta.ChannelId, modelRatio, groupRatio, audioModel, meta.TokenName)
	//}(c.Request.Context())

	for k, v := range resp.Header {
		c.Writer.Header().Set(k, v[0])
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
