package controller

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gocarina/gocsv"
	"github.com/songquanpeng/one-api/model"
)

type AddModelMetaRequest struct {
	Model           string   `json:"model" binding:"required"`
	ChannelType     int      `json:"channel_type" binding:"required"`
	Status          int      `json:"status"`
	ModelRatio      *float64 `json:"model_ratio" binding:"required"`
	CompletionRatio *float64 `json:"completion_ratio" binding:"required"`
}

// UpdateModelMetaRequest 更新 ModelMeta 的请求结构体
type UpdateModelMetaRequest struct {
	Id              int      `json:"id" binding:"required"`
	Status          int      `json:"status"`
	Model           *string  `json:"model"`
	ChannelType     *int     `json:"channel_type"`
	ModelRatio      *float64 `json:"model_ratio"`
	CompletionRatio *float64 `json:"completion_ratio"`
}

type UpdateModelRatioRequest struct {
	Model           string   `json:"model" binding:"required"`
	ModelRatio      *float64 `json:"model_ratio" binding:"required"`
	CompletionRatio *float64 `json:"completion_ratio" binding:"required"`
}

// GetAllModelMetas 获取模型元数据列表（支持分页）
// @Summary 获取模型元数据列表
// @Param p query int false "起始索引"
// @Param pageSize query int false "每页大小"
// @Success 200 {array} model.ModelMeta
// @Router /api/model_metas [get]
func GetAllModelMetas(c *gin.Context) {
	// 解析分页参数
	page, _ := strconv.Atoi(c.DefaultQuery("p", "0"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "10"))

	// 获取数据
	metas, err := model.GetModelMetaList(page*pageSize, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to fetch model metas",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    metas,
	})
}

// AddModelMeta 处理添加 ModelMeta 的请求
func AddModelMeta(c *gin.Context) {
	var req AddModelMetaRequest

	// 解析请求体
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}
	//if !c.Request.URL.Query().Has("status") {
	//	req.Status = model.ModelStatusOn
	//}

	// 构造 ModelMeta 对象
	meta := &model.ModelMeta{
		Model:           req.Model,
		ChannelType:     req.ChannelType,
		Status:          req.Status,
		ModelRatio:      *req.ModelRatio,
		CompletionRatio: *req.CompletionRatio,
	}

	// 调用 AddModelMeta 函数
	if err := model.CreateOrUpdateModelMeta(meta); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to add model meta",
			"details": err.Error(),
		})
		return
	}

	RefreshModels()

	// 返回成功响应
	c.JSON(http.StatusOK, gin.H{
		"message": "Model meta added successfully",
		"success": true,
	})
}

func GetModelMeta(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	// 构造 ModelMeta 对象
	model_meta := model.ModelMeta{Id: id}
	// 调用 DeleteModelMeta 函数
	err := model_meta.Get()
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
		"data":    model_meta,
	})
	return
}

// UpdateModelMeta 更新模型元数据
// @Summary 更新模型元数据
// @Param id path int true "ID"
// @Param channel_id path int true "Channel ID"
// @Param request body UpdateModelMetaRequest true "请求参数"
// @Success 200 {object} gin.H
// @Router /api/v1/model_metas/{id}/{channel_id} [put]
// UpdateModelMeta 更新模型元数据
// @Summary 更新模型元数据
// @Param id path int true "ID"
// @Param request body UpdateModelMetaRequest true "请求参数"
// @Success 200 {object} gin.H
// @Router /api/v1/model_metas/{id} [put]
func UpdateModelMeta(c *gin.Context) {
	var req UpdateModelMetaRequest

	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 构建动态更新字段
	updates := make(map[string]interface{})
	updates["status"] = req.Status // Status 是非指针类型，必填

	if req.ChannelType != nil {
		updates["channel_type"] = *req.ChannelType
	}
	if req.Model != nil {
		updates["model"] = *req.Model
	}
	if req.ModelRatio != nil {
		updates["model_ratio"] = *req.ModelRatio
	}
	if req.CompletionRatio != nil {
		updates["completion_ratio"] = *req.CompletionRatio // 确保 0 值被包含
	}

	//logger.Infof(c, "model meta : %#v", model_meta)
	err = model.ModelMetaUpdate(req.Id, updates)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	RefreshModels()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		//"data":    model_meta,
	})
}

// UpdateModelRatio 更新模型费率
// @Summary 更新模型费率
// @Param request body UpdateModelRatioRequest true "请求参数"
// @Success 200 {object} gin.H
// @Router /api/v1/model_metas/update_ratio [put]
func UpdateModelRatio(c *gin.Context) {
	var req UpdateModelRatioRequest

	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

    // 构建动态更新字段
	updates := make(map[string]interface{})
	updates["model_ratio"] = *req.ModelRatio
	updates["completion_ratio"] = *req.CompletionRatio

	err = model.ModelMetaUpdateByModel(req.Model, updates)
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
}



// DeleteModelMeta 删除模型元数据
// @Summary 删除模型元数据
// @Param id path int true "ID"
// @Success 200 {object} gin.H
// @Router /api/v1/model_metas/{id}/{channel_id} [delete]
func DeleteModelMeta(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	// 构造 ModelMeta 对象
	model_meta := model.ModelMeta{Id: id}

	// 调用 DeleteModelMeta 函数
	err := model_meta.Delete()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	RefreshModels()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
	return
}

// SearchModelMetas 搜索模型元数据
// @Summary 搜索模型元数据
// @Param keyword query string true "搜索关键字"
// @Success 200 {array} model.ModelMeta
// @Router /api/channel/search [get]
func SearchModelMetas(c *gin.Context) {
	keyword := c.Query("keyword")
	if keyword == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Keyword is required",
		})
		return
	}

	// 调用搜索函数
	metas, err := model.SearchModelMetas(keyword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to search model metas",
			"details": err.Error(),
		})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    metas,
	})
	return
}

// UploadModelMeta 上传模型元数据
// @Summary 上传模型元数据
// @Param file formData file true "文件"
// @Success 200 {object} gin.H
// @Router /api/model_metas/upload [post]
func UploadModelMeta(c *gin.Context) {
	// 上传json格式的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 读取文件内容
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.Comma = '|'
		return r // Allows use pipe as delimiter
	})

	// 解析文件内容
	var modelMetas []model.ModelMeta
	if err := gocsv.Unmarshal(f, &modelMetas); err != nil { // Load clients from file
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// 遍历模型元数据
	for _, modelMeta := range modelMetas {
		if err = model.CreateOrUpdateModelMeta(&modelMeta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	RefreshModels()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// BatchAddModelMeta 批量添加模型元数据
// @Summary 批量添加模型元数据
// @Param text body string true "模型元数据列表"
// @Success 200 {object} gin.H
// @Router /api/model_metas/batch_add [post]

func BatchAddModelMeta(c *gin.Context) {
	var data struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	gocsv.SetCSVReader(func(in io.Reader) gocsv.CSVReader {
		r := csv.NewReader(in)
		r.Comma = '|'
		return r // Allows use pipe as delimiter
	})

	var modelMetas []model.ModelMeta
	if err := gocsv.UnmarshalString(data.Text, &modelMetas); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	count := 0
	for _, modelMeta := range modelMetas {
		if modelMeta.Model == "" {
			continue
		}
		count++
		if err := model.CreateOrUpdateModelMeta(&modelMeta); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
	}

	RefreshModels()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("成功添加 %d 条模型元数据", count),
	})
}
