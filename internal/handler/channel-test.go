package controller

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zicorn/llm-proxy/internal/objects"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common/client"
	"github.com/zicorn/llm-proxy/pkg/common/config"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/pkg/common/logger"
	"github.com/zicorn/llm-proxy/pkg/common/message"
	"github.com/zicorn/llm-proxy/internal/middleware"
	"github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/monitor"
	relay "github.com/zicorn/llm-proxy/internal/relay"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
	"github.com/zicorn/llm-proxy/internal/relay/controller"
	relaymodel "github.com/zicorn/llm-proxy/internal/relay/entity"
	"github.com/zicorn/llm-proxy/internal/relay/relaymode"
)

func buildTestRequest(model string) *relaymodel.GeneralOpenAIRequest {
	if model == "" {
		model = config.DefaultChatModel
	}
	testRequest := &relaymodel.GeneralOpenAIRequest{
		MaxTokens: 2,
		Model:     model,
	}
	testMessage := relaymodel.Message{
		Role:    "user",
		Content: "hi",
	}
	testRequest.Messages = append(testRequest.Messages, testMessage)
	return testRequest
}

// seedanceTasksProbePath 是 Seedance 渠道探活用的只读路径（查询任务列表）。
const seedanceTasksProbePath = "/api/v3/contents/generations/tasks?page_size=1"

// testSeedanceChannel 用一条只读请求探活 Seedance 渠道。
//
// 不能走通用的 testChannel：那条路径打的是 /v1/chat/completions，本渠道没有
// 这个接口，会把健康渠道判成故障 —— 而 testChannels 是定时跑的，判故障的后
// 果是自动禁用。也不用「创建任务」探活，那会真的产生一笔视频费用。
func testSeedanceChannel(channel *model.Channel) (error, *objects.Error) {
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = channeltype.ChannelBaseURLs[channel.Type]
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+seedanceTasksProbePath, nil)
	if err != nil {
		return err, nil
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err, nil
	}
	// 通用 testChannel 靠 adaptor.DoResponse 关 body，这里没有那一步，自己关。
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		relayErr := controller.RelayErrorHandler(resp)
		return fmt.Errorf("status code %d: %s", resp.StatusCode, relayErr.Error.Message), &relayErr.Error
	}
	return nil, nil
}

// wan3TasksProbePath 是 Wan3 渠道探活用的只读路径。故意用一个不存在的 task_id：
// 百炼对不存在的任务返回业务错误码，而不是 401 —— 那正好证明"Key 被接受了"。
const wan3TasksProbePath = "/api/v1/tasks/healthcheck-probe"

// testWan3Channel 探活 Wan3 渠道。
//
// 与 Seedance 同理，不能走通用的 testChannel（那条路径打 /v1/chat/completions，
// 本渠道没有这个接口，会把健康渠道判成故障，进而被定时任务自动禁用）。
// 也不能用「建单」探活 —— 那是要花钱的。
//
// 判定口径：只有 401/403 才算失败（Key 被拒），其余一律视为通过 ——
// 探的是一个不存在的任务，百炼必然会回业务错误，那是"鉴权没问题"的证据，
// 不是"渠道坏了"。
func testWan3Channel(channel *model.Channel) (error, *objects.Error) {
	baseURL := channel.GetBaseURL()
	if baseURL == "" {
		baseURL = channeltype.ChannelBaseURLs[channel.Type]
	}
	if baseURL == "" {
		return errors.New("未配置 BaseURL（形如 https://{WorkspaceId}.{region}.maas.aliyuncs.com）"), nil
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+wan3TasksProbePath, nil)
	if err != nil {
		return err, nil
	}
	req.Header.Set("Authorization", "Bearer "+channel.Key)
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err, nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		relayErr := controller.RelayErrorHandler(resp)
		return fmt.Errorf("Key 被上游拒绝（HTTP %d）：%s", resp.StatusCode, relayErr.Error.Message), &relayErr.Error
	}
	if resp.StatusCode >= 500 {
		relayErr := controller.RelayErrorHandler(resp)
		return fmt.Errorf("上游返回 %d：%s", resp.StatusCode, relayErr.Error.Message), &relayErr.Error
	}
	return nil, nil
}

