package objects

import (
	"encoding/json"
	"os"
)

type Model struct {
	Name             string  `json:"name"`
	InputTokensCost  any     `json:"inputTokensCost"`
	OutputTokensCost float64 `json:"outputTokensCost"`
	Platform         string  `json:"platform"`
	Type             string  `json:"type"`
}

type ChannelModel struct {
	ChannelType int     `json:"channelType"`
	Models      []Model `json:"models"`
}

// AllModels 模型列表，从配置文件中读取，包含所有支持的模型
var AllModels []ChannelModel

func init() {
	filename := os.Getenv("PATH_TO_CONFIG")
	if filename == "" {
		filename = "models.json"
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(data, &AllModels)
	if err != nil {
		panic(err)
	}
}
