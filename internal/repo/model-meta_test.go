package model

import "testing"

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
