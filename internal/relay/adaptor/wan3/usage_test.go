package wan3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstimateUsesRequestedDuration(t *testing.T) {
	est := EstimateFor(createReqBody{
		Parameters: struct {
			Duration   int    `json:"duration"`
			Resolution string `json:"resolution"`
		}{Duration: 8, Resolution: "720P"},
	})
	assert.Equal(t, 8.0, est.Seconds)
	assert.Equal(t, "720P", est.Resolution)
	assert.False(t, est.DurationFallback)
}

// duration 缺省与 -1（智能推荐时长）都取不到真实秒数，按文档写明的默认值兜底。
func TestEstimateFallsBackWhenDurationUnsetOrAuto(t *testing.T) {
	for _, d := range []int{0, -1} {
		est := EstimateFor(createReqBody{
			Parameters: struct {
				Duration   int    `json:"duration"`
				Resolution string `json:"resolution"`
			}{Duration: d},
		})
		assert.Equal(t, float64(defaultDuration), est.Seconds, "duration=%d", d)
		assert.True(t, est.DurationFallback, "duration=%d 应标记为兜底", d)
	}
}

// 超出上限的请求上游会拒（拒了就不计费）；clamp 是为了防"上游偷偷按上限截断"时多扣。
func TestEstimateClampsToMaxDuration(t *testing.T) {
	est := EstimateFor(createReqBody{
		Parameters: struct {
			Duration   int    `json:"duration"`
			Resolution string `json:"resolution"`
		}{Duration: 600},
	})
	assert.Equal(t, float64(maxDuration), est.Seconds)
	assert.False(t, est.DurationFallback)
}

func TestEstimateResolutionDefault(t *testing.T) {
	est := EstimateFor(createReqBody{})
	assert.Equal(t, defaultResolution, est.Resolution)
}

// 百炼按输出时长计费，参考视频不额外计价（这点与 Seedance 相反），
// 所以 HasVideoInput 只影响日志措辞，不影响秒数。
func TestEstimateHasVideoInputDoesNotChangeSeconds(t *testing.T) {
	var withVideo createReqBody
	withVideo.Parameters.Duration = 5
	withVideo.Input.Media = append(withVideo.Input.Media,
		struct {
			Type string `json:"type"`
		}{Type: "reference_video"})

	plain := EstimateFor(createReqBody{})
	assert.False(t, plain.HasVideoInput)
	est := EstimateFor(withVideo)
	assert.True(t, est.HasVideoInput)
	assert.Equal(t, 5.0, est.Seconds)
}

func TestEstimateFromBodyReadsParams(t *testing.T) {
	body := []byte(`{
		"model": "wan3.0-video",
		"input": {"prompt": "a cat", "media": [{"type": "reference_image", "url": "https://e/a.jpg"}]},
		"parameters": {"resolution": "1080P", "ratio": "9:16", "duration": 12, "audio": true}
	}`)
	est, err := EstimateFromBody(body)
	require.NoError(t, err)
	assert.Equal(t, 12.0, est.Seconds)
	assert.Equal(t, "1080P", est.Resolution)
	assert.False(t, est.HasVideoInput)
	assert.False(t, est.DurationFallback)
}

func TestEstimateFromBodyRejectsBadJSON(t *testing.T) {
	_, err := EstimateFromBody([]byte("{not json"))
	assert.Error(t, err)
}

// 百炼按分辨率分三档计价，官方价目是干净的 1 : 2 : 4，基准档取 720P。
func TestResolutionFactorLadder(t *testing.T) {
	cases := []struct {
		resolution string
		wantFactor float64
	}{
		{"480P", 0.5},
		{"720P", 1.0},
		{"1080P", 2.0},
	}
	for _, tc := range cases {
		t.Run(tc.resolution, func(t *testing.T) {
			var req createReqBody
			req.Parameters.Duration = 10
			req.Parameters.Resolution = tc.resolution

			est := EstimateFor(req)
			assert.Equal(t, tc.wantFactor, est.ResolutionFactor)
			assert.Equal(t, 10.0, est.Seconds, "原始秒数不受折算影响")
			assert.Equal(t, 10.0*tc.wantFactor, est.BillableSeconds())
		})
	}
}

// 缺省分辨率是 1080P（上游默认），所以它按 2 倍计价，不是 1 倍。
func TestDefaultResolutionIs1080PAndCostsDouble(t *testing.T) {
	var req createReqBody
	req.Parameters.Duration = 5

	est := EstimateFor(req)
	assert.Equal(t, "1080P", est.Resolution)
	assert.Equal(t, 10.0, est.BillableSeconds(), "5 秒 × 2")
}

// 分辨率字面值只认大写（与上游一致）；小写是客户端传错，上游会拒，
// 这里按基准档算而不是悄悄替它兜底成别的档。
func TestUnknownResolutionFallsBackToBaseFactor(t *testing.T) {
	var req createReqBody
	req.Parameters.Duration = 5
	req.Parameters.Resolution = "720p"

	est := EstimateFor(req)
	assert.Equal(t, 1.0, est.ResolutionFactor)
	assert.Equal(t, "720p", est.Resolution, "日志里保留原值，便于看出要补档")
}
