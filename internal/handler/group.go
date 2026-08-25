package controller

import (
	"net/http"

	"github.com/gin-gonic/gin"
	billingratio "github.com/zicorn/llm-proxy/internal/relay/billing/ratio"
)

func GetGroups(c *gin.Context) {
	groupNames := make([]string, 0)
	for groupName := range billingratio.GroupRatio {
		groupNames = append(groupNames, groupName)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    groupNames,
	})
}
