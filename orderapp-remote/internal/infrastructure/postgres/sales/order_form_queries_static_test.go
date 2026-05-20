package sales

import (
	"encoding/json"
	salesapp "orderapp/internal/application/sales"
	"os"
	"strings"
	"testing"
)

func TestOrderFormProductQueryKeepsRoastLevelAndProductKindScanShape(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "SELECT id, name, COALESCE(roast_level,'')") {
		t.Fatalf("order form product query must select roast_level as the third product column before default_price")
	}
	if strings.Contains(text, "SELECT id, name, COALESCE(NULLIF(product_kind,''),'roasted')") {
		t.Fatalf("order form product query must not scan product_kind into ProductOption.RoastLevel")
	}
}

func TestOrderFormProductQueryDoesNotExposeBoundRoastedTiersForGreenBeanProducts(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"green_bound_tiers",
		"green_bean_bound_roasted_tier",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("order form product tiers must not expose bound roasted tiers for green bean products; found %q", forbidden)
		}
	}
}

func TestOrderSaveRejectsMissingGreenBeanListPriceWithoutBoundRoastedFallback(t *testing.T) {
	source, err := os.ReadFile("repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	text := string(source)
	for _, forbidden := range []string{
		"greenBeanOrderPriceProductIDTx",
		"greenBeanBoundRoastedTierPriceSourceJSON",
		"green_bean_bound_roasted_tier",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("green bean order save must not fall back to bound roasted tiers; found %q", forbidden)
		}
	}
	if !strings.Contains(text, "missing green bean list price") && !strings.Contains(text, "缺少生豆豆单价格") {
		t.Fatalf("green bean order save must return an explicit missing green bean list price error")
	}
}

func TestOrderFormBeanListVersionOptionsArePartitionedByListType(t *testing.T) {
	source, err := os.ReadFile("order_form_queries.go")
	if err != nil {
		t.Fatalf("read order_form_queries.go: %v", err)
	}
	text := string(source)
	for _, want := range []string{
		"b.list_type",
		"PARTITION BY c.id, b.list_type",
		"&row.ListType",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("order form bean-list versions must be grouped by customer and list type; missing %q", want)
		}
	}
}

func TestGreenBeanOrderPublicationTiersParseTemplatePrices(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":416,
				"name":"曲奇拼配2.0",
				"green_bean_sale_tiers":[
					{"label":"1KG","spec_g":1000,"min_qty":1,"max_qty":59,"price_per_unit":63.9,"template_id":5,"template_tier_id":50,"display_unit":"kg"},
					{"label":"60kG","spec_g":1000,"min_qty":60,"price_per_unit":63.9,"template_id":5,"template_tier_id":51,"display_unit":"kg"}
				]
			}]
		}]
	}`)

	tiersByProduct := greenBeanOrderTierMapFromPublicationContent(12, "V3.0.5", content)
	tiers := tiersByProduct[416]
	if len(tiers) != 2 {
		t.Fatalf("green bean tiers = %+v, want 2 tiers", tiers)
	}
	if tiers[0].SpecG != 1000 || tiers[0].MinQty != 1 || tiers[0].MaxQty == nil || *tiers[0].MaxQty != 59 || tiers[0].UnitPrice != 63.9 {
		t.Fatalf("first green bean tier = %+v", tiers[0])
	}
	if tiers[0].ProductKind != "green_bean" {
		t.Fatalf("product_kind = %q, want green_bean", tiers[0].ProductKind)
	}
	if tiers[0].DisplayUnit != "kg" || tiers[1].DisplayUnit != "kg" {
		t.Fatalf("display_unit = %q/%q, want kg/kg", tiers[0].DisplayUnit, tiers[1].DisplayUnit)
	}
	var source map[string]any
	if err := json.Unmarshal([]byte(tiers[0].PriceSourceJSON), &source); err != nil {
		t.Fatalf("price source json invalid: %v", err)
	}
	if source["list_type"] != "green" || int64(source["publication_id"].(float64)) != 12 || int64(source["template_id"].(float64)) != 5 {
		t.Fatalf("price source = %+v", source)
	}
}

func TestApplyGreenBeanOrderPublicationTiersReplacesDirectProductTiers(t *testing.T) {
	products := []salesapp.ProductOption{
		{
			ID:          416,
			Name:        "曲奇拼配2.0",
			ProductKind: "green_bean",
			Tiers: []salesapp.ProductTierOption{{
				ID:          460,
				SpecG:       454,
				MinQty:      2,
				UnitPrice:   63,
				ProductKind: "roasted_bean",
			}},
		},
		{
			ID:          417,
			Name:        "曲奇拼配2.0",
			ProductKind: "roasted",
			Tiers: []salesapp.ProductTierOption{{
				ID:          461,
				SpecG:       454,
				MinQty:      14,
				UnitPrice:   56,
				ProductKind: "roasted_bean",
			}},
		},
	}
	publicationTiers := map[int64][]salesapp.ProductTierOption{
		416: {{
			ID:          50,
			SpecG:       1000,
			MinQty:      1,
			UnitPrice:   63.9,
			ProductKind: "green_bean",
		}},
	}

	applyGreenBeanOrderPublicationTiers(products, publicationTiers)

	if len(products[0].Tiers) != 1 || products[0].Tiers[0].SpecG != 1000 || products[0].Tiers[0].UnitPrice != 63.9 || products[0].Tiers[0].ProductKind != "green_bean" {
		t.Fatalf("green bean product tiers = %+v", products[0].Tiers)
	}
	if len(products[1].Tiers) != 1 || products[1].Tiers[0].ID != 461 || products[1].Tiers[0].UnitPrice != 56 {
		t.Fatalf("roasted product tiers changed = %+v", products[1].Tiers)
	}
}
