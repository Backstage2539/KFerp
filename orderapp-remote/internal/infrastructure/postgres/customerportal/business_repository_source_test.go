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
