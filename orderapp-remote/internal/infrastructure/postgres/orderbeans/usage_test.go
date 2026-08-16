package orderbeans

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPublishedUnitPriceFromContentMatchesGreenBeanTiers(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":88,
				"name":"巴拿马生豆",
				"green_bean_sale_tiers":[
					{"label":"1-23kg","spec_g":1000,"min_qty":1,"max_qty":23,"price_per_unit":128,"price_per_lb":58.112,"display_unit":"kg"},
					{"label":"24kg+","spec_g":1000,"min_qty":24,"price_per_unit":118,"price_per_lb":53.572,"display_unit":"kg"}
				]
			}]
		}]
	}`)

	got, ok := publishedUnitPriceFromContent(content, 88, 1000, 2)
	if !ok || got != 128 {
		t.Fatalf("published price for 2kg = %.2f/%v, want 128/true", got, ok)
	}
	got, ok = publishedUnitPriceFromContent(content, 88, 1000, 30)
	if !ok || got != 118 {
		t.Fatalf("published price for 30kg = %.2f/%v, want 118/true", got, ok)
	}
}

func TestInspectPublishedProductSpecRequiresConcreteSKUAndFreezesSalesSpec(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":68,
			 "inventory_unit":"kg","inventory_conversion_json":{"袋":{"kg":0.227}},
			 "effective_sales_spec":{"sku_id":551,"spec_key":"bag-227g","spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}},
			{"product_id":550,"sku_id":552,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":118,
			 "effective_sales_spec":{"sku_id":552,"spec_key":"bag-454g","spec_name":"454g袋装","spec_label":"454g","sales_unit":"袋","net_content_qty":454,"net_content_unit":"g"}}
		]
	}`)

	got, err := inspectPublishedProductSpecContent(content, 551)
	if err != nil {
		t.Fatalf("inspect concrete publication: %v", err)
	}
	if !got.ConcretePublication || !got.ProductFound || got.SKUID != 551 || got.ParentProductID != 550 || got.SpecName != "227g袋装" || got.SpecLabel != "227g" || got.SalesUnit != "袋" || got.NetContentQty != 227 || got.NetContentUnit != "g" {
		t.Fatalf("concrete product spec = %+v", got)
	}
	if got.InventoryUnit != "kg" || !strings.Contains(got.InventoryConversionJSON, `"袋":{"kg":0.227}`) {
		t.Fatalf("typed concrete product inventory conversion = %+v", got)
	}
	for _, want := range []string{`"parent_product_id":550`, `"inventory_unit":"kg"`, `"inventory_conversion_json":{"袋":{"kg":0.227}}`} {
		if !strings.Contains(got.EffectiveSalesSpecJSON, want) {
			t.Fatalf("effective sales spec must freeze %s: %s", want, got.EffectiveSalesSpecJSON)
		}
	}
	missing, err := inspectPublishedProductSpecContent(content, 553)
	if err != nil {
		t.Fatalf("inspect missing SKU: %v", err)
	}
	if !missing.ConcretePublication || missing.ProductFound {
		t.Fatalf("missing concrete SKU = %+v, want concrete publication with product_found=false", missing)
	}
}

