package materials

import (
	"os"
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
	if got.Code != "m-1" || got.Name != "物料1" || got.Kind != "other" || got.Unit != "g" || got.CostUnit != "kg" {
		t.Fatalf("normalizeMaterialInput() = %+v", got)
	}
}

func TestNormalizeMaterialInputUsesKgCostUnitForWeightAndInventoryUnitForDiscrete(t *testing.T) {
	weight, err := normalizeMaterialInput(materialInput{Code: "bean-cost", Name: "重量物料", Unit: "g", CostUnit: "kg"})
	if err != nil {
		t.Fatal(err)
	}
	if weight.CostUnit != "kg" {
		t.Fatalf("weight cost unit = %q, want kg", weight.CostUnit)
	}
	discrete, err := normalizeMaterialInput(materialInput{Code: "pack-cost", Name: "计件物料", Unit: "个"})
	if err != nil {
		t.Fatal(err)
	}
	if discrete.CostUnit != "个" {
		t.Fatalf("discrete cost unit = %q, want 个", discrete.CostUnit)
	}
	if _, err := normalizeMaterialInput(materialInput{Code: "bad-cost", Name: "错误计价", Unit: "g", CostUnit: "g"}); err == nil || !strings.Contains(err.Error(), "重量物料成本计价单位必须为 kg") {
		t.Fatalf("invalid weight cost unit error = %v", err)
	}
}

func TestMaterialInventoryUnitIsLockedAfterCreate(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"assertMaterialInventoryUnitReadOnly",
		"requestedInventoryUnit",
		"库存单位保存后不能修改",
		"old.Unit",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("material inventory unit lock missing marker %q", want)
		}
	}
}

func TestMaterialCostUnitIsLockedAfterCreate(t *testing.T) {
	repository, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(repository)
	for _, want := range []string{
		"assertMaterialCostUnitReadOnly",
		"requestedCostUnit",
		"成本计价单位保存后不能修改",
		"old.CostUnit",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("material cost unit lock missing marker %q", want)
		}
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

func TestAssertMaterialStockFieldsReadOnlyAllowsBaseFieldEdits(t *testing.T) {
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
		Unit:          "kg",
		BatchNo:       "20260427",
		PurchasePrice: 90,
		SalePrice:     99,
	}
	err := assertMaterialStockFieldsReadOnly(old, next)
	if err != nil {
		t.Fatalf("assertMaterialStockFieldsReadOnly() error = %v, want base fields editable", err)
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
	err := assertMaterialStockFieldsReadOnly(old, next)
	if err == nil || !strings.Contains(err.Error(), "stock adjustment") {
		t.Fatalf("assertMaterialStockFieldsReadOnly() error = %v, want stock adjustment", err)
	}
}

func TestNormalizeMaterialInputCarriesIndustryTemplateAndFields(t *testing.T) {
	got, err := normalizeMaterialInput(materialInput{
		Code:                    "bean-industry",
		Name:                    "行业字段豆",
		Unit:                    "kg",
		IndustryFieldTemplateID: 7,
		IndustryFields: []materialIndustryFieldInput{
			{FieldKey: " 产地 ", ValueText: " 云南 "},
			{FieldKey: "", ValueText: "忽略"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.IndustryFieldTemplateID != 7 || len(got.IndustryFields) != 1 || got.IndustryFields[0].FieldKey != "产地" || got.IndustryFields[0].ValueText != "云南" {
		t.Fatalf("industry fields = %+v template=%d", got.IndustryFields, got.IndustryFieldTemplateID)
	}
}
