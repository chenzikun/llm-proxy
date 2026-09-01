package controller

import (
	"testing"

	model "github.com/zicorn/llm-proxy/internal/repo"
)

func TestValidateModelMetaBillingUnits(t *testing.T) {
	metas := []model.ModelMeta{
		{Model: "m1", BillingUnit: ""},
		{Model: "m2", BillingUnit: model.BillingUnitChar},
		{Model: "", BillingUnit: "invalid-should-skip"},
	}
	if invalidModel, invalidUnit, ok := validateModelMetaBillingUnits(metas); !ok {
		t.Fatalf("validateModelMetaBillingUnits() = (%q, %q, false), want ok", invalidModel, invalidUnit)
	}
	if metas[0].BillingUnit != model.BillingUnitToken {
		t.Errorf("空 billing_unit 应默认 token，got %q", metas[0].BillingUnit)
	}

	metas = []model.ModelMeta{{Model: "bad", BillingUnit: "minute"}}
	invalidModel, invalidUnit, ok := validateModelMetaBillingUnits(metas)
	if ok {
		t.Fatal("非法 billing_unit 应返回 ok=false")
	}
	if invalidModel != "bad" || invalidUnit != "minute" {
		t.Errorf("got (%q, %q), want (bad, minute)", invalidModel, invalidUnit)
	}
}
