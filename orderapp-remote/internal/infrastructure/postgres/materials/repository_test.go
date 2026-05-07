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

func TestNormalizeMaterialInputAcceptsLegacyKindAliases(t *testing.T) {
	bean, err := normalizeMaterialInput(materialInput{Code: "raw-a", Name: "旧生豆", Kind: " raw_bean ", Unit: "kg"})
	if err != nil {
		t.Fatalf("normalize raw_bean alias: %v", err)
	}
	if bean.Kind != "bean" {
		t.Fatalf("raw_bean kind = %q, want bean", bean.Kind)
	}

	pack, err := normalizeMaterialInput(materialInput{Code: "bag-a", Name: "旧包材", Kind: "packaging", Unit: "个"})
	if err != nil {
		t.Fatalf("normalize packaging alias: %v", err)
	}
	if pack.Kind != "pack" {
		t.Fatalf("packaging kind = %q, want pack", pack.Kind)
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
		Code:        "bean-a",
		Name:        "豆子A",
		Kind:        "bean",
		Unit:        "kg",
		Profile:     &beanProfileInput{Origin: " 云南 ", Flavor: " 柑橘 "},
		PackProfile: &packProfileInput{SizeSpec: "不应保留"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Profile == nil || got.Profile.Origin != "云南" || got.Profile.Flavor != "柑橘" {
		t.Fatalf("bean profile = %+v", got.Profile)
	}
	if got.PackProfile != nil {
		t.Fatalf("bean pack profile = %+v, want nil", got.PackProfile)
	}

	pack, err := normalizeMaterialInput(materialInput{
		Code:        "pack-a",
		Name:        "袋子A",
		Kind:        "pack",
		Unit:        "个",
		Profile:     &beanProfileInput{Flavor: "不应保留"},
		PackProfile: &packProfileInput{SizeSpec: " 227g ", Dimensions: " 12x20cm "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.Profile != nil {
		t.Fatalf("pack profile = %+v, want nil", pack.Profile)
	}
	if pack.PackProfile == nil || pack.PackProfile.SizeSpec != "227g" || pack.PackProfile.Dimensions != "12x20cm" {
		t.Fatalf("pack profile = %+v", pack.PackProfile)
	}
}

func TestAssertImmutableMaterialFieldsRejectsChangedBaseFields(t *testing.T) {
	old := materialRow{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "g",
		BatchNo:       "20260427",
		PurchasePrice: 88,
		SalePrice:     99,
	}
	next := materialInput{
		Code:          "bean-a",
		Name:          "豆子A新",
		Kind:          "bean",
		Unit:          "g",
		BatchNo:       "20260427",
		PurchasePrice: 88,
		SalePrice:     99,
	}
	err := assertImmutableMaterialFields(old, next)
	if err == nil || !strings.Contains(err.Error(), "copy material") {
		t.Fatalf("assertImmutableMaterialFields() error = %v, want copy material", err)
	}
}

func TestAssertImmutableMaterialFieldsRejectsInlineStockChange(t *testing.T) {
	old := materialRow{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "g",
		BatchNo:       "20260427",
		PurchasePrice: 88,
		SalePrice:     99,
		OnhandG:       1000,
		OnhandUnits:   2,
	}
	next := materialInput{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "g",
		BatchNo:       "20260427",
		PurchasePrice: 88,
		SalePrice:     99,
		OnhandG:       1200,
		OnhandUnits:   2,
	}
	err := assertImmutableMaterialFields(old, next)
	if err == nil || !strings.Contains(err.Error(), "stock adjustment") {
		t.Fatalf("assertImmutableMaterialFields() error = %v, want stock adjustment", err)
	}
}
