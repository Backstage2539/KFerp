package customerportal

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	customerportalapp "orderapp/internal/application/customerportal"
)

func TestCustomerPortalBusinessRepositoryLoadsOrderItemsForCustomerOrders(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"FROM %s.order_items",
		"listCustomerOrderItems",
		"CustomerOrderItemSummary",
		"ORDER BY oi.order_id, oi.line_no, oi.id",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer order item loading missing %q", want)
		}
	}
}

func TestCustomerPortalBeanListFallsBackToLatestOfficialPublications(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"listLatestOfficialBeanLists",
		"owner_type='official'",
		"status='published'",
		"DISTINCT ON (list_type)",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("latest official bean list fallback missing %q", want)
		}
	}
}

func TestCustomerPortalBeanListLoadsDisplayItemsFromPublishedContent(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"config_json",
		"content_json",
		"parseBeanListDisplaySummary",
		"BeanListGroupSummary",
		"BeanListProductSummary",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal bean list content display missing %q", want)
		}
	}
}

func TestCustomerPortalBeanListSummariesExposeNativeCacheMetadata(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"CacheKey",
		"beanListCacheKey",
		"LayoutStyle",
		"CardsPerRow",
		"ShowCategoryNumbers",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("bean list native display cache metadata missing %q", want)
		}
	}
}

func TestCustomerPortalOrderQuerySupportsKeywordAndDateFilters(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"query.Query",
		"query.DateFrom",
		"query.DateTo",
		"LOWER(COALESCE(c.contact,''))",
		"LOWER(COALESCE(c.address,''))",
		"LOWER(COALESCE(o.receiver_name,''))",
		"LOWER(COALESCE(o.receiver_phone,''))",
		"LOWER(COALESCE(o.receiver_address,''))",
		"COALESCE(NULLIF(o.receiver_name,''), NULLIF(c.contact,''), c.name, '')",
		"COALESCE(NULLIF(o.receiver_phone,''), c.phone, '')",
		"COALESCE(NULLIF(o.receiver_address,''), NULLIF(c.address,''), c.company_address, '')",
		"EXISTS (SELECT 1 FROM %s.order_items",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal order filter query missing %q", want)
		}
	}
}

func TestCustomerPortalOrderQuerySupportsStatusFilters(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"query.ProcessStatus",
		"query.PayStatus",
		"query.ShipStatus",
		"LOWER(COALESCE(ops.name,'')) =",
		"LOWER(COALESCE(ps.name,'')) =",
		"LOWER(COALESCE(ss.name,'')) =",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal order status filter query missing %q", want)
		}
	}
}

func TestCustomerPortalOrderBackboneUsesSharedOrdersAndExcludesVoided(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"case customerportalapp.ServiceKeyOrders:",
		"case customerportalapp.ServiceKeySettlement:",
		"page.Orders, err = r.listCustomerOrders(ctx, query, limit, true)",
		"page.Orders, err = r.listCustomerOrders(ctx, query, limit, false)",
		`where := []string{"o.customer_id=$1", "o.is_void=false"}`,
		"FROM %s.orders o",
		"WHERE id=$1 AND customer_id=$2 AND is_void=false",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal order backbone missing %q", want)
		}
	}
}

func TestBusinessRepositoryCreatesMallOrdersFromPublishedMallProducts(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"LoadMallPage",
		"CreateMallOrder",
		"mall_products",
		"PortalServiceMall",
		"created_by_mini_user_id",
		"status='published'",
		"line_no,product_id,product_kind,bean_list_publication_id,bean_list_version_no,item_name,qty,unit,spec,unit_price,line_total",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("mall order repository missing %q", want)
		}
	}
}

func TestCustomerPortalFulfillmentPricingUsesPublishedSnapshotsOnly(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"ResolvePublishedPricingForPublicationWithUnit",
		"ResolveUsageForPublication",
		"published_price_snapshot",
		"source_price_record_id",
		"inventory_conversion_json",
		"bean_list_publication_id,bean_list_version_no",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal fulfillment pricing missing published snapshot marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"portalFulfillmentUnitPriceTx",
		"portalDripUnitPriceTiersTx",
		"FROM %s.product_price_tiers",
		"published_unit_price",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("customer portal fulfillment pricing should not keep legacy price fallback %q", forbidden)
		}
	}
}

func TestPortalMallLinePricingUsesBagQuoteForDripBoxOrders(t *testing.T) {
	got, err := portalMallLinePricingFor(mallOrderLine{
		ProductID:       18,
		Title:           "花魁挂耳",
		UnitPrice:       3.5,
		ProductKind:     "drip_bag",
		DripBagGrams:    10,
		DripBoxBagCount: 12,
	}, customerportalapp.MallOrderItemCommand{
		Qty:          3,
		SalesUnit:    "box",
		UnitBagCount: 1,
		UnitBeanG:    8,
	})
	if err != nil {
		t.Fatalf("portalMallLinePricingFor() err=%v", err)
	}
	if got.DisplayUnit != "盒" || got.SpecText != "10g*12袋/盒" || got.UnitPrice != 42 || got.LineTotal != 126 || got.UnitBagCount != 12 || got.UnitBeanG != 10 {
		t.Fatalf("pricing=%+v, want authoritative product box metadata and bag quote multiplied to box price", got)
	}
}

