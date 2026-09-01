package model

import (
	"fmt"
	"testing"

	"github.com/zicorn/llm-proxy/pkg/common/helper"
)

func requireDB(t *testing.T) {
	t.Helper()
	if DB == nil {
		t.Skip("repo 集成测试需要数据库，当前 DB 未初始化")
	}
	sqlDB, err := DB.DB()
	if err != nil {
		t.Skipf("repo 集成测试需要数据库，无法获取连接: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Skipf("repo 集成测试需要数据库，Ping 失败: %v", err)
	}
}

func TestIsValidBillingUnit(t *testing.T) {
	valid := []string{BillingUnitToken, BillingUnitChar, BillingUnitSecond, BillingUnitImage}
	for _, u := range valid {
		if !IsValidBillingUnit(u) {
			t.Errorf("IsValidBillingUnit(%q) = false, 期望 true", u)
		}
	}
	invalid := []string{"", "minute", "TOKEN", "images", "秒"}
	for _, u := range invalid {
		if IsValidBillingUnit(u) {
			t.Errorf("IsValidBillingUnit(%q) = true, 期望 false", u)
		}
	}
}

func TestCreateOrUpdateModelMeta_UpdatesBillingUnit(t *testing.T) {
	requireDB(t)
	modelName := fmt.Sprintf("test-billing-unit-%d", helper.GetTimestamp())
	initial := &ModelMeta{
		Model:       modelName,
		ChannelType: 1,
		Status:      ModelStatusOn,
		BillingUnit: BillingUnitToken,
		PriceUnit:   "CNY",
	}
	if err := AddModelMeta(initial); err != nil {
		t.Fatalf("AddModelMeta: %v", err)
	}
	t.Cleanup(func() {
		_ = (&ModelMeta{Id: initial.Id}).Delete()
	})

	updated := &ModelMeta{
		Model:       modelName,
		ChannelType: 1,
		BillingUnit: BillingUnitImage,
		PriceUnit:   "CNY",
	}
	if err := CreateOrUpdateModelMeta(updated); err != nil {
		t.Fatalf("CreateOrUpdateModelMeta: %v", err)
	}

	got, err := GetModelMetaByModel(modelName)
	if err != nil {
		t.Fatalf("GetModelMetaByModel: %v", err)
	}
	if got.BillingUnit != BillingUnitImage {
		t.Errorf("BillingUnit = %q, want %q", got.BillingUnit, BillingUnitImage)
	}
}
