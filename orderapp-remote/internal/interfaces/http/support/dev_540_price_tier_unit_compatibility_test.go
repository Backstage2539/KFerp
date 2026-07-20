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

	// PR-540 remains historical delivery evidence. PR-541 supersedes its
	// runtime kg/lb blocking contract: new rows freeze the concrete SKU sales
	// specification and interpret tier bounds as counts of that specification.
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "application", "costing", "service.go"): {
			"sales_spec_count", "effective_sales_spec", "tier_quantity_unit", "applyFlatRowEffectiveSalesSpec",
		},
		filepath.Join("internal", "infrastructure", "postgres", "costing", "repository.go"): {
			"derived_spec_key", "&input.SpecKey", "beanListContentProductIDs", "sku_id",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "bean-list-pdf.js"): {
			"quantity_basis", "effective_sales_spec", "tier_quantity_unit",
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

	currentPDFHelper := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "bean-list-pdf.js")))
	for _, want := range []string{
		"row.quantity_basis !== 'sales_spec_count'",
		"row.tier_unit_compatible === false",
		"flatRowsForPdfItem(item, blockedRows)",
	} {
		if !strings.Contains(currentPDFHelper, want) {
			t.Fatalf("PR-541 must ignore PR-540 compatibility metadata only for new count rows while retaining the legacy preview guard; missing %q", want)
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