func TestPortalProductVisibleSQLExcludesBaseProductWithCustomerAlias(t *testing.T) {
	sql := portalProductVisibleToCustomerAliasSQL("products", "p", "$2")

	for _, want := range []string{
		"AND NOT",
		"EXISTS",
		"base_product_id",
		"alias_products",
		"customer_only",
	} {
		if !strings.Contains(sql, want) {
			t.Fatalf("portal product visibility SQL missing %q: %s", want, sql)
		}
	}
}

func TestParseBeanListDisplaySummaryExtractsPublishedStyleAndContent(t *testing.T) {
	configRaw, err := json.Marshal(map[string]any{
		"brandName":           "棵凡咖啡",
		"brandIntro":          "源头工厂现烘现发",
		"layoutStyle":         "table",
		"cardsPerRow":         3,
		"showVersion":         true,
		"showChangelog":       false,
		"showCategoryNumbers": false,
		"backgroundColor":     "#f8f1e5",
		"fontColor":           "#171717",
		"backgroundImage":     "/uploads/bean-bg.png",
		"logoImage":           "/uploads/logo.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	contentRaw, err := json.Marshal(map[string]any{
		"title":    "棵凡咖啡批发豆单",
		"subtitle": "报价不含税、不含运",
		"groups": []any{
			map[string]any{
				"category":     "原产地精选豆",
				"showCategory": false,
				"items": []any{
					map[string]any{
						"code":           "5.2",
						"name":           "乌拉嘎",
						"badge":          "new",
						"badgeLabel":     "NEW",
						"recommendedUse": "手冲/SOE/冷萃",
						"flavor":         "柑橘/莓果",
						"description":    "干净明亮",
						"beanListQuality": map[string]any{
							"factoryFlavorDescription": "茉莉花、柑橘",
							"moisture":                 "10.8%",
							"density":                  "780g/L",
							"inspectionCreatedAt":      "2026-05-18 09:30",
						},
						"highlightTerms": []any{"乌拉嘎", "柑橘"},
						"prices": []any{
							map[string]any{"label": "454g", "price": 118, "unit": "包", "red": true},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	row := customerportalapp.BeanListSummary{
		ID:        7,
		ListType:  "commercial",
		VersionNo: "V3.0.6",
		Changelog: "后端兜底更新说明",
	}
	if err := parseBeanListDisplaySummary(configRaw, contentRaw, &row); err != nil {
		t.Fatalf("parseBeanListDisplaySummary() err=%v", err)
	}
	if row.Title != "棵凡咖啡批发豆单" ||
		row.Subtitle != "报价不含税、不含运" ||
		row.ListTypeLabel != "商用" ||
		row.BrandName != "棵凡咖啡" ||
		row.BrandIntro != "源头工厂现烘现发" {
		t.Fatalf("display header=%+v", row)
	}
	if row.LayoutStyle != "table" ||
		row.CardsPerRow != 3 ||
		!row.ShowVersion ||
		row.ShowChangelog ||
		row.ShowCategoryNumbers ||
		row.BackgroundColor != "#f8f1e5" ||
		row.FontColor != "#171717" ||
		row.BackgroundImage != "/uploads/bean-bg.png" ||
		row.LogoImage != "/uploads/logo.png" {
		t.Fatalf("display style=%+v", row)
	}
	if len(row.Groups) != 1 || row.Groups[0].Category != "原产地精选豆" || row.Groups[0].ShowCategory || len(row.Groups[0].Items) != 1 {
		t.Fatalf("groups=%+v", row.Groups)
	}
	item := row.Groups[0].Items[0]
	if item.Code != "5.2" ||
		item.Name != "乌拉嘎" ||
		item.Badge != "new" ||
		item.BadgeLabel != "NEW" ||
		item.RecommendedUse == "" ||
		item.Flavor == "" ||
		item.Description == "" ||
		item.BeanListQuality.FactoryFlavorDescription != "茉莉花、柑橘" ||
		item.BeanListQuality.Moisture != "10.8%" ||
		item.BeanListQuality.Density != "780g/L" ||
		item.BeanListQuality.InspectionCreatedAt != "2026-05-18 09:30" ||
		len(item.HighlightTerms) != 2 ||
		item.HighlightTerms[0] != "乌拉嘎" {
		t.Fatalf("item=%+v", item)
	}
	if len(item.Prices) != 1 || item.Prices[0].Label != "454g" || item.Prices[0].Value != "118/包" || !item.Prices[0].Red {
		t.Fatalf("prices=%+v", item.Prices)
	}
	if row.CacheKey != "bean-list:7:V3.0.6" {
		t.Fatalf("cache_key=%q", row.CacheKey)
	}
}
