package customerportal

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
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
		"content_json",
		"parseBeanListContentSummary",
		"BeanListGroupSummary",
		"BeanListProductSummary",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("customer portal bean list content display missing %q", want)
		}
	}
}

func TestCustomerPortalBeanListSummariesExposePDFCacheMetadata(t *testing.T) {
	body, err := os.ReadFile("business_repository.go")
	if err != nil {
		t.Fatalf("read business_repository.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"PDFURL",
		"CacheKey",
		"beanListPDFPath",
		"beanListCacheKey",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("bean list PDF cache metadata missing %q", want)
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

func TestParseBeanListContentSummaryExtractsGroupsItemsAndPrices(t *testing.T) {
	raw, err := json.Marshal(map[string]any{
		"groups": []any{
			map[string]any{
				"category": "原产地精选豆",
				"items": []any{
					map[string]any{
						"code":           "5.2",
						"name":           "乌拉嘎",
						"recommendedUse": "手冲/SOE/冷萃",
						"flavor":         "柑橘/莓果",
						"prices": []any{
							map[string]any{"label": "454g", "price": 118, "unit": "包"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	groups, err := parseBeanListContentSummary(raw)
	if err != nil {
		t.Fatalf("parseBeanListContentSummary() err=%v", err)
	}
	if len(groups) != 1 || groups[0].Category != "原产地精选豆" || len(groups[0].Items) != 1 {
		t.Fatalf("groups=%+v", groups)
	}
	item := groups[0].Items[0]
	if item.Code != "5.2" || item.Name != "乌拉嘎" || item.RecommendedUse == "" || item.Flavor == "" {
		t.Fatalf("item=%+v", item)
	}
	if len(item.Prices) != 1 || item.Prices[0].Label != "454g" || item.Prices[0].Value != "¥118/包" {
		t.Fatalf("prices=%+v", item.Prices)
	}
}
