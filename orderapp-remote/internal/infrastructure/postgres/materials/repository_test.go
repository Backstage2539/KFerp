package materials

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeMaterialInputRejectsInvalidValues(t *testing.T) {
	_, err := normalizeMaterialInput(materialInput{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "g",
		PurchasePrice: -1,
	})
	if err == nil || !strings.Contains(err.Error(), "negative price") {
		t.Fatalf("normalizeMaterialInput() error = %v, want negative price", err)
	}
}

func TestNormalizeMaterialInputDefaultsKindAndUnit(t *testing.T) {
	got, err := normalizeMaterialInput(materialInput{Code: " m-1 ", Name: " 物料1 "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "m-1" || got.Name != "物料1" || got.Kind != "other" || got.Unit != "g" {
		t.Fatalf("normalizeMaterialInput() = %+v", got)
	}
}

func TestNormalizeMaterialInputDefaultsBatchNoToToday(t *testing.T) {
	got, err := normalizeMaterialInput(materialInput{Code: "bean-a", Name: "豆子A", Kind: "bean", Unit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	if got.BatchNo != time.Now().Format("20060102") {
		t.Fatalf("batch no = %q, want today", got.BatchNo)
	}
}

func TestNormalizeMaterialInputKeepsBeanProfileOnlyForBeans(t *testing.T) {
	got, err := normalizeMaterialInput(materialInput{
		Code:    "bean-a",
		Name:    "豆子A",
		Kind:    "bean",
		Unit:    "kg",
		Profile: &beanProfileInput{Origin: " 云南 ", Flavor: " 柑橘 "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile == nil || got.Profile.Origin != "云南" || got.Profile.Flavor != "柑橘" {
		t.Fatalf("bean profile = %+v", got.Profile)
	}

	pack, err := normalizeMaterialInput(materialInput{
		Code:    "pack-a",
		Name:    "袋子A",
		Kind:    "pack",
		Unit:    "个",
		Profile: &beanProfileInput{Flavor: "不应保留"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Profile != nil {
		t.Fatalf("pack profile = %+v, want nil", pack.Profile)
	}
}
