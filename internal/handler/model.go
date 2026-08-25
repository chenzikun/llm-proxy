package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/zicorn/llm-proxy/pkg/common/ctxkey"
	"github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/internal/objects"
	"github.com/zicorn/llm-proxy/internal/relay/adaptor/openai"
	"github.com/zicorn/llm-proxy/internal/relay/channeltype"
)

// https://platform.openai.com/docs/api-reference/models/list

type OpenAIModelPermission struct {
	Id                 string  `json:"id"`
	Object             string  `json:"object"`
	Created            int     `json:"created"`
	AllowCreateEngine  bool    `json:"allow_create_engine"`
	AllowSampling      bool    `json:"allow_sampling"`
	AllowLogprobs      bool    `json:"allow_logprobs"`
	AllowSearchIndices bool    `json:"allow_search_indices"`
	AllowView          bool    `json:"allow_view"`
	AllowFineTuning    bool    `json:"allow_fine_tuning"`
	Organization       string  `json:"organization"`
	Group              *string `json:"group"`
	IsBlocking         bool    `json:"is_blocking"`
}

type OpenAIModels struct {
	Id         string                  `json:"id"`
	Object     string                  `json:"object"`
	Created    int                     `json:"created"`
	OwnedBy    string                  `json:"owned_by"`
	Permission []OpenAIModelPermission `json:"permission"`
	Root       string                  `json:"root"`
	Parent     *string                 `json:"parent"`
}

// var models []OpenAIModels
// var modelsMap map[string]OpenAIModels

// // todo:zikun 重要，保存了channel与ID的关系
// var channelId2Models map[int][]string

func RefreshModels() ([]OpenAIModels, map[string]OpenAIModels, map[int][]string) {
	// 清空
	var models []OpenAIModels
    var modelsMap map[string]OpenAIModels

    // todo:zikun 重要，保存了channel与ID的关系
    var channelId2Models map[int][]string

	// 开始干活
	var permission []OpenAIModelPermission
	permission = append(permission, OpenAIModelPermission{
		Id:                 "modelperm-LwHkVFn8AcMItP432fKKDIKJ",
		Object:             "model_permission",
		Created:            1626777600,
		AllowCreateEngine:  true,
		AllowSampling:      true,
		AllowLogprobs:      true,
		AllowSearchIndices: false,
		AllowView:          true,
		AllowFineTuning:    false,
		Organization:       "*",
		Group:              nil,
		IsBlocking:         false,
	})
	// https://platform.openai.com/docs/models/model-endpoint-compatibility
	// for i := 0; i < apitype.Dummy; i++ {
	// 	if i == apitype.AIProxyLibrary {
	// 		continue
	// 	}
	// 	adaptor := relay.GetAdaptor(i)
	// 	channelName := adaptor.GetChannelName()
	// 	// modelNames := adaptor.GetModelList()
	// 	modelNames, err := model.GetModelListByChannelType(i)
	// 	if err != nil {
	// 		continue
	// 	}
	// 	for _, modelName := range modelNames {
	// 		models = append(models, OpenAIModels{
	// 			Id:         modelName,
	// 			Object:     "model",
	// 			Created:    1626777600,
	// 			OwnedBy:    channelName,
	// 			Permission: permission,
	// 			Root:       modelName,
	// 			Parent:     nil,
	// 		})
	// 	}
	// }

	// 获取models
	for channelType := 1; channelType < channeltype.Dummy; channelType++ {
		if channelType == channeltype.Azure {
			continue
		}
		channelName, channelModelList := openai.GetCompatibleChannelMeta(channelType)
		for _, modelName := range channelModelList {
			models = append(models, OpenAIModels{
				Id:         modelName,
				Object:     "model",
				Created:    1626777600,
				OwnedBy:    channelName,
				Permission: permission,
				Root:       modelName,
				Parent:     nil,
			})
		}
	}
	// 将models转换为map
	modelsMap = make(map[string]OpenAIModels)
	for _, model_ := range models {
		modelsMap[model_.Id] = model_
	}
	// 缓存每个渠道的模型列表
	channelId2Models = make(map[int][]string)
	for i := 1; i < channeltype.Dummy; i++ {
		// adaptor := relay.GetAdaptor(channeltype.ToAPIType(i))
		// meta := &objects.Meta{
		// 	ChannelType: i,
		// }
		// if err := adaptor.Init(meta); err != nil {
		// 	// 这里报错没关系
		// }
		// channelId2Models[i] = adaptor.GetModelList()
		modelNames, err := model.GetModelListByChannelType(i)
		if err != nil {
			continue
		}
		channelId2Models[i] = modelNames
	}
    return models, modelsMap, channelId2Models
}

func DashboardListModels(c *gin.Context) {
	_, _, channelId2Models := RefreshModels()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channelId2Models,
	})
}

func ListAllModels(c *gin.Context) {
	models, _, _ := RefreshModels()
	c.JSON(200, gin.H{
		"object": "list",
		"data":   models,
	})
}

func ListModels(c *gin.Context) {
    models, _, _ := RefreshModels()
	ctx := c.Request.Context()
	var availableModels []string
	if c.GetString(ctxkey.AvailableModels) != "" {
		availableModels = strings.Split(c.GetString(ctxkey.AvailableModels), ",")
	} else {
		userId := c.GetInt(ctxkey.UserId)
		userGroup, _ := model.CacheGetUserGroup(userId)
		availableModels, _ = model.CacheGetGroupModels(ctx, userGroup)
	}
	modelSet := make(map[string]bool)
	for _, availableModel := range availableModels {
		modelSet[availableModel] = true
	}
	availableOpenAIModels := make([]OpenAIModels, 0)
	for _, model_ := range models {
		if _, ok := modelSet[model_.Id]; ok {
			modelSet[model_.Id] = false
			availableOpenAIModels = append(availableOpenAIModels, model_)
		}
	}
	for modelName, ok := range modelSet {
		if ok {
			availableOpenAIModels = append(availableOpenAIModels, OpenAIModels{
				Id:      modelName,
				Object:  "model",
				Created: 1626777600,
				OwnedBy: "custom",
				Root:    modelName,
				Parent:  nil,
			})
		}
	}
	c.JSON(200, gin.H{
		"object": "list",
		"data":   availableOpenAIModels,
	})
}

func RetrieveModel(c *gin.Context) {
	_, modelsMap, _ := RefreshModels()
	modelId := c.Param("model")

	if model_, ok := modelsMap[modelId]; ok {
		c.JSON(200, model_)
	} else {
		Error := objects.Error{
			Message: fmt.Sprintf("The model '%s' does not exist", modelId),
			Type:    "invalid_request_error",
			Param:   "model",
			Code:    "model_not_found",
		}
		c.JSON(200, gin.H{
			"error": Error,
		})
	}
}

func GetUserAvailableModels(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.GetInt(ctxkey.UserId)
	userGroup, err := model.CacheGetUserGroup(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	models, err := model.CacheGetGroupModels(ctx, userGroup)
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
		"data":    models,
	})
	return
}