func TestPublishedPricingEnrichesTypedEffectiveSalesSpecWithInventoryConversion(t *testing.T) {
	content := []byte(`{
		"price_rows":[{
			"product_id":550,
			"sku_id":551,
			"parent_product_id":550,
			"quantity_basis":"sales_spec_count",
			"min_qty":1,
			"final_unit_price":68,
			"inventory_unit":"kg",
			"inventory_conversion_json":{"袋":{"kg":0.227}},
			"effective_sales_spec":{"sku_id":551,"spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(content, 551, ListTypeCommercial, 227, 2, "袋", 0)
	if !ok {
		t.Fatal("published concrete SKU pricing not found")
	}
	for _, want := range []string{`"parent_product_id":550`, `"inventory_unit":"kg"`, `"inventory_conversion_json":{"袋":{"kg":0.227}}`} {
		if !strings.Contains(got.EffectiveSalesSpecJSON, want) {
			t.Fatalf("published pricing effective sales spec must freeze %s: %s", want, got.EffectiveSalesSpecJSON)
		}
	}
}

func TestInspectPublishedProductSpecAllowsPriceRowConversionSubsetOfEffectiveSpecAuthority(t *testing.T) {
	content := []byte(`{
		"price_rows":[{
			"product_id":644,
			"sku_id":789,
			"parent_product_id":644,
			"quantity_basis":"sales_spec_count",
			"inventory_unit":"kg",
			"inventory_conversion_json":{"454g":{"kg":0.454}},
			"effective_sales_spec":{
				"sku_id":789,
				"spec_name":"454g",
				"spec_label":"454g",
				"sales_unit":"454g",
				"net_content_qty":454,
				"net_content_unit":"g",
				"inventory_unit":"kg",
				"inventory_conversion_json":{
					"g":{"kg":0.001},
					"kg":{"kg":1},
					"lb":{"kg":0.45359237},
					"磅":{"kg":0.45359237},
					"454g":{"kg":0.454}
				}
			}
		}]
	}`)

	got, err := inspectPublishedProductSpecContent(content, 789)
	if err != nil {
		t.Fatalf("inspect compatible conversion subset: %v", err)
	}
	if !got.ConcretePublication || !got.ProductFound || got.SKUID != 789 {
		t.Fatalf("concrete product spec = %+v", got)
	}
	if !strings.Contains(got.InventoryConversionJSON, `"454g":{"kg":0.454}`) ||
		!strings.Contains(got.InventoryConversionJSON, `"磅":{"kg":0.45359237}`) {
		t.Fatalf("effective inventory conversion authority = %s", got.InventoryConversionJSON)
	}
	snapshot, err := BuildProductionQuantitySnapshot(got)
	if err != nil {
		t.Fatalf("build production quantity snapshot: %v", err)
	}
	if snapshot.SalesUnit != "454g" || snapshot.InventoryUnit != "kg" || snapshot.InventoryQtyPerSalesUnit != 0.454 {
		t.Fatalf("production quantity snapshot = %+v", snapshot)
	}
}

func TestInspectPublishedProductSpecAllowsEquivalentFlatAndNestedConversionShapes(t *testing.T) {
	content := []byte(`{
		"price_rows":[{
			"product_id":791,
			"sku_id":791,
			"parent_product_id":644,
			"quantity_basis":"sales_spec_count",
			"inventory_unit":"kg",
			"inventory_conversion_json":{"kg":1},
			"effective_sales_spec":{
				"sku_id":791,
				"spec_name":"1Kg",
				"spec_label":"1Kg",
				"sales_unit":"kg",
				"net_content_qty":1,
				"net_content_unit":"kg",
				"inventory_unit":"kg",
				"inventory_conversion_json":{"kg":{"kg":1}}
			}
		}]
	}`)

	got, err := inspectPublishedProductSpecContent(content, 791)
	if err != nil {
		t.Fatalf("inspect equivalent flat conversion: %v", err)
	}
	if !got.ConcretePublication || !got.ProductFound || got.SKUID != 791 {
		t.Fatalf("concrete product spec = %+v", got)
	}
}

func TestValidatePublishedEffectiveSalesSpecAllowsEquivalentPoundAliases(t *testing.T) {
	frozen := publishedEffectiveSalesSpec{
		SalesUnit:     "磅",
		InventoryUnit: "kg",
		InventoryConversionJSON: json.RawMessage(`{
			"lb":{"kg":0.45359237},
			"lbs":{"kg":0.45359237},
			"磅":{"kg":0.45359237}
		}`),
	}
	if err := validatePublishedEffectiveSalesSpecInventoryAuthority(
		frozen,
		"公斤",
		json.RawMessage(`{"lb":{"公斤":0.45359237}}`),
	); err != nil {
		t.Fatalf("validate equivalent pound aliases: %v", err)
	}
}

func TestInspectPublishedProductSpecRejectsConflictingInventoryConversionSnapshots(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "inventory unit conflicts",
			content: `{
				"price_rows":[{
					"product_id":550,
					"sku_id":551,
					"parent_product_id":550,
					"quantity_basis":"sales_spec_count",
					"inventory_unit":"kg",
					"inventory_conversion_json":{"袋":{"kg":0.454}},
					"effective_sales_spec":{
						"sku_id":551,
						"spec_label":"454g",
						"sales_unit":"袋",
						"net_content_qty":454,
						"net_content_unit":"g",
						"inventory_unit":"g",
						"inventory_conversion_json":{"袋":{"g":454}}
					}
				}]
			}`,
			wantErr: "库存单位",
		},
		{
			name: "conversion graph conflicts",
			content: `{
				"price_rows":[{
					"product_id":550,
					"sku_id":551,
					"parent_product_id":550,
					"quantity_basis":"sales_spec_count",
					"inventory_unit":"kg",
					"inventory_conversion_json":{"袋":{"kg":0.454}},
					"effective_sales_spec":{
						"sku_id":551,
						"spec_label":"454g",
						"sales_unit":"袋",
						"net_content_qty":454,
						"net_content_unit":"g",
						"inventory_unit":"kg",
						"inventory_conversion_json":{"袋":{"kg":0.227}}
					}
				}]
			}`,
			wantErr: "库存换算",
		},
		{
			name: "equivalent pound aliases disagree",
			content: `{
				"price_rows":[{
					"product_id":550,
					"sku_id":551,
					"parent_product_id":550,
					"quantity_basis":"sales_spec_count",
					"inventory_unit":"kg",
					"inventory_conversion_json":{"lb":{"kg":0.454}},
					"effective_sales_spec":{
						"sku_id":551,
						"spec_label":"磅",
						"sales_unit":"磅",
						"inventory_unit":"kg",
						"inventory_conversion_json":{
							"lb":{"kg":0.454},
							"磅":{"kg":0.45359237}
						}
					}
				}]
			}`,
			wantErr: "库存换算",
		},
		{
			name: "empty conversion edge",
			content: `{
				"price_rows":[{
					"product_id":550,
					"sku_id":551,
					"parent_product_id":550,
					"quantity_basis":"sales_spec_count",
					"inventory_unit":"kg",
					"inventory_conversion_json":{"磅":{}},
					"effective_sales_spec":{
						"sku_id":551,
						"spec_label":"磅",
						"sales_unit":"磅",
						"inventory_unit":"kg",
						"inventory_conversion_json":{"磅":{"kg":0.45359237}}
					}
				}]
			}`,
			wantErr: "库存换算",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := inspectPublishedProductSpecContent([]byte(tt.content), 551); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("inspect conflict err = %v, want %s conflict", err, tt.wantErr)
			}
		})
	}
}

func TestInspectPublishedProductSpecIgnoresUnrelatedSKUConversionConflict(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{
				"product_id":700,
				"sku_id":700,
				"parent_product_id":699,
				"quantity_basis":"sales_spec_count",
				"inventory_unit":"kg",
				"inventory_conversion_json":{"袋":{"kg":0.5}},
				"effective_sales_spec":{
					"sku_id":700,
					"sales_unit":"袋",
					"inventory_unit":"kg",
					"inventory_conversion_json":{"袋":{"kg":0.25}}
				}
			},
			{
				"product_id":789,
				"sku_id":789,
				"parent_product_id":644,
				"quantity_basis":"sales_spec_count",
				"inventory_unit":"kg",
				"inventory_conversion_json":{"454g":{"kg":0.454}},
				"effective_sales_spec":{
					"sku_id":789,
					"sales_unit":"454g",
					"inventory_unit":"kg",
					"inventory_conversion_json":{
						"454g":{"kg":0.454},
						"磅":{"kg":0.45359237}
					}
				}
			}
		]
	}`)

	got, err := inspectPublishedProductSpecContent(content, 789)
	if err != nil {
		t.Fatalf("inspect target SKU with unrelated conflict: %v", err)
	}
	if !got.ProductFound || got.SKUID != 789 {
		t.Fatalf("target product spec = %+v", got)
	}
	if _, err := inspectPublishedProductSpecContent(content, 700); err == nil || !strings.Contains(err.Error(), "库存换算") {
		t.Fatalf("inspect conflicting SKU error = %v, want inventory conversion conflict", err)
	}
}

