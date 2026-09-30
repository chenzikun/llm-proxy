package wan3

import (
	"encoding/json"
	"fmt"
)

// 百炼按**秒**计费，且秒数就在建单请求里，不需要像 Seedance 那样反推 token。
// 查询接口的 usage.duration 是权威值，但那是任务跑完之后的事；本代理选择在建单
// 时刻结算（与图片、转写、Seedance 一致），因此这里读的是请求参数。

const (
	// defaultDuration 是 parameters.duration 缺省时的兜底秒数 —— 文档写明默认 5。
	defaultDuration = 5

	// maxDuration 是文档给出的输出时长上限（秒）。
	maxDuration = 30

	// defaultResolution 是 parameters.resolution 缺省时的档位。
	defaultResolution = "1080P"

	// asyncEnable 是建单必须带的异步标识头的值。
	asyncEnable = "enable"

	// mediaTypeReferenceVideo 是参考视频的 media type。
	mediaTypeReferenceVideo = "reference_video"
)

// Estimate 是一次建单的计费用量。
type Estimate struct {
	// Seconds 是计入计费的输出时长（秒）。
	Seconds float64
	// Resolution 只进消费日志，不参与算钱：分辨率档位不同单价不同，而
	// model_meta 一个模型只有一个价位。写进日志，便于对账时解释差异。
	Resolution string
	// HasVideoInput 表示请求带了参考视频。
	HasVideoInput bool
	// DurationFallback 表示 duration 缺省或为 -1（智能推荐），秒数用的是兜底值。
	// 这种情况下秒数是猜的，日志里要写明。
	DurationFallback bool
}

// createReqBody 是建单请求体里参与估算的字段。
// 其余字段（prompt、media 的 url、seed…）原样透传，不在这里解析。
type createReqBody struct {
	Model string `json:"model"`
	Input struct {
		Media []struct {
			Type string `json:"type"`
		} `json:"media"`
	} `json:"input"`
	Parameters struct {
		Duration   int    `json:"duration"`
		Resolution string `json:"resolution"`
	} `json:"parameters"`
}

// EstimateFromBody 解析建单请求体并算出计费秒数。
func EstimateFromBody(body []byte) (Estimate, error) {
	var req createReqBody
	if err := json.Unmarshal(body, &req); err != nil {
		return Estimate{}, fmt.Errorf("wan3: 解析建单请求体失败: %w", err)
	}
	return EstimateFor(req), nil
}

// EstimateFor 对已解析的请求体做估算，导出出来只为让测试能直接构造入参。
func EstimateFor(req createReqBody) Estimate {
	fallback := false
	seconds := float64(req.Parameters.Duration)
	switch {
	case req.Parameters.Duration <= 0:
		// 缺省与 -1（智能推荐时长）都落到这里。上游按实际输出时长计费，而那个值
		// 要等任务跑完才知道，这里只能按文档写明的默认值占位。
		seconds = defaultDuration
		fallback = true
	case seconds > maxDuration:
		// 超出上限的请求上游会拒（拒了就不计费），clamp 只是为了别在
		// "上游偷偷按上限截断"的情况下多扣。
		seconds = maxDuration
	}

	resolution := req.Parameters.Resolution
	if resolution == "" {
		resolution = defaultResolution
	}

	return Estimate{
		Seconds:          seconds,
		Resolution:       resolution,
		HasVideoInput:    hasVideoInput(req),
		DurationFallback: fallback,
	}
}

// hasVideoInput 判断 media 里是否带了参考视频。
// 它只影响日志措辞：百炼按输出时长计费，输入视频不额外计价
// （这点和 Seedance 不同 —— 那边输入视频时长是要算钱的）。
func hasVideoInput(req createReqBody) bool {
	for _, m := range req.Input.Media {
		if m.Type == mediaTypeReferenceVideo {
			return true
		}
	}
	return false
}
