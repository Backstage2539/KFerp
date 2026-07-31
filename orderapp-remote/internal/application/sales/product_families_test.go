package sales

import (
	"strings"
	"testing"
)

func TestBuildOrderProductFamiliesGroupsParentSKUsAndKeepsUnpricedSpecs(t *testing.T) {
	families := BuildOrderProductFamilies([]ProductOption{
		{ID: 550, SKUID: 550, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎", ProductCode: "PARENT-550", DefaultSKUID: 552},
		{ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 227g", SKUName: "227g袋装", SKUCode: "WLG-227", SpecLabel: "227g", NetContentQty: 227, NetContentUnit: "g"},
		{ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 454g", SKUName: "454g袋装", SKUCode: "WLG-454", SpecLabel: "454g", NetContentQty: 454, NetContentUnit: "g", IsDefaultSKU: true},
		{ID: 553, SKUID: 553, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎 1Kg", SKUName: "1Kg袋装", SKUCode: "WLG-1K", SpecLabel: "1Kg", NetContentQty: 1, NetContentUnit: "kg"},
	})
	if len(families) != 1 || families[0]["name"] != "乌拉嘎" || families[0]["default_sku_id"] != int64(552) {
		t.Fatalf("families=%#v", families)
	}
	if families[0]["product_code"] != "PARENT-550" {
		t.Fatalf("family product code was overwritten by a later SKU: %#v", families[0])
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	if len(specs) != 3 {
		t.Fatalf("specs=%#v, want all three concrete SKUs including the unpriced SKU", specs)
	}
	if specs[0]["sku_id"] != int64(552) || specs[0]["spec_label"] != "454g" {
		t.Fatalf("default spec=%#v", specs[0])
	}
	if families[0]["py"] == "" || families[0]["pyi"] == "" || specs[0]["py"] == "" || specs[0]["pyi"] == "" {
		t.Fatalf("search fields missing: family=%#v spec=%#v", families[0], specs[0])
	}
}

func TestBuildOrderProductFamiliesRoutesLegacyWeightAndConcreteSKUPrices(t *testing.T) {
	families := BuildOrderProductFamilies([]ProductOption{
		{
			ID: 550, SKUID: 550, ParentProductID: 550, ParentProductName: "乌拉嘎", Name: "乌拉嘎", DefaultSKUID: 551,
			Tiers: []ProductTierOption{
				{ID: 11, SpecG: 454, MinQty: 1, UnitPrice: 118, PublicationID: 901, PublicationVersionNo: "LEGACY-V1"},
				{ID: 21, SpecG: 227, MinQty: 1, UnitPrice: 70, PublicationID: 902, PublicationVersionNo: "V2", QuantityBasis: "sales_spec_count", EffectiveSalesSpec: map[string]any{"sku_id": float64(551), "spec_label": "227g"}},
			},
		},
		{ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "227g袋装", SpecLabel: "227g", NetContentQty: 227, NetContentUnit: "g", IsDefaultSKU: true},
		{ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "454g袋装", SpecLabel: "454g", NetContentQty: 454, NetContentUnit: "g"},
	})
	if len(families) != 1 || families[0]["__order_concrete_price_family"] != true {
		t.Fatalf("families=%#v", families)
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	bySKU := map[int64]map[string]any{}
	for _, spec := range specs {
		bySKU[spec["sku_id"].(int64)] = spec
	}
	tiers227, _ := bySKU[551]["tiers"].([]map[string]any)
	tiers454, _ := bySKU[552]["tiers"].([]map[string]any)
	if len(tiers227) != 1 || tiers227[0]["publication_id"] != int64(902) || tiers227[0]["quantity_basis"] != "sales_spec_count" {
		t.Fatalf("227g tiers=%#v", tiers227)
	}
	if len(tiers454) != 1 || tiers454[0]["publication_id"] != int64(901) || tiers454[0]["spec_g"] != int64(454) {
		t.Fatalf("454g legacy tiers=%#v", tiers454)
	}
}

func TestBuildOrderProductFamiliesKeepsCustomerAliasesSeparate(t *testing.T) {
	families := BuildOrderProductFamilies([]ProductOption{
		{ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", CustomerID: 8, CustomerProductAliasID: 81, CustomerProductDisplayName: "客户甲豆 227g", SpecLabel: "227g"},
		{ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎", CustomerID: 8, CustomerProductAliasID: 81, CustomerProductDisplayName: "客户甲豆 454g", SpecLabel: "454g"},
		{ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", CustomerID: 9, CustomerProductAliasID: 82, CustomerProductDisplayName: "客户乙豆", SpecLabel: "227g"},
	})
	if len(families) != 2 || families[0]["name"] != "客户甲豆" || families[1]["name"] != "客户乙豆" {
		t.Fatalf("families=%#v", families)
	}
	if families[0]["alias_name"] != "客户甲豆" || !strings.Contains(families[0]["py"].(string), "wulaga") {
		t.Fatalf("alias and parent search fields=%#v", families[0])
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	if len(specs) != 2 {
		t.Fatalf("same alias family specs=%#v", specs)
	}
}

func TestBuildOrderProductFamiliesRemovesAliasSpecAfterCollectingWholeShuffledFamily(t *testing.T) {
	families := BuildOrderProductFamilies([]ProductOption{
		{
			ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎",
			CustomerID: 8, CustomerProductAliasID: 81, CustomerProductDisplayName: "客户乌拉嘎 454g",
			SpecLabel: "227g", NetContentQty: 227, NetContentUnit: "g",
		},
		{
			ID: 550, SKUID: 550, ParentProductID: 550, ParentProductName: "乌拉嘎",
			CustomerID: 8, CustomerProductAliasID: 81, CustomerProductDisplayName: "客户乌拉嘎 454g",
		},
		{
			ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎",
			CustomerID: 8, CustomerProductAliasID: 81, CustomerProductDisplayName: "客户乌拉嘎 454g",
			SpecLabel: "454g", NetContentQty: 454, NetContentUnit: "g",
		},
	})
	if len(families) != 1 {
		t.Fatalf("families=%#v", families)
	}
	if families[0]["name"] != "客户乌拉嘎" || families[0]["alias_name"] != "客户乌拉嘎" || families[0]["customer_product_display_name"] != "客户乌拉嘎" {
		t.Fatalf("family alias still carries a specification: %#v", families[0])
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	if len(specs) != 2 || specs[0]["spec_label"] != "227g" || specs[1]["spec_label"] != "454g" {
		t.Fatalf("specs=%#v", specs)
	}
}
