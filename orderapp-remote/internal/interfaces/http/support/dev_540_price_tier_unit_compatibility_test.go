package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev540PriceTierUnitCompatibilityContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_product", code: "PR-540-PRICE-TIER-UNIT-COMPATIBILITY", status: "review"},
		{table: "req_dev", code: "DEV-540-TIER-UNIT-COMPATIBILITY", status: "done"},
		{table: "req_dev", code: "DEV-540-PUBLISH-UNIT-GUARD", status: "done"},
		{table: "req_dev", code: "DEV-540-DOCS-DEPLOY", status: "done"},
		{table: "req_review", code: "REV-540-PRICE-TIER-UNIT-COMPATIBILITY", status: "todo"},
	} {
		requireDev540SeedRow(t, reqStore, row.table, row.code, row.status)
	}

	for rel, wants := range map[string][]string{
		filepath.Join("internal", "application", "costing", "service.go"): {
			"validatePriceTierTemplateUnitCompatibility", "ResolvePriceTierTemplateUnitRule", "DefaultSalesUnit", "阶梯模板不可用",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"ResolvePriceTierTemplateUnitRule", "quantity_unit", "default_sales_unit",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"priceTierTemplateUnitCompatibility", "tier_unit_compatible", "阶梯模板不可用",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"priceListTierTemplateOptionDisabled", "product-picker-tier-warning", "priceListTierTemplateCompatibilityForItem", "priceListProductTierTemplateWarning", "productCurrentSalesSpecUnit",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "bean-list-pdf.js"): {
			"blockedRows", "tier_unit_compatibility_error", "tier_unit_compatible",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-540-PRICE-TIER-UNIT-COMPATIBILITY", "初晓", "可换算", "阶梯模板不可用", "历史已发布价格表快照",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-540-PRICE-TIER-UNIT-COMPATIBILITY", "商品规格“磅”与阶梯规格“kg”不匹配", "后端", "手工改价",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-540-PRICE-TIER-UNIT-COMPATIBILITY", "lbs", "公斤", "客户别名", "Pricing Rule 试算",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K82. 商品销售规格与阶梯模板数量单位严格匹配", "继承", "PDF", "历史已发布价格表快照",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-540 商品销售规格与阶梯模板单位校验", "磅", "kg", "重新选择匹配规格的阶梯模板",
		},
		filepath.Join("docs", "acceptance", "2026-07-19-price-tier-unit-compatibility.md"): {
			"PR-540 商品销售规格与阶梯模板单位兼容验收", "RED（实现前）", "GREEN（实现后定向验证）", "生产环境未部署、未写入、未切换入口",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-540 marker %q", rel, want)
			}
		}
	}

	rootRequirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	docsRequirements := string(readOrderAppFileForTest(t, filepath.Join("docs", "REQUIREMENTS.md")))
	if got, want := dev540MarkdownSectionBody(rootRequirements, "PR-540-PRICE-TIER-UNIT-COMPATIBILITY"), dev540MarkdownSectionBody(docsRequirements, "PR-540-PRICE-TIER-UNIT-COMPATIBILITY"); got != want {
		t.Fatal("root and orderapp-remote/docs PR-540 requirement bodies must stay mirrored")
	}

	rootAcceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	docsAcceptance := string(readOrderAppFileForTest(t, filepath.Join("docs", "ACCEPTANCE_TESTS.md")))
	if got, want := dev540MarkdownSectionBody(rootAcceptance, "PR-540-PRICE-TIER-UNIT-COMPATIBILITY"), dev540MarkdownSectionBody(docsAcceptance, "PR-540-PRICE-TIER-UNIT-COMPATIBILITY"); got != want {
		t.Fatal("root and orderapp-remote/docs PR-540 acceptance bodies must stay mirrored")
	}
}

func requireDev540SeedRow(t *testing.T, src, table, code, status string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s", table, code, status)
	}
}

func dev540MarkdownSectionBody(src, marker string) string {
	markerIndex := strings.Index(src, marker)
	if markerIndex < 0 {
		return ""
	}
	headingEnd := strings.Index(src[markerIndex:], "\n")
	if headingEnd < 0 {
		return ""
	}
	bodyStart := markerIndex + headingEnd + 1
	body := src[bodyStart:]
	if nextHeading := strings.Index(body, "\n##"); nextHeading >= 0 {
		body = body[:nextHeading]
	}
	return strings.TrimSpace(body)
}
