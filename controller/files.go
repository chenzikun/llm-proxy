package controller

import (
	"encoding/json"
	"net/http"

	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/objects"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

type OpenAIFile struct {
	Id        string `json:"id"`
	Object    string `json:"object"`
	Bytes     int    `json:"bytes"`
	CreatedAt int    `json:"created_at"`
	Filename  string `json:"filename"`
	Purpose   string `json:"purpose"`
}

type OpenAIFileListResponse struct {
	Data []OpenAIFile `json:"data"`
    Object string `json:"object"`
    HasMore bool `json:"has_more"`
    FirstId string `json:"first_id"`
    LastId string `json:"last_id"`
}

func GetFileRelay(c *gin.Context) {
	// fileId := c.Param("id")
	// userId := c.GetInt(ctxkey.UserId)
    ctx := c.Request.Context()
	// 创建代理对象
	proxy, err := common.NewProxy(c)
	if err != nil {
		logger.Errorf(ctx, "Failed to create proxy: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "create_proxy_failed",
		})
		return
	}
	defer proxy.Close()

    // 发送请求
	if err := proxy.Request(); err != nil {
		logger.Errorf(ctx, "Request failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "do_request_failed",
		})
	}

    // 读取并解析返回值
	var openAIFile OpenAIFile
	data, err := proxy.ReadResponseBody()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "read_response_body_failed",
		})
		return
	}
	if err = json.Unmarshal(data, &openAIFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "unmarshal_response_body_failed",
		})
		return
	}

    proxy.WriteResponse()
}

func UploadFileRelay(c *gin.Context) {
	ctx := c.Request.Context()
	userId := c.GetInt(ctxkey.UserId)
	requestModel := c.GetString(ctxkey.RequestModel)

	// 创建代理对象
	proxy, err := common.NewProxy(c)
	if err != nil {
		logger.Errorf(ctx, "Failed to create proxy: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "create_proxy_failed",
		})
		return
	}
	defer proxy.Close()

	// 计算token数量
	content := proxy.GetRequestBody()
	tokens := objects.CountTokenInput(string(content), requestModel)

	// 发送请求
	if err := proxy.Request(); err != nil {
		logger.Errorf(ctx, "Request failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "do_request_failed",
		})
		return
	}
	// 读取并解析返回值
	var openAIFile OpenAIFile
	data, err := proxy.ReadResponseBody()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "read_response_body_failed",
		})
		return
	}
	if err = json.Unmarshal(data, &openAIFile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "unmarshal_response_body_failed",
		})
		return
	}
	file := model.File{
		ObjectId: openAIFile.Id,
		Object:   openAIFile.Object,
		Bytes:    openAIFile.Bytes,
		Filename: openAIFile.Filename,
		Purpose:  openAIFile.Purpose,
		Tokens:   tokens,
		UserId:   userId,
	}
	if err = file.Insert(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "write_db_failed",
		})
		return
	}
	proxy.WriteResponse()
}


func ListFilesRelay(c *gin.Context) {
	// userId := c.GetInt(ctxkey.UserId)
    ctx := c.Request.Context()

    // 创建代理对象
	proxy, err := common.NewProxy(c)
	if err != nil {
		logger.Errorf(ctx, "Failed to create proxy: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "create_proxy_failed",
		})
		return
	}
	defer proxy.Close()

    // 发送请求
	if err := proxy.Request(); err != nil {
		logger.Errorf(ctx, "Request failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "do_request_failed",
		})
		return
	}
	// 读取并解析返回值
	var openAIFileList OpenAIFileListResponse
	data, err := proxy.ReadResponseBody()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "read_response_body_failed",
		})
		return
	}
	if err = json.Unmarshal(data, &openAIFileList); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "unmarshal_response_body_failed",
		})
		return
	}

    proxy.WriteResponse()
}


type OpenAIDeleteFileResponse struct {
	Object  string `json:"object"`
	Id      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

func DeleteFileRelay(c *gin.Context) {
	ctx := c.Request.Context()
	proxy, err := common.NewProxy(c)
	if err != nil {
		logger.Errorf(ctx, "Failed to create proxy: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "create_proxy_failed",
		})
		return
	}
	defer proxy.Close()
	if err := proxy.Request(); err != nil {
		logger.Errorf(ctx, "Request failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "do_request_failed",
		})
		return
	}
	// 读取返回内容
	content, err := proxy.ReadResponseBody()
	if err != nil {
		logger.Errorf(ctx, "ReadResponse failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "read_response_failed",
		})
		return
	}
	// 解析返回内容
	var r OpenAIDeleteFileResponse
	if err = json.Unmarshal(content, &r); err != nil {
		logger.Errorf(ctx, "Unmarshal failed: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "unmarshal_response_failed",
		})
		return
	}
	if !r.Deleted {
		var errMsg map[string]map[string]string
		_ = json.Unmarshal(content, &errMsg)
		c.JSON(http.StatusInternalServerError, errMsg)
		return
	}
	// 删除数据库记录
	if err = model.DeleteFile(r.Id); err != nil {
		logger.Errorf(ctx, "DeleteFile from db failed: %s", err.Error())
	}
	proxy.WriteResponse()
}
