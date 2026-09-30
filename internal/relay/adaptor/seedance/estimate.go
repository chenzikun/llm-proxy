package seedance

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// 本文件实现 BytePlus 自己的用量估算公式。上游按 output token 计费，
// 计费文档给的是：
//
//	tokens = (输入视频时长 + 输出视频时长) × 输出宽 × 输出高 × 输出帧率 / 1024
//
// 这里照抄该公式。它同时是上游预扣时用的式子，所以本地算出来的数与
// BytePlus 账单上的数应当同一量级（实测校验见 estimate_test.go）。

// defaultFrameRate 是上游未指定帧率时的输出帧率。
const defaultFrameRate = 24

// areaByResolution 是各分辨率档位在 16:9 下的输出画幅面积（像素）。
//
// 480p 的宽取官方的整数 854（而非 853.33），这样四档算出的 token/秒与
// BytePlus 公布的 9608 / 21600 / 48600 / 194400 逐位一致。
var areaByResolution = map[string]float64{
	"480p":  854 * 480,
	"720p":  1280 * 720,
	"1080p": 1920 * 1080,
	"4k":    3840 * 2160,
}

// baseRatio 是 areaByResolution 的基准宽高比。
const baseRatio = 16.0 / 9.0

// ratioFactor 是各宽高比相对 16:9 的面积倍率。
//
// 横竖屏只差一次旋转，面积相同，所以 16:9 与 9:16 共用同一个倍率。
// "adaptive" 不在表里，按 16:9 处理 —— 它是"跟随输入"的意思，
// 而输入是图或文本时上游会落到 16:9。
var ratioFactor = map[string]float64{
	"16:9": 1,
	"9:16": 1,
	"4:3":  (4.0 / 3.0) / baseRatio,
	"3:4":  (4.0 / 3.0) / baseRatio,
	"1:1":  1 / baseRatio,
	"21:9": (21.0 / 9.0) / baseRatio,
}

// 分辨率带来的**单价台阶**。注意它与 token 数是两回事：token 数里已经含了
// 宽 × 高，1080P 的 token 本来就是 720P 的 2.25 倍；这里说的是每 token 的价钱
// 在 1080P 档又高了一截。
//
// 官方价目（BytePlus 模型详情页）里 480P 与 720P 同价、1080P 另起一档：
//
//	含视频输入  $6.4 → $7.0 /M tokens
//	不含视频输入 $10.70 → $11.70 /M tokens
//
// 两档的比例一致（≈1.094），所以这里用一个系数表达，而不是给每档硬编码价格。
const (
	priceFactorBase = 1.0
	priceFactorHigh = 7.0 / 6.4 // 1080P 及以上
)

// 模型最长时长（秒），用于 duration = -1（上游自动决定）时的兜底。
// 取值来自文档 §五「模型选型」。用**前缀**匹配而不是全等，是因为同一代模型
// 存在 doubao-seedance-2-5-pro-250528 与 dreamina-seedance-2-5-260628
// 两套命名，全等匹配会在换名时静默落到兜底值。
const (
	maxDurationSeedance25 = 30
	maxDurationSeedance20 = 15
	maxDurationDefault    = 12 // 1.x 系列；也是认不出的模型的兜底
)

// Estimate 是一次生成任务的计费用量估算。
type Estimate struct {
	// Tokens 是公式算出的 output token 数。
	Tokens int
	// HasVideoInput 表示请求里带了参考视频。
	HasVideoInput bool
	// InputVideoSeconds 是**实际计入公式**的输入视频时长。0 表示没计入。
	//
	// 上游把输入视频的时长也算进用量，而请求里只有它的 URL，时长本地拿不到。
	// 调用方可以通过 input_video_duration 参数声明（见 inbound），声明了就用，
	// 没声明就只能是 0 —— 那一档会系统性偏低，日志里要写出来。
	InputVideoSeconds float64
	// PriceFactor 是该分辨率档相对基础档的**单价倍数**（不是 token 倍数）。
	// 1080P 及以上为 priceFactorHigh，其余为 1。
	//
	// 之所以让计费层去乘单价、而不是在这里把 Tokens 乘掉：Tokens 要如实等于
	// 上游控制台显示的用量，对账时两边必须逐位一致。系数是价格口径，不是用量。
	PriceFactor float64
}

