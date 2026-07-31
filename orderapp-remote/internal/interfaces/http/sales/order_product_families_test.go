package sales

import (
	"encoding/json"
	"testing"
)

func TestAPIProductFamiliesAggregatePricedConcreteSKUsUnderParent(t *testing.T) {
	products := []ProductOption{
		{
			ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "227g袋装", SpecLabel: "227g",
			NetContentQty: 227, NetContentUnit: "g", IsDefaultSKU: true, Name: "乌拉嘎 227g", ProductRecordName: "乌拉嘎",
			ProductKind: "roasted_bean", OrderUnit: "袋",
			Tiers: []ProductTierOption{{
				ID: 11, MinQty: 1, UnitPrice: 68, PublicationID: 901, PublicationVersionNo: "V1", ListType: "commercial",
				QuantityBasis: "sales_spec_count", EffectiveSalesSpec: map[string]any{
					"sku_id": float64(551), "spec_name": "227g袋装", "spec_label": "227g", "sales_unit": "袋", "net_content_qty": float64(227), "net_content_unit": "g",
				},
			}},
		},
		{
			ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "454g袋装", SpecLabel: "454g",
			NetContentQty: 454, NetContentUnit: "g", Name: "乌拉嘎 454g", ProductRecordName: "乌拉嘎",
			ProductKind: "roasted_bean", OrderUnit: "袋",
			Tiers: []ProductTierOption{{
				ID: 12, MinQty: 1, UnitPrice: 118, PublicationID: 901, PublicationVersionNo: "V1", ListType: "commercial",
				QuantityBasis: "sales_spec_count", EffectiveSalesSpec: map[string]any{
					"sku_id": float64(552), "spec_name": "454g袋装", "spec_label": "454g", "sales_unit": "袋", "net_content_qty": float64(454), "net_content_unit": "g",
				},
			}},
		},
		{
			ID: 553, SKUID: 553, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "1Kg袋装", SpecLabel: "1Kg",
			NetContentQty: 1, NetContentUnit: "kg", Name: "乌拉嘎 1Kg", ProductRecordName: "乌拉嘎", ProductKind: "roasted_bean",
		},
	}

	families := apiProductFamilies(products)
	encoded, err := json.Marshal(families)
	if err != nil {
		t.Fatal(err)
	}
	var got []struct {
		ParentProductID   int64  `json:"parent_product_id"`
		ParentProductName string `json:"parent_product_name"`
		Name              string `json:"name"`
		Specs             []struct {
			SKUID                int64   `json:"sku_id"`
			SKUName              string  `json:"sku_name"`
			SpecLabel            string  `json:"spec_label"`
			NetContentQty        float64 `json:"net_content_qty"`
			NetContentUnit       string  `json:"net_content_unit"`
			IsDefaultSKU         bool    `json:"is_default_sku"`
			PublicationIDs       []int64 `json:"publication_ids"`
			DefaultPublicationID int64   `json:"default_publication_id"`
			Tiers                []struct {
				PublicationID int64  `json:"publication_id"`
				ListType      string `json:"list_type"`
				VersionNo     string `json:"publication_version_no"`
			} `json:"tiers"`
		} `json:"specs"`
	}
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode families: %v; json=%s", err, encoded)
	}
	if len(got) != 1 || got[0].ParentProductID != 550 || got[0].ParentProductName != "乌拉嘎" || got[0].Name != "乌拉嘎" {
		t.Fatalf("families = %+v", got)
	}
	if len(got[0].Specs) != 3 {
		t.Fatalf("family specs = %+v, want all concrete SKUs including the unpriced SKU", got[0].Specs)
	}
	first := got[0].Specs[0]
	if first.SKUID != 551 || first.SKUName != "227g袋装" || first.SpecLabel != "227g" || first.NetContentQty != 227 || first.NetContentUnit != "g" || !first.IsDefaultSKU {
		t.Fatalf("first spec = %+v", first)
	}
	if len(first.PublicationIDs) != 1 || first.PublicationIDs[0] != 901 || first.DefaultPublicationID != 901 || len(first.Tiers) != 1 || first.Tiers[0].PublicationID != 901 || first.Tiers[0].ListType != "commercial" || first.Tiers[0].VersionNo != "V1" {
		t.Fatalf("first spec publication identity = %+v", first)
	}
}