func testChannel(channel *model.Channel, request *relaymodel.GeneralOpenAIRequest) (err error, openaiErr *objects.Error) {
	switch channel.Type {
	case channeltype.Seedance:
		return testSeedanceChannel(channel)
	case channeltype.Wan3:
		return testWan3Channel(channel)
	}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Method: "POST",
		URL:    &url.URL{Path: "/v1/chat/completions"},
		Body:   nil,
		Header: make(http.Header),
	}
	c.Request.Header.Set("Authorization", "Bearer "+channel.Key)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(ctxkey.ChannelType, channel.Type)
	c.Set(ctxkey.BaseURL, channel.GetBaseURL())
	cfg, _ := channel.LoadConfig()
	c.Set(ctxkey.Config, cfg)
	middleware.SetupContextForSelectedChannel(c, channel, "")
	meta := objects.GetRequestMeta(c)
	apiType := channeltype.ToAPIType(channel.Type)
	adaptor := relay.GetAdaptor(apiType)
	if adaptor == nil {
		return fmt.Errorf("invalid api type: %d, adaptor is nil", apiType), nil
	}
	modelName := request.Model
	modelMap := channel.GetModelMapping()
	if modelName == "" || !strings.Contains(channel.Models, modelName) {
		modelNames := strings.Split(channel.Models, ",")
		if len(modelNames) > 0 {
			modelName = modelNames[0]
		}
		if modelMap != nil && modelMap[modelName] != "" {
			modelName = modelMap[modelName]
		}
	}
	meta.OriginModelName, meta.ActualModelName = request.Model, modelName
	request.Model = modelName
	if err := adaptor.Init(meta); err != nil {
		return fmt.Errorf("init failed: %s", err.Error()), nil
	}
	convertedRequest, err := adaptor.ConvertRequest(c, relaymode.ChatCompletions, request)
	if err != nil {
		return err, nil
	}
	jsonData, err := json.Marshal(convertedRequest)
	if err != nil {
		return err, nil
	}
	logger.SysLog(string(jsonData))
	requestBody := bytes.NewBuffer(jsonData)
	c.Request.Body = io.NopCloser(requestBody)
	resp, err := adaptor.DoRequest(c, meta, requestBody)
	if err != nil {
		return err, nil
	}
	if resp != nil && resp.StatusCode != http.StatusOK {
		err := controller.RelayErrorHandler(resp)
		return fmt.Errorf("status code %d: %s", resp.StatusCode, err.Error.Message), &err.Error
	}
	usage, _, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		return fmt.Errorf("%s", respErr.Error.Message), &respErr.Error
	}
	if usage == nil {
		return errors.New("usage is nil"), nil
	}
	result := w.Result()
	// print result.Body
	respBody, err := io.ReadAll(result.Body)
	if err != nil {
		return err, nil
	}
	logger.SysLog(fmt.Sprintf("testing channel #%d, response: \n%s", channel.Id, string(respBody)))
	return nil, nil
}

func TestChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channel, err := model.GetChannelById(id, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	model := c.Query("model")
	testRequest := buildTestRequest(model)
	tik := time.Now()
	err, _ = testChannel(channel, testRequest)
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	if err != nil {
		milliseconds = 0
	}
	go channel.UpdateResponseTime(milliseconds)
	consumedTime := float64(milliseconds) / 1000.0
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"time":    consumedTime,
			"model":   model,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"time":    consumedTime,
		"model":   model,
	})
	return
}

var testAllChannelsLock sync.Mutex
var testAllChannelsRunning bool = false

func testChannels(notify bool, scope string) error {
	if config.RootUserEmail == "" {
		config.RootUserEmail = model.GetRootUserEmail()
	}
	testAllChannelsLock.Lock()
	if testAllChannelsRunning {
		testAllChannelsLock.Unlock()
		return errors.New("测试已在运行中")
	}
	testAllChannelsRunning = true
	testAllChannelsLock.Unlock()
	channels, err := model.GetAllChannels(0, 0, scope)
	if err != nil {
		return err
	}
	var disableThreshold = int64(config.ChannelDisableThreshold * 1000)
	if disableThreshold == 0 {
		disableThreshold = 10000000 // a impossible value
	}
	go func() {
		for _, channel := range channels {
			isChannelEnabled := channel.Status == model.ChannelStatusEnabled
			tik := time.Now()
			testRequest := buildTestRequest("")
			err, openaiErr := testChannel(channel, testRequest)
			tok := time.Now()
			milliseconds := tok.Sub(tik).Milliseconds()
			if isChannelEnabled && milliseconds > disableThreshold {
				err = fmt.Errorf("响应时间 %.2fs 超过阈值 %.2fs", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0)
				if config.AutomaticDisableChannelEnabled {
					monitor.DisableChannel(channel.Id, channel.Name, err.Error())
				} else {
					_ = message.Notify(message.ByAll, fmt.Sprintf("渠道 %s （%d）测试超时", channel.Name, channel.Id), "", err.Error())
				}
			}
			if isChannelEnabled && monitor.ShouldDisableChannel(openaiErr, -1) {
				monitor.DisableChannel(channel.Id, channel.Name, err.Error())
			}
			if !isChannelEnabled && monitor.ShouldEnableChannel(err, openaiErr) {
				monitor.EnableChannel(channel.Id, channel.Name)
			}
			channel.UpdateResponseTime(milliseconds)
			time.Sleep(config.RequestInterval)
		}
		testAllChannelsLock.Lock()
		testAllChannelsRunning = false
		testAllChannelsLock.Unlock()
		if notify {
			err := message.Notify(message.ByAll, "渠道测试完成", "", "渠道测试完成，如果没有收到禁用通知，说明所有渠道都正常")
			if err != nil {
				logger.SysError(fmt.Sprintf("failed to send email: %s", err.Error()))
			}
		}
	}()
	return nil
}

func TestChannels(c *gin.Context) {
	scope := c.Query("scope")
	if scope == "" {
		scope = "all"
	}
	err := testChannels(true, scope)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

func AutomaticallyTestChannels(frequency int) {
	if frequency <= 0 {
		return
	}
	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		logger.SysLog("testing all channels")
		_ = testChannels(false, "all")
		logger.SysLog("channel test finished")
	}
}