// createReqBody 是创建任务请求体中参与估算的字段。
// 其余字段（content 里的提示词、seed、watermark…）原样透传，不在这里解析。
type createReqBody struct {
	Model           string        `json:"model"`
	Resolution      string        `json:"resolution"`
	Ratio           string        `json:"ratio"`
	Duration        int           `json:"duration"`
	FramesPerSecond int           `json:"framespersecond"`
	Content         []contentItem `json:"content"`
}

type contentItem struct {
	Type     string          `json:"type"`
	VideoURL json.RawMessage `json:"video_url"`
}

// EstimateFromBody 解析创建任务的请求体并估算用量。
//
// model 用**映射后**的模型名（meta.ActualModelName）：maxDuration 按模型代次取值，
// 而代次信息在真实模型名上，不在用户填的别名上。
//
// inputVideoSeconds 是调用方声明的参考视频总时长，0 表示未声明。
func EstimateFromBody(body []byte, model string, inputVideoSeconds float64) (Estimate, error) {
	var req createReqBody
	if err := json.Unmarshal(body, &req); err != nil {
		return Estimate{}, fmt.Errorf("seedance: 解析创建任务请求体失败: %w", err)
	}
	return EstimateFor(req, model, inputVideoSeconds), nil
}

// EstimateFor 对已解析的请求体做估算，导出出来只为让测试能直接构造入参。
func EstimateFor(req createReqBody, model string, inputVideoSeconds float64) Estimate {
	area := outputArea(req.Resolution, req.Ratio)
	fps := req.FramesPerSecond
	if fps <= 0 {
		fps = defaultFrameRate
	}
	seconds := req.Duration
	if seconds <= 0 {
		// duration = -1 表示由上游自动决定时长（文档 §1.1）。按该模型的上限估，
		// 否则会按 0 结算 —— 那等于白送。
		seconds = maxDurationFor(model)
	}

	// 声明的输入时长只在请求真的带了参考视频时才计入：上游的公式里那一项就是
	// 输入视频的时长，没有输入视频时为 0。若客户端在没带参考视频的请求上塞了
	// 一个时长，照收会凭空多扣钱。
	hasVideo := hasVideoInput(req.Content)
	inputSeconds := 0.0
	if hasVideo && inputVideoSeconds > 0 {
		inputSeconds = inputVideoSeconds
	}

	tokens := (inputSeconds + float64(seconds)) * area * float64(fps) / 1024.0
	return Estimate{
		Tokens:            int(math.Round(tokens)),
		HasVideoInput:     hasVideo,
		InputVideoSeconds: inputSeconds,
		PriceFactor:       resolutionPriceFactor(req.Resolution),
	}
}

// resolutionPriceFactor 返回该分辨率档的单价倍数。
//
// 认不出的档位按基础档算：上游拿到非法分辨率会直接拒（拒了就不计费），
// 所以这条分支实际只在"上游将来加了新档位"时才会走到。
func resolutionPriceFactor(resolution string) float64 {
	switch strings.ToLower(resolution) {
	case "1080p", "4k":
		return priceFactorHigh
	default:
		return priceFactorBase
	}
}

// outputArea 返回输出画幅面积（像素）。分辨率或宽高比认不出时按 720p / 16:9 处理：
// 那是最常见的档位，且比按 0 计费安全。
func outputArea(resolution, ratio string) float64 {
	area, ok := areaByResolution[strings.ToLower(resolution)]
	if !ok {
		area = areaByResolution["720p"]
	}
	factor, ok := ratioFactor[ratio]
	if !ok {
		factor = 1
	}
	return area * factor
}

// maxDurationFor 按模型代次取最长时长。
func maxDurationFor(model string) int {
	m := strings.ToLower(model)
	switch {
	case strings.Contains(m, "seedance-2-5"), strings.Contains(m, "seedance-2.5"):
		return maxDurationSeedance25
	case strings.Contains(m, "seedance-2-0"), strings.Contains(m, "seedance-2.0"),
		strings.Contains(m, "seedance-2"):
		return maxDurationSeedance20
	default:
		return maxDurationDefault
	}
}

// hasVideoInput 判断 content 里是否带了参考视频（type = video_url）。
func hasVideoInput(items []contentItem) bool {
	for _, item := range items {
		if item.Type == "video_url" && len(item.VideoURL) > 0 && string(item.VideoURL) != "null" {
			return true
		}
	}
	return false
}
