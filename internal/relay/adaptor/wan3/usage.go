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

// 分辨率系数。百炼按分辨率分三档计价，官方价目是干净的 1 : 2 : 4
// （$0.05 / $0.1 / $0.2 每秒；国内 0.21 / 0.42 / 0.84 元，同比例）。
//
// 基准档取 **720P**：model_meta 一个模型只有一个价位，管理员填的单价即"720P 的
// 每秒价"，其余档位按系数折算。所以 480P 只收一半、1080P 收两倍，而不是一刀切。
const (
	resolution480P  = "480P"
	resolution720P  = "720P"
	resolution1080P = "1080P"

	factor480P  = 0.5
	factor720P  = 1.0
	factor1080P = 2.0
)

// resolutionFactors 是分辨率 → 折算系数。**只认大写**，与上游一致；
// 小写的 720p 会被上游当非法值，这里也不该悄悄替它兜底。
var resolutionFactors = map[string]float64{
	resolution480P:  factor480P,
	resolution720P:  factor720P,
	resolution1080P: factor1080P,
}

// Estimate 是一次建单的计费用量。
type Estimate struct {
	// Seconds 是请求的输出时长（秒），未按分辨率折算。
	Seconds float64
	// Resolution 是请求的分辨率档位（缺省时为 defaultResolution）。
	Resolution string
	// ResolutionFactor 是该档位相对 720P 的折算系数。
	ResolutionFactor float64
	// HasVideoInput 表示请求带了参考视频。
	HasVideoInput bool
	// DurationFallback 表示 duration 缺省或为 -1（智能推荐），秒数用的是兜底值。
	// 这种情况下秒数是猜的，日志里要写明。
	DurationFallback bool
}

// BillableSeconds 是折算到基准档（720P）后的计费秒数。
func (e Estimate) BillableSeconds() float64 {
	return e.Seconds * e.ResolutionFactor
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
	factor, known := resolutionFactors[resolution]
	if !known {
		// 认不出的档位按基准档算。上游拿到非法分辨率会直接拒（拒了就不计费），
		// 所以这条分支实际只在"上游将来加了新档位"时才会走到 —— 那时按 720P
		// 算至少不会把单价乘歪，日志里的原始分辨率能让人看出需要补档。
		factor = factor720P
	}

	return Estimate{
		Seconds:          seconds,
		Resolution:       resolution,
		ResolutionFactor: factor,
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