func TestAttachProductionQuantitySnapshotUsesPublishedConversionAndBlocksMissingConversion(t *testing.T) {
	source, err := AttachProductionQuantitySnapshot(`{"source":"published_price_snapshot"}`, PublishedProductSpec{
		ConcretePublication:     true,
		ProductFound:            true,
		SKUID:                   551,
		ParentProductID:         550,
		SpecLabel:               "454g",
		SalesUnit:               "袋",
		InventoryUnit:           "kg",
		InventoryConversionJSON: `{"袋":{"kg":0.454}}`,
	})
	if err != nil {
		t.Fatalf("attach production quantity snapshot: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(source), &decoded); err != nil {
		t.Fatalf("decode attached source: %v", err)
	}
	snapshot, ok := decoded["production_quantity_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("production quantity snapshot missing: %s", source)
	}
	if snapshot["sku_id"] != float64(551) || snapshot["parent_product_id"] != float64(550) ||
		snapshot["inventory_qty_per_sales_unit"] != 0.454 ||
		snapshot["conversion_source"] != "published_inventory_conversion" {
		t.Fatalf("production quantity snapshot = %#v", snapshot)
	}

	if _, err := AttachProductionQuantitySnapshot(`{}`, PublishedProductSpec{
		ConcretePublication: true,
		ProductFound:        true,
		SKUID:               552,
		ParentProductID:     550,
		SpecLabel:           "454g",
		SalesUnit:           "袋",
		InventoryUnit:       "kg",
	}); err == nil || !strings.Contains(err.Error(), "缺少") {
		t.Fatalf("missing concrete conversion err = %v, want blocking error", err)
	}

	legacy, err := AttachProductionQuantitySnapshot(`{"source":"legacy"}`, PublishedProductSpec{ProductFound: true})
	if err != nil || !strings.Contains(legacy, `"source":"legacy"`) {
		t.Fatalf("legacy source = %s / %v, want compatibility", legacy, err)
	}
}

func TestAttachProductionQuantitySnapshotFreezesResolvedCurrentCatalogSpecForLegacyPublication(t *testing.T) {
	source, err := AttachProductionQuantitySnapshot(`{"source":"legacy"}`, PublishedProductSpec{
		CurrentCatalogAuthority: true,
		ProductFound:            true,
		SKUID:                   789,
		ParentProductID:         644,
		SpecLabel:               "454g",
		SalesUnit:               "454g",
		NetContentQty:           454,
		NetContentUnit:          "g",
		InventoryUnit:           "kg",
		InventoryConversionJSON: `{"454g":{"kg":0.454}}`,
	})
	if err != nil {
		t.Fatalf("attach current catalog production quantity snapshot: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(source), &decoded); err != nil {
		t.Fatalf("decode current catalog snapshot: %v", err)
	}
	snapshot, ok := decoded["production_quantity_snapshot"].(map[string]any)
	if !ok {
		t.Fatalf("current catalog production quantity snapshot missing: %s", source)
	}
	if snapshot["sku_id"] != float64(789) ||
		snapshot["parent_product_id"] != float64(644) ||
		snapshot["inventory_qty_per_sales_unit"] != 0.454 ||
		snapshot["conversion_source"] != "current_catalog_inventory_conversion" {
		t.Fatalf("current catalog production quantity snapshot = %#v", snapshot)
	}

	if _, err := AttachProductionQuantitySnapshot(`{"source":"legacy"}`, PublishedProductSpec{
		CurrentCatalogAuthority: true,
		ProductFound:            true,
		SKUID:                   790,
		ParentProductID:         644,
		SpecLabel:               "自定义规格",
		SalesUnit:               "盒",
		InventoryUnit:           "袋",
	}); err == nil || !strings.Contains(err.Error(), "缺少") {
		t.Fatalf("current catalog missing conversion err = %v, want blocking error", err)
	}
}

type currentProductionSpecTestQuerier struct {
	row     currentProductionSpecTestRow
	queries []string
}

func (q *currentProductionSpecTestQuerier) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	q.queries = append(q.queries, sql)
	return q.row
}

type currentProductionSpecTestRow struct {
	values []any
	err    error
}

func (r currentProductionSpecTestRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return errors.New("unexpected production spec scan width")
	}
	for idx, value := range r.values {
		switch target := dest[idx].(type) {
		case *int64:
			*target = value.(int64)
		case *float64:
			*target = value.(float64)
		case *string:
			*target = value.(string)
		case *bool:
			*target = value.(bool)
		default:
			return errors.New("unexpected production spec scan destination")
		}
	}
	return nil
}