func TestAPIProductFamiliesKeepPureLegacyPublicationTierOnItsConcreteSpec(t *testing.T) {
	products := []ProductOption{{
		ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "227g袋装", SpecLabel: "227g",
		Name: "乌拉嘎 227g", ProductRecordName: "乌拉嘎", ProductKind: "roasted_bean",
		Tiers: []ProductTierOption{{
			ID: 11, SpecG: 227, MinQty: 1, UnitPrice: 68, PublicationID: 901, PublicationVersionNo: "LEGACY-V1", ListType: "commercial",
		}},
	}}

	families := apiProductFamilies(products)
	if len(families) != 1 {
		t.Fatalf("pure legacy publication family = %#v", families)
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	if len(specs) != 1 {
		t.Fatalf("pure legacy specs = %#v", specs)
	}
	tiers, _ := specs[0]["tiers"].([]map[string]any)
	if len(tiers) != 1 || tiers[0]["publication_id"] != int64(901) || tiers[0]["spec_g"] != int64(227) {
		t.Fatalf("pure legacy tiers = %#v", tiers)
	}
}

func TestAPIProductFamiliesKeepConcreteAndLegacyTiersFromMixedProductHistory(t *testing.T) {
	products := []ProductOption{{
		ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "227g袋装", SpecLabel: "227g",
		Name: "乌拉嘎 227g", ProductRecordName: "乌拉嘎", ProductKind: "roasted_bean",
		Tiers: []ProductTierOption{
			{
				ID: 21, MinQty: 1, UnitPrice: 70, PublicationID: 902, PublicationVersionNo: "V2", ListType: "commercial",
				QuantityBasis: "sales_spec_count", EffectiveSalesSpec: map[string]any{
					"sku_id": float64(551), "spec_label": "227g", "sales_unit": "袋",
				},
			},
			{ID: 11, SpecG: 227, MinQty: 1, UnitPrice: 68, PublicationID: 901, PublicationVersionNo: "LEGACY-V1", ListType: "commercial"},
		},
	}}

	families := apiProductFamilies(products)
	if len(families) != 1 {
		t.Fatalf("mixed product families = %#v, want one concrete family", families)
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	if len(specs) != 1 {
		t.Fatalf("mixed product specs = %#v", specs)
	}
	tiers, _ := specs[0]["tiers"].([]map[string]any)
	if len(tiers) != 2 || tiers[0]["publication_id"] != int64(902) || tiers[1]["publication_id"] != int64(901) {
		t.Fatalf("mixed product tiers = %#v, want V2 concrete and legacy V1 histories", tiers)
	}
}

func TestAPIProductFamiliesUseEffectiveParentCustomerAliasWithoutAppendingSpec(t *testing.T) {
	products := []ProductOption{{
		ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "227g袋装", SpecLabel: "227g",
		Name: "客户乌拉嘎", ProductRecordName: "乌拉嘎", CustomerID: 3, CustomerProductAliasID: 81,
		CustomerProductDisplayName: "客户乌拉嘎", CustomerItemCode: "C-WLG", ProductKind: "roasted_bean",
		Tiers: []ProductTierOption{{
			ID: 11, MinQty: 1, UnitPrice: 68, PublicationID: 901, PublicationVersionNo: "V1", ListType: "commercial",
			QuantityBasis: "sales_spec_count", EffectiveSalesSpec: map[string]any{
				"sku_id": float64(551), "spec_label": "227g", "sales_unit": "袋",
			},
		}},
	}}

	families := apiProductFamilies(products)
	encoded, err := json.Marshal(families)
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["name"] != "客户乌拉嘎" || got[0]["parent_product_name"] != "乌拉嘎" || got[0]["customer_product_alias_id"] != float64(81) {
		t.Fatalf("aliased family = %#v", got)
	}
	if got[0]["name"] == "客户乌拉嘎 227g" {
		t.Fatalf("family display name must not append the selected specification: %#v", got[0])
	}
}

func TestOrderItemMappingAcceptsItemParentProductID(t *testing.T) {
	items := orderItemCommandsFromCreateRequest(CreateOrderRequest{
		ProductID:           []string{"551"},
		ParentProductID:     []string{"999"},
		ItemParentProductID: []string{"550"},
		ItemName:            []string{"乌拉嘎"},
		Qty:                 []string{"2"},
		Spec:                []string{"227"},
	})
	if len(items) != 1 || items[0].ProductID == nil || *items[0].ProductID != 551 || items[0].ParentProductID != 550 {
		t.Fatalf("mapped order item = %+v", items)
	}
}

func TestAPIProductFamiliesKeepPublicationSpecificSKUAvailabilityAcrossVersions(t *testing.T) {
	products := []ProductOption{
		{
			ID: 551, SKUID: 551, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "227g袋装", SpecLabel: "227g", Name: "乌拉嘎 227g",
			Tiers: []ProductTierOption{
				{ID: 21, UnitPrice: 70, PublicationID: 902, PublicationVersionNo: "V2", ListType: "commercial", QuantityBasis: "sales_spec_count", EffectiveSalesSpec: map[string]any{"sku_id": float64(551), "spec_label": "227g", "sales_unit": "袋"}},
				{ID: 11, UnitPrice: 68, PublicationID: 901, PublicationVersionNo: "V1", ListType: "commercial", QuantityBasis: "sales_spec_count", EffectiveSalesSpec: map[string]any{"sku_id": float64(551), "spec_label": "227g", "sales_unit": "袋"}},
			},
		},
		{
			ID: 552, SKUID: 552, ParentProductID: 550, ParentProductName: "乌拉嘎", SKUName: "454g袋装", SpecLabel: "454g", Name: "乌拉嘎 454g",
			Tiers: []ProductTierOption{{ID: 22, UnitPrice: 118, PublicationID: 902, PublicationVersionNo: "V2", ListType: "commercial", QuantityBasis: "sales_spec_count", EffectiveSalesSpec: map[string]any{"sku_id": float64(552), "spec_label": "454g", "sales_unit": "袋"}}},
		},
	}
	families := apiProductFamilies(products)
	if len(families) != 1 {
		t.Fatalf("families = %#v", families)
	}
	specs, _ := families[0]["specs"].([]map[string]any)
	if len(specs) != 2 {
		t.Fatalf("family specs = %#v", specs)
	}
	bySKU := map[int64]map[string]any{}
	for _, spec := range specs {
		bySKU[spec["sku_id"].(int64)] = spec
	}
	firstPublications, _ := bySKU[551]["publication_ids"].([]int64)
	secondPublications, _ := bySKU[552]["publication_ids"].([]int64)
	if len(firstPublications) != 2 || firstPublications[0] != 902 || firstPublications[1] != 901 {
		t.Fatalf("SKU 551 publication ids = %#v", firstPublications)
	}
	if len(secondPublications) != 1 || secondPublications[0] != 902 {
		t.Fatalf("SKU 552 publication ids = %#v", secondPublications)
	}
}
