package objects

import (
	"math"
	"testing"

	model "github.com/zicorn/llm-proxy/internal/repo"
	"github.com/zicorn/llm-proxy/pkg/common/config"
)

func TestUnitQuotaRatio(t *testing.T) {
	// QuotaPerUnit = 1e6 时，比率退化为 单价 × 分组倍率
	got := unitQuotaRatio(3.0, 1.0)
	if math.Abs(got-3.0) > 1e-9 {
		t.Errorf("unitQuotaRatio(3.0, 1.0) = %v, 期望 3.0", got)
	}
	got = unitQuotaRatio(3.0, 0.5)
	if math.Abs(got-1.5) > 1e-9 {
		t.Errorf("unitQuotaRatio(3.0, 0.5) = %v, 期望 1.5", got)
	}
}

func TestMeasuredQuota(t *testing.T) {
	// 四个模态共用同一个算法，差别只在传入的计量数怎么算出来。
	// 图片：每张 ¥0.3 => 300000（¥/1M 张）；2 张 × 尺寸系数 2.0 => ¥1.2 => 1200000 额度
	if q := measuredQuota(300000, 1.0, 2*2.0); q != 1_200_000 {
		t.Errorf("图片 quota = %d, 期望 1200000", q)
	}
	// 转写：¥720/1M 秒 × 60 秒 => 43200 额度
	if q := measuredQuota(720, 1.0, 60); q != 43200 {
		t.Errorf("转写 quota = %d, 期望 43200", q)
	}
	// 微调：¥3/1M token × (3 epochs × 100000 tokens) => 900000 额度
	if q := measuredQuota(3, 1.0, 3*100000); q != 900_000 {
		t.Errorf("微调 quota = %d, 期望 900000", q)
	}
	// 分组倍率参与计算
	if q := measuredQuota(720, 0.5, 60); q != 21600 {
		t.Errorf("分组倍率 0.5 时 quota = %d, 期望 21600", q)
	}
	// 单价为 0 时不收费
	if q := measuredQuota(0, 1.0, 100); q != 0 {
		t.Errorf("单价为 0 时 quota = %d, 期望 0", q)
	}
	// 计量数为 0 时不收费
	if q := measuredQuota(720, 1.0, 0); q != 0 {
		t.Errorf("计量数为 0 时 quota = %d, 期望 0", q)
	}
	// 单价极小时至少收 1 额度，不能因取整而免费
	if q := measuredQuota(0.0001, 1.0, 1); q != 1 {
		t.Errorf("极小单价 quota = %d, 期望 1", q)
	}
}

func TestTranscriptionSecondsRounding(t *testing.T) {
	// 秒数向上取整后再计费：0.4s -> 1s，1.0s -> 1s，1.1s -> 2s
	cases := []struct {
		seconds float64
		want    int64
	}{{0.4, 720}, {1.0, 720}, {1.1, 1440}}
	for _, c := range cases {
		got := measuredQuota(720, 1.0, float64(billedSeconds(c.seconds)))
		if got != c.want {
			t.Errorf("%.1fs 的 quota = %d, 期望 %d", c.seconds, got, c.want)
		}
	}
}

func TestBilledSeconds(t *testing.T) {
	cases := []struct {
		in   float64
		want int64
	}{{0, 0}, {0.001, 1}, {0.4, 1}, {1.0, 1}, {1.1, 2}, {59.9, 60}}
	for _, c := range cases {
		if got := billedSeconds(c.in); got != c.want {
			t.Errorf("billedSeconds(%v) = %d, 期望 %d", c.in, got, c.want)
		}
	}
}

func TestUSDConversionAppliesToImage(t *testing.T) {
	// 以 USD 计价的每张价格需按汇率换算成 ¥
	origRate := config.ExchangeRate
	t.Cleanup(func() { config.ExchangeRate = origRate })
	config.ExchangeRate = 7.2
	meta := &model.ModelMeta{OutputPrice: 100000, PriceUnit: "USD"}
	_, outputCNY, _ := getModelPricesInCNY(meta)
	if math.Abs(outputCNY-720000) > 1e-6 {
		t.Errorf("USD 换算后 = %v, 期望 720000", outputCNY)
	}
}