func TestResolveOrderProductionProductSpecUsesCurrentActiveSKUForLegacyPublication(t *testing.T) {
	q := &currentProductionSpecTestQuerier{row: currentProductionSpecTestRow{values: []any{
		int64(789), int64(644), "454g", "454g", "454g", "454g",
		float64(454), "g", "roasted_bean", int64(0), float64(0),
		"kg", `{}`, true,
	}}}
	spec, err := ResolveOrderProductionProductSpec(
		context.Background(),
		q,
		"test_schema",
		789,
		PublishedProductSpec{ProductFound: true},
	)
	if err != nil {
		t.Fatalf("resolve legacy order production spec: %v", err)
	}
	if !spec.CurrentCatalogAuthority || spec.SKUID != 789 || spec.ParentProductID != 644 ||
		spec.InventoryConversionJSON != `{"454g":{"kg":0.454}}` {
		t.Fatalf("resolved current catalog spec = %+v", spec)
	}

	if _, err := ResolveOrderProductionProductSpec(
		context.Background(),
		q,
		"test_schema",
		789,
		PublishedProductSpec{
			ConcretePublication: true,
			ProductFound:        true,
			SKUID:               789,
			ParentProductID:     999,
			SpecLabel:           "454g",
			SalesUnit:           "454g",
			InventoryUnit:       "kg",
			NetContentQty:       454,
			NetContentUnit:      "g",
		},
	); err == nil || !strings.Contains(err.Error(), "不属于") {
		t.Fatalf("concrete publication parent mismatch err = %v", err)
	}
}

func TestResolveCurrentOrderProductionProductSpecNeverSearchesPublishedPriceLists(t *testing.T) {
	q := &currentProductionSpecTestQuerier{row: currentProductionSpecTestRow{values: []any{
		int64(789), int64(644), "454g", "454g", "454g", "454g",
		float64(454), "g", "roasted_bean", int64(0), float64(0),
		"kg", `{}`, true,
	}}}

	spec, err := ResolveCurrentOrderProductionProductSpec(
		context.Background(),
		q,
		"test_schema",
		789,
	)
	if err != nil {
		t.Fatalf("resolve current order production spec: %v", err)
	}
	if !spec.CurrentCatalogAuthority || spec.SKUID != 789 || spec.ParentProductID != 644 {
		t.Fatalf("current catalog spec = %+v", spec)
	}
	if len(q.queries) != 1 {
		t.Fatalf("current catalog resolver queries = %d, want exactly one product master query", len(q.queries))
	}
	if strings.Contains(q.queries[0], "bean_list_publications") {
		t.Fatalf("current catalog resolver must not search published price lists: %s", q.queries[0])
	}
}

func TestInspectPublishedProductSpecRejectsFrozenSKUIdentityMismatch(t *testing.T) {
	content := []byte(`{
		"price_rows":[{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":68,
		 "effective_sales_spec":{"sku_id":552,"spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}}]
	}`)

	if _, err := inspectPublishedProductSpecContent(content, 551); err == nil || !strings.Contains(err.Error(), "SKU") {
		t.Fatalf("frozen SKU mismatch err = %v, want SKU identity error", err)
	}
}

func TestInspectPublishedProductSpecKeepsLegacyPublicationCompatible(t *testing.T) {
	content := []byte(`{"groups":[{"items":[{"productId":7,"commercial_wholesale_tiers":[{"spec_g":454,"min_qty":2,"price_per_unit":68}]}]}]}`)

	got, err := inspectPublishedProductSpecContent(content, 7)
	if err != nil {
		t.Fatalf("inspect legacy publication: %v", err)
	}
	if got.ConcretePublication || !got.ProductFound {
		t.Fatalf("legacy publication identity = %+v", got)
	}
}

