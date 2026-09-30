package seedance

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 官方公布的 token/秒（24fps，16:9）。公式若被改动，这四条会先炸。
func TestTokensPerSecondMatchesPublishedRates(t *testing.T) {
	cases := []struct {
		resolution string
		wantPerSec int
	}{
		{"480p", 9608},
		{"720p", 21600},
		{"1080p", 48600},
		{"4k", 194400},
	}
	for _, tc := range cases {
		t.Run(tc.resolution, func(t *testing.T) {
			got := EstimateFor(createReqBody{
				Resolution: tc.resolution, Ratio: "16:9", Duration: 1,
			}, "doubao-seedance-2-5-pro-250528", 0)
			assert.Equal(t, tc.wantPerSec, got.Tokens)
		})
	}
}

// 横竖屏只差一次旋转，面积相同 ⇒ token 数必须相同。
func TestOrientationDoesNotChangeTokens(t *testing.T) {
	h := EstimateFor(createReqBody{Resolution: "720p", Ratio: "16:9", Duration: 5}, "", 0)
	v := EstimateFor(createReqBody{Resolution: "720p", Ratio: "9:16", Duration: 5}, "", 0)
	assert.Equal(t, h.Tokens, v.Tokens)
}

// 短边固定：720p 下 1:1 是 720×720，面积是 16:9 的 9/16。
func TestSquareRatioScalesArea(t *testing.T) {
	square := EstimateFor(createReqBody{Resolution: "720p", Ratio: "1:1", Duration: 1}, "", 0)
	widescreen := EstimateFor(createReqBody{Resolution: "720p", Ratio: "16:9", Duration: 1}, "", 0)
	assert.Equal(t, 21600*9/16, square.Tokens)
	assert.Equal(t, 21600, widescreen.Tokens)
}

// duration = -1（上游自动决定时长）不能按 0 结算，要按该模型的上限估。
func TestAutoDurationFallsBackToModelMax(t *testing.T) {
	// 2.5 Pro 上限 30s，720p/24fps ⇒ 21600 × 30
	got := EstimateFor(createReqBody{Resolution: "720p", Ratio: "16:9", Duration: -1},
		"doubao-seedance-2-5-pro-250528", 0)
	assert.Equal(t, 21600*30, got.Tokens)

	// 2.0 系列上限 15s
	got = EstimateFor(createReqBody{Resolution: "720p", Ratio: "16:9", Duration: -1},
		"doubao-seedance-2-0-pro-250415", 0)
	assert.Equal(t, 21600*15, got.Tokens)

	// 认不出的模型走兜底（1.x 的 12s）
	got = EstimateFor(createReqBody{Resolution: "720p", Ratio: "16:9", Duration: -1},
		"some-unknown-model", 0)
	assert.Equal(t, 21600*12, got.Tokens)
}

// 2.5 存在 doubao- 与 dreamina- 两套命名，前缀匹配必须都能认出代次。
func TestMaxDurationMatchesBothNamingSchemes(t *testing.T) {
	for _, model := range []string{
		"doubao-seedance-2-5-pro-250528",
		"dreamina-seedance-2-5-260628",
	} {
		assert.Equal(t, maxDurationSeedance25, maxDurationFor(model), model)
	}
	for _, model := range []string{
		"doubao-seedance-2-0-pro-250415",
		"dreamina-seedance-2-0-260128",
	} {
		assert.Equal(t, maxDurationSeedance20, maxDurationFor(model), model)
	}
}

// 请求体里的 framespersecond 优先于默认的 24。
func TestFrameRateFromRequestWins(t *testing.T) {
	got := EstimateFor(createReqBody{
		Resolution: "720p", Ratio: "16:9", Duration: 1, FramesPerSecond: 48,
	}, "", 0)
	assert.Equal(t, 21600*2, got.Tokens)
}

// 认不出的分辨率/宽高比按 720p / 16:9 兜底，不能算出 0。
func TestUnknownResolutionAndRatioFallBack(t *testing.T) {
	got := EstimateFor(createReqBody{Resolution: "8k", Ratio: "3:2", Duration: 1}, "", 0)
	assert.Equal(t, 21600, got.Tokens)
}

// 带参考视频时标记出来（上游会把输入视频时长也计费，本地估不出那段）。
func TestHasVideoInput(t *testing.T) {
	noVideo := EstimateFor(createReqBody{
		Resolution: "720p", Duration: 1,
		Content: []contentItem{{Type: "text"}, {Type: "image_url"}},
	}, "", 0)
	assert.False(t, noVideo.HasVideoInput)

	withVideo := EstimateFor(createReqBody{
		Resolution: "720p", Duration: 1,
		Content: []contentItem{{Type: "video_url", VideoURL: []byte(`{"url":"https://x/v.mp4"}`)}},
	}, "", 0)
	assert.True(t, withVideo.HasVideoInput)
}

func TestEstimateFromBodyRejectsBadJSON(t *testing.T) {
	_, err := EstimateFromBody([]byte("{not json"), "", 0)
	assert.Error(t, err)
}

func TestEstimateFromBodyReadsParams(t *testing.T) {
	body := []byte(`{
		"model": "doubao-seedance-2-5-pro-250528",
		"content": [{"type":"text","text":"a cat"}],
		"resolution": "1080p",
		"ratio": "16:9",
		"duration": 5
	}`)
	got, err := EstimateFromBody(body, "doubao-seedance-2-5-pro-250528", 0)
	assert.NoError(t, err)
	assert.Equal(t, 48600*5, got.Tokens)
	assert.False(t, got.HasVideoInput)
}

// 上游的公式要把输入视频的时长也加上。一组真实用量（720p 5s 出片 + 5s 参考视频
// 实测 217k tokens）反过来验证这一项确实存在，而不是我们凭空补的。
func TestDeclaredInputVideoDurationIsCounted(t *testing.T) {
	got := EstimateFor(createReqBody{
		Resolution: "720p", Ratio: "16:9", Duration: 5,
		Content: []contentItem{{Type: "text"}, {Type: "video_url", VideoURL: []byte(`{"url":"https://x/r.mp4"}`)}},
	}, "doubao-seedance-2-5-pro-250528", 5)

	// (5 输入 + 5 输出) × 21600 = 216000，官方实测 217k（含少量开销）
	assert.Equal(t, 21600*10, got.Tokens)
	assert.Equal(t, 5.0, got.InputVideoSeconds)

	// 不算输入时长的话只有一半 —— 这正是"不声明就少扣一半"的量级
	without := EstimateFor(createReqBody{
		Resolution: "720p", Ratio: "16:9", Duration: 5,
		Content: []contentItem{{Type: "video_url", VideoURL: []byte(`{"url":"https://x/r.mp4"}`)}},
	}, "doubao-seedance-2-5-pro-250528", 0)
	assert.Equal(t, 21600*5, without.Tokens)
	assert.Equal(t, 0.0, without.InputVideoSeconds)
}

// 没有参考视频时，上游公式里那一项就是 0；此时若还照收声明的时长，
// 等于对一个文生视频请求凭空多扣钱。
func TestInputVideoDurationIgnoredWithoutReferenceVideo(t *testing.T) {
	got := EstimateFor(createReqBody{
		Resolution: "720p", Ratio: "16:9", Duration: 5,
		Content: []contentItem{{Type: "text"}},
	}, "", 30)

	assert.Equal(t, 21600*5, got.Tokens)
	assert.False(t, got.HasVideoInput)
	assert.Equal(t, 0.0, got.InputVideoSeconds)
}