func TestInspectPublishedProductSpecMixedRowsRequireTargetConcreteSnapshot(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{"product_id":550,"sku_id":551,"quantity_basis":"sales_spec_count","final_unit_price":68},
			{"product_id":550,"sku_id":552,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":118,
			 "effective_sales_spec":{"sku_id":552,"spec_name":"454g袋装","spec_label":"454g","sales_unit":"袋","net_content_qty":454,"net_content_unit":"g"}}
		]
	}`)

	got, err := inspectPublishedProductSpecContent(content, 551)
	if err != nil {
		t.Fatalf("inspect mixed publication: %v", err)
	}
	if !got.ConcretePublication || got.ProductFound {
		t.Fatalf("mixed publication target = %+v, want concrete publication with incomplete target rejected", got)
	}
}

func TestInspectPublishedProductSpecMixedPublicationKeepsLegacyProductCompatible(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{"productId":700,"name":"历史豆"}]}],
		"price_rows":[
			{"product_id":550,"sku_id":551,"parent_product_id":550,"quantity_basis":"sales_spec_count","final_unit_price":68,
			 "effective_sales_spec":{"sku_id":551,"spec_name":"227g袋装","spec_label":"227g","sales_unit":"袋","net_content_qty":227,"net_content_unit":"g"}},
			{"product_id":700,"spec_g":454,"min_qty":1,"final_unit_price":66}
		]
	}`)

	legacy, err := inspectPublishedProductSpecContent(content, 700)
	if err != nil {
		t.Fatalf("inspect mixed publication legacy product: %v", err)
	}
	if legacy.ConcretePublication || !legacy.ProductFound {
		t.Fatalf("mixed publication legacy product = %+v, want legacy-compatible product", legacy)
	}

	concrete, err := inspectPublishedProductSpecContent(content, 551)
	if err != nil {
		t.Fatalf("inspect mixed publication concrete product: %v", err)
	}
	if !concrete.ConcretePublication || !concrete.ProductFound || concrete.SKUID != 551 {
		t.Fatalf("mixed publication concrete product = %+v", concrete)
	}

	missing, err := inspectPublishedProductSpecContent(content, 999)
	if err != nil {
		t.Fatalf("inspect mixed publication missing product: %v", err)
	}
	if !missing.ConcretePublication || missing.ProductFound {
		t.Fatalf("mixed publication missing product = %+v, want strict not-found", missing)
	}
}

func TestPublishedUnitPriceFromContentMatchesCommercialAndDripTiers(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"commercial_wholesale_tiers":[
					{"label":"1-9kg","spec_g":1000,"min_qty":1,"max_qty":9,"price_per_unit":80},
					{"label":"10kg+","spec_g":1000,"min_qty":10,"price_per_unit":75}
				]
			},{
				"productId":12,
				"drip_wholesale_tiers":[
					{"label":"100袋+","sales_unit":"bag","min_qty":100,"price_per_unit":3},
					{"label":"20盒+","sales_unit":"box","unit_bag_count":10,"min_qty":20,"price_per_unit":28}
				]
			}]
		}]
	}`)

	got, ok := publishedUnitPriceFromContentForListType(content, 11, ListTypeCommercial, 1000, 12, "", 0)
	if !ok || got != 75 {
		t.Fatalf("commercial published price = %.2f/%v, want 75/true", got, ok)
	}
	got, ok = publishedUnitPriceFromContentForListType(content, 12, ListTypeDrip, 10, 120, "bag", 1)
	if !ok || got != 3 {
		t.Fatalf("drip bag published price = %.2f/%v, want 3/true", got, ok)
	}
	got, ok = publishedUnitPriceFromContentForListType(content, 12, ListTypeDrip, 100, 25, "box", 10)
	if !ok || got != 28 {
		t.Fatalf("drip box published price = %.2f/%v, want 28/true", got, ok)
	}
}

func TestCommercialFlatRowsPriceDerivedDripSKUsAndKeepSalesUnits(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[
			{"productId":700,"name":"金色山脉 挂耳 袋（10g）"},
			{"productId":701,"name":"金色山脉 挂耳 盒（10袋）"}
		]}],
		"price_rows":[
			{"product_id":700,"tier_label":"100袋+","spec_g":10,"min_qty":100,"max_qty":999,"final_unit_price":3.08,"price_unit":"袋（10g）","inventory_unit":"袋","inventory_conversion_json":{"袋（10g）":{"袋":1}}},
			{"product_id":701,"tier_label":"10盒+","spec_g":100,"min_qty":10,"max_qty":99,"final_unit_price":32.8,"price_unit":"盒（10袋）","inventory_unit":"袋","inventory_conversion_json":{"盒（10袋）":{"袋":10}}}
		]
	}`)

	bag, ok := publishedPricingFromContentForListType(content, 700, ListTypeCommercial, 10, 120, "bag", 1)
	if !ok || bag.UnitPrice != 3.08 || bag.PriceUnit != "袋（10g）" || bag.InventoryUnit != "袋" {
		t.Fatalf("commercial derived drip bag pricing = %+v/%v", bag, ok)
	}
	box, ok := publishedPricingFromContentForListType(content, 701, ListTypeCommercial, 100, 20, "box", 10)
	if !ok || box.UnitPrice != 32.8 || box.PriceUnit != "盒（10袋）" || box.InventoryUnit != "袋" {
		t.Fatalf("commercial derived drip box pricing = %+v/%v", box, ok)
	}
}

func TestPublishedPricingKeepsKgDisplayUnitForSmallCommercialPackInsideTier(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"commercial_wholesale_tiers":[
					{"label":"25-49kg","spec_g":1000,"min_qty":25,"max_qty":49,"price_per_unit":82,"price_per_lb":37.23,"display_unit":"kg","price_unit":"kg"}
				]
			}]
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 80, 313, "", 0)
	if !ok {
		t.Fatalf("published pricing missing")
	}
	if got.UnitPrice != 82 || got.PriceUnit != "kg" || got.UnitG != 1000 {
		t.Fatalf("published pricing = %+v, want 82 kg/1000g", got)
	}
}

func TestPublishedCommercialPricingRejectsQuantityOutsideEveryTier(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"commercial_wholesale_tiers":[
					{"label":"25-49kg","spec_g":1000,"min_qty":25,"max_qty":49,"price_per_unit":82,"price_unit":"kg"},
					{"label":"51-60kg","spec_g":1000,"min_qty":51,"max_qty":60,"price_per_unit":78,"price_unit":"kg"}
				]
			}]
		}]
	}`)

	for _, qty := range []int64{1, 50, 61} {
		if got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 1000, qty, "", 0); ok {
			t.Fatalf("commercial qty %d pricing = %+v/true, want explicit missing price", qty, got)
		}
	}
}

func TestPublishedCommercialFlatRowsRejectDerivedDripQuantityBelowTier(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{"product_id":700,"tier_label":"100袋+","spec_g":10,"min_qty":100,"max_qty":999,"final_unit_price":3.08,"price_unit":"袋（10g）"},
			{"product_id":701,"tier_label":"10盒+","spec_g":100,"min_qty":10,"max_qty":99,"final_unit_price":32.8,"price_unit":"盒（10袋）"}
		]
	}`)

	if got, ok := publishedPricingFromContentForListType(content, 700, ListTypeCommercial, 10, 99, "bag", 1); ok {
		t.Fatalf("commercial drip bag below minimum = %+v/true, want missing price", got)
	}
	if got, ok := publishedPricingFromContentForListType(content, 701, ListTypeCommercial, 100, 9, "box", 10); ok {
		t.Fatalf("commercial drip box below minimum = %+v/true, want missing price", got)
	}
}

func TestPublishedCommercialFlatRowsDoNotBypassExplicitWeightBoundsWithUnitQuantity(t *testing.T) {
	content := []byte(`{
		"price_rows":[
			{"product_id":710,"tier_label":"1-6kg","spec_g":227,"min_qty":1,"max_qty":6,"min_weight_g":1000,"max_weight_g":6999.999,"final_unit_price":20,"price_unit":"g227"}
		]
	}`)

	if got, ok := publishedPricingFromContentForListType(content, 710, ListTypeCommercial, 227, 1, "", 0); ok {
		t.Fatalf("227g one-pack price = %+v/true, want missing price below explicit 1kg bound", got)
	}
	if got, ok := publishedPricingFromContentForListType(content, 710, ListTypeCommercial, 227, 5, "", 0); !ok || got.UnitPrice != 20 {
		t.Fatalf("227g five-pack price = %+v/%v, want legal explicit-weight tier", got, ok)
	}
}

func TestPublishedCommercialPricingDoesNotBypassAnOutOfRangeExactSpecTier(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{
			"productId":711,
			"commercial_wholesale_tiers":[
				{"spec_g":227,"min_qty":1,"max_qty":10,"price_per_unit":20},
				{"spec_g":454,"min_qty":5,"max_qty":10,"price_per_unit":30}
			]
		}]}]
	}`)

	if got, ok := publishedPricingFromContentForListType(content, 711, ListTypeCommercial, 454, 1, "", 0); ok {
		t.Fatalf("454g one-pack price = %+v/true, want missing price instead of falling through to the 227g tier", got)
	}
	if got, ok := publishedPricingFromContentForListType(content, 711, ListTypeCommercial, 454, 5, "", 0); !ok || got.UnitPrice != 30 {
		t.Fatalf("454g five-pack price = %+v/%v, want legal exact-spec tier 30", got, ok)
	}
}

func TestPublishedLegacyDripPricingRequiresLegalBagAndBoxTier(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{
			"productId":12,
			"drip_wholesale_tiers":[
				{"label":"1-99袋","sales_unit":"bag","min_qty":1,"max_qty":99,"price_per_unit":4},
				{"label":"100-199袋","sales_unit":"bag","min_qty":100,"max_qty":199,"price_per_unit":3},
				{"label":"300袋+","sales_unit":"bag","min_qty":300,"price_per_unit":2.8},
				{"label":"20-29盒","sales_unit":"box","unit_bag_count":10,"min_qty":20,"max_qty":29,"price_per_unit":28}
			]
		}]}]
	}`)

	if got, ok := publishedUnitPriceFromContentForListType(content, 12, ListTypeDrip, 10, 150, "bag", 1); !ok || got != 3 {
		t.Fatalf("legacy drip valid bag tier = %.2f/%v, want 3/true", got, ok)
	}
	if got, ok := publishedUnitPriceFromContentForListType(content, 12, ListTypeDrip, 100, 25, "box", 10); !ok || got != 28 {
		t.Fatalf("legacy drip valid box tier = %.2f/%v, want 28/true", got, ok)
	}
	if got, ok := publishedPricingFromContentForListType(content, 12, ListTypeDrip, 10, 250, "bag", 1); ok {
		t.Fatalf("legacy drip bag gap = %+v/true, want missing price", got)
	}
	if got, ok := publishedPricingFromContentForListType(content, 12, ListTypeDrip, 100, 10, "box", 10); ok {
		t.Fatalf("legacy drip box below explicit box tier = %+v/true, want missing price", got)
	}
}

func TestPublishedPricingCarriesFinalPriceSnapshotMetadata(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"commercial_wholesale_tiers":[
					{"label":"25kg+","source_price_record_id":701,"spec_g":1000,"min_qty":25,"final_unit_price":82,"price_per_unit":82,"display_unit":"kg","price_unit":"kg","inventory_unit":"kg","inventory_conversion_json":{"kg":{"kg":1}}}
				]
			}]
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 1000, 25, "", 0)
	if !ok {
		t.Fatalf("published pricing missing")
	}
	if got.SourcePriceRecordID != 701 || got.InventoryUnit != "kg" || !strings.Contains(got.InventoryConversionJSON, `"kg"`) {
		t.Fatalf("published pricing snapshot metadata = %+v", got)
	}
}

func TestPublishedPricingMatchesPR440FlatPriceRows(t *testing.T) {
	content := []byte(`{
		"groups":[{
			"items":[{
				"productId":11,
				"name":"PR440 平铺价格商品"
			}]
		}],
		"price_rows":[{
			"product_id":11,
			"tier_label":"1kg+",
			"min_qty":1,
			"final_unit_price":88,
			"price_unit":"kg",
			"inventory_unit":"kg",
			"inventory_conversion_json":{"kg":1},
			"pricing_rule_version":"PR440/v1",
			"customer_reference_snapshot":{"customer_id":3,"customer_display_name":"客户显示名"}
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(content, 11, ListTypeCommercial, 1000, 1, "", 0)
	if !ok {
		t.Fatalf("PR-440 flat price row should resolve published pricing")
	}
	if got.UnitPrice != 88 || got.PriceUnit != "kg" || got.UnitG != 1000 || got.InventoryUnit != "kg" || !strings.Contains(got.InventoryConversionJSON, `"kg"`) {
		t.Fatalf("published PR-440 flat pricing = %+v, want 88 kg with inventory snapshot", got)
	}
}

func TestPublishedCommercialFlatPriceRowsPreferDerivedSKUIDentity(t *testing.T) {
	content := []byte(`{
		"groups":[{"items":[{
			"productId":500,
			"sku_id":711,
			"commercial_wholesale_tiers":[{"spec_g":100,"min_qty":10,"max_qty":99,"price_per_unit":31}]
		}]}],
		"price_rows":[{
			"product_id":500,
			"sku_id":711,
			"parent_product_id":500,
			"tier_label":"10盒+",
			"spec_g":100,
			"min_qty":10,
			"max_qty":99,
			"final_unit_price":32.8,
			"price_unit":"box",
			"sales_unit":"box",
			"unit_bag_count":10
		}]
	}`)

	if got, ok := publishedPricingFromContentForListType(content, 711, ListTypeCommercial, 100, 20, "box", 10); !ok || got.UnitPrice != 32.8 {
		t.Fatalf("derived SKU flat price = %+v/%v, want sku_id 711 price 32.8", got, ok)
	}
	if got, ok := publishedPricingFromContentForListType(content, 500, ListTypeCommercial, 100, 20, "box", 10); ok {
		t.Fatalf("parent product must not consume derived SKU flat price: %+v", got)
	}
}

func TestPublishedCommercialFlatRowsUseConcreteSalesSpecCountAndKeepLegacyWeightFallback(t *testing.T) {
	newContent := []byte(`{
		"price_rows":[{
			"product_id":550,
			"sku_id":551,
			"parent_product_id":550,
			"quantity_basis":"sales_spec_count",
			"effective_sales_spec":{"sku_id":551,"spec_name":"磅","sales_unit":"磅","net_content_qty":1,"net_content_unit":"lb"},
			"tier_label":"2-4磅",
			"min_qty":2,
			"max_qty":4,
			"final_unit_price":68,
			"price_unit":"kg"
		}]
	}`)

	got, ok := publishedPricingFromContentForListType(newContent, 551, ListTypeCommercial, 454, 2, "磅", 0)
	if !ok || got.UnitPrice != 68 {
		t.Fatalf("sales-spec-count pricing = %+v/%v, want two concrete pound SKUs to match", got, ok)
	}
	if got.QuantityBasis != "sales_spec_count" || got.TierQuantityUnit != "" || !strings.Contains(got.EffectiveSalesSpecJSON, `"net_content_unit":"lb"`) {
		t.Fatalf("sales-spec-count pricing must preserve frozen order semantics: %+v", got)
	}
	if _, ok := publishedPricingFromContentForListType(newContent, 550, ListTypeCommercial, 454, 2, "磅", 0); ok {
		t.Fatal("parent product must not consume its child SKU count tier")
	}

	legacyContent := []byte(`{
		"price_rows":[{
			"product_id":552,
			"tier_label":"1kg+",
			"spec_g":1000,
			"min_qty":1,
			"final_unit_price":82,
			"price_unit":"kg"
		}]
	}`)
	legacy, ok := publishedPricingFromContentForListType(legacyContent, 552, ListTypeCommercial, 454, 3, "磅", 0)
	if !ok || legacy.UnitPrice != 82 {
		t.Fatalf("legacy weight fallback = %+v/%v, want 3lb to keep matching the old 1kg tier", legacy, ok)
	}
}

func TestExplicitPublicationSelectionRequiresPublishedSnapshots(t *testing.T) {
	source, err := os.ReadFile("usage.go")
	if err != nil {
		t.Fatalf("read usage.go: %v", err)
	}
	text := string(source)
	if strings.Contains(text, "blp.status IN ('published','withdrawn')") {
		t.Fatalf("explicit bean-list publication selection must reject withdrawn snapshots")
	}
	if !strings.Contains(text, "WHERE blp.status='published'") {
		t.Fatalf("bean-list publication selection must use published snapshots only")
	}
}

func TestPublishedPricingUsageOnlySelectsFactorySupplyPublications(t *testing.T) {
	source, err := os.ReadFile("usage.go")
	if err != nil {
		t.Fatalf("read usage.go: %v", err)
	}
	text := string(source)
	start := strings.Index(text, "func ResolveUsageForPublication")
	end := strings.Index(text, "type publishedBeanListContent")
	if start < 0 || end <= start {
		t.Fatal("published pricing usage resolver not found")
	}
	if count := strings.Count(text[start:end], "blp.publication_purpose='factory_supply'"); count != 2 {
		t.Fatalf("explicit and automatic order pricing usage must reject customer_resale; factory_supply filter count=%d", count)
	}
	for _, want := range []string{"content_json->'price_rows'", "row_json->>'sku_id'"} {
		if !strings.Contains(text[start:end], want) {
			t.Fatalf("automatic order pricing usage must recognize derived SKU flat rows; missing %q", want)
		}
	}
}

func TestListTypeForProductKindUsesGreenBeanList(t *testing.T) {
	if got := ListTypeForProductKind("green_bean", false); got != ListTypeGreen {
		t.Fatalf("green bean list type = %q, want %q", got, ListTypeGreen)
	}
	if got := ListTypeForProductKind("drip_bag", false); got != ListTypeDrip {
		t.Fatalf("shared portal-compatible drip list type = %q, want %q", got, ListTypeDrip)
	}
	if got := ListTypeForProductKind("roasted", true); got != ListTypeRetail {
		t.Fatalf("retail roasted list type = %q, want %q", got, ListTypeRetail)
	}
}

func TestPublishedPricingMatchesStableBOMSpecAcrossVersionsWithoutWeightConversion(t *testing.T) {
	raw := []byte(`{
		"price_rows":[
			{"product_id":10,"bom_spec_id":101,"bom_variant_id":1001,"min_qty":1,"final_unit_price":68,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"},
			{"product_id":10,"bom_spec_id":102,"bom_variant_id":1002,"min_qty":1,"final_unit_price":88,"price_unit":"盒","inventory_unit":"盒","quantity_basis":"sales_spec_count"}
		]
	}`)
	pricing, ok := publishedPricingFromContentForBOMSpec(raw, 10, 101, 2001, 2, ListTypeCommercial)
	if !ok || pricing.UnitPrice != 68 || pricing.PriceUnit != "袋" || pricing.InventoryUnit != "袋" || pricing.QuantityBasis != "sales_spec_count" {
		t.Fatalf("BOM spec pricing=%+v/%v", pricing, ok)
	}
	if _, ok := publishedPricingFromContentForBOMSpec(raw, 10, 103, 1001, 2, ListTypeCommercial); ok {
		t.Fatal("a different stable BOM spec must not match")
	}
	grouped := []byte(`{
		"groups":[{"items":[{
			"product_id":10,"bom_spec_id":101,"bom_variant_id":1001,
			"commercial_wholesale_tiers":[{"min_qty":1,"final_unit_price":72,"price_unit":"袋","inventory_unit":"袋","quantity_basis":"sales_spec_count"}]
		}]}]
	}`)
	pricing, ok = publishedPricingFromContentForBOMSpec(grouped, 10, 101, 2001, 2, ListTypeCommercial)
	if !ok || pricing.UnitPrice != 72 {
		t.Fatalf("grouped stable BOM spec pricing=%+v/%v", pricing, ok)
	}
}
