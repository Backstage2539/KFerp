package support

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev539PricingMarkupDrawerContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_product", code: "PR-539-PRICING-MARKUP-DRAWER", status: "review"},
		{table: "req_dev", code: "DEV-539-MARKUP-ONLY", status: "done"},
		{table: "req_dev", code: "DEV-539-PRICING-RULE-DRAWER", status: "done"},
		{table: "req_dev", code: "DEV-539-DOCS-DEPLOY", status: "done"},
		{table: "req_review", code: "REV-539-PRICING-MARKUP-DRAWER", status: "todo"},
	} {
		requireDev539SeedRow(t, reqStore, row.table, row.code, row.status)
	}

	for rel, wants := range map[string][]string{
		filepath.Join("internal", "application", "catalog", "service_test.go"): {
			"TestProductPricingRuleUsesMarkupAsTheOnlyPricingMethod",
			"TestProductPricingRuleRejectsCleanUpdateOverQuarantinedExistingTemplate",
			"TestListProductPricingRulesNormalizesLegacyMethodsWithoutChangingPublishedPrices",
		},
		filepath.Join("internal", "application", "costing", "service_test.go"): {
			"TestPricingRuleTrialUsesMarkupForLegacyAndCurrentTemplates",
			"TestPricingRuleTrialRejectsUnsupportedLegacyFixedAddTemplate",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_settings_api_test.go"): {
			"TestProductPricingRuleAPINormalizesLegacyWholePercentToMarkup",
			"TestProductPricingRuleAPIRejectsCleanUpdateOverQuarantinedExistingTemplate",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository_test.go"): {
			"TestPricingRuleMarkupOnlyMigrationIsSafeAndIdempotent",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"pricingRuleEditorDrawerOpen", "pricing-rule-editor-drawer", "aria-label=\"\u4ef7\u683c\u8ba1\u7b97\u6a21\u677f\u7f16\u8f91\"",
			"\u52a0\u4ef7\u7387\uff0880%=0.8\uff09", "\u7a0e\u524d\u4ef7 = \u6210\u672c\u57fa\u6570 \u00d7 (1 + \u52a0\u4ef7\u7387)", "\u6700\u7ec8\u552e\u4ef7\u518d\u8ba1\u7b97\u7a0e\u989d\u548c\u53d6\u6574",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"const profitMethod = !rawProfitMethod", "['gross_margin', 'markup'].includes(rawProfitMethod)",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.test.js"): {
			"copied pricing rule should open the editor drawer", "openPricingRuleEditorDrawer()", "role=\"dialog\"",
		},
		filepath.Join("..", "ACTIVE_REQUIREMENTS.md"): {
			"PR-539-PRICING-MARKUP-DRAWER", "DEV-539-MARKUP-ONLY", "DEV-539-PRICING-RULE-DRAWER", "development only", "production not authorized",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-539-PRICING-MARKUP-DRAWER", "\u6210\u672c\u57fa\u6570 \u00d7 (1 + \u52a0\u4ef7\u7387)", "\u6210\u672c 100", "44.44%", "fixed_add", "\u53f3\u4fa7\u6253\u5f00",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-539-PRICING-MARKUP-DRAWER", "profit_method=markup", "\u6700\u7ec8\u4ef7\u5747\u4e3a 180", "\u53f3\u4fa7\u6253\u5f00", "\u5df2\u53d1\u5e03\u4ef7\u683c\u8868\u5feb\u7167",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-539-PRICING-MARKUP-DRAWER", "gross_margin", "80` \u89c4\u8303\u4e3a `0.8", "\u8ba2\u5355\u884c\u51bb\u7ed3\u4ef7\u683c", "\u4ef7\u683c\u8ba1\u7b97\u6a21\u677f\u7f16\u8f91\u62bd\u5c49",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"K81. \u4ef7\u683c\u8ba1\u7b97\u6a21\u677f\u7edf\u4e00\u52a0\u4ef7\u7387", "gross_margin=80", "markup=1.2", "\u8fc7\u671f\u9519\u8bef\u6216\u6210\u529f\u63d0\u793a",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-539 \u4ef7\u683c\u8ba1\u7b97\u6a21\u677f\u52a0\u4ef7\u7387\u4e0e\u7f16\u8f91\u62bd\u5c49", "80% \u586b `0.8`", "\u7a0e\u524d\u4ef7 = \u6210\u672c\u57fa\u6570 \u00d7 (1 + \u52a0\u4ef7\u7387)", "\u5b9e\u9645\u6bdb\u5229\u7387", "\u53f3\u4fa7\u6253\u5f00\u540c\u4e00\u7f16\u8f91\u62bd\u5c49",
		},
		filepath.Join("docs", "acceptance", "2026-07-17-pricing-markup-drawer.md"): {
			"PR-539 \u4ef7\u683c\u8ba1\u7b97\u6a21\u677f\u7edf\u4e00\u52a0\u4ef7\u7387\u4e0e\u53f3\u4fa7\u7f16\u8f91\u62bd\u5c49\u9a8c\u6536", "RED\uff08\u5b9e\u73b0\u524d\uff09", "GREEN\uff08\u5b9e\u73b0\u540e\u5b9a\u5411\u9a8c\u8bc1\uff09", "\u751f\u4ea7\u73af\u5883\u672a\u90e8\u7f72\u3001\u672a\u5199\u5165\u3001\u672a\u5207\u6362\u5165\u53e3",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-539 marker %q", rel, want)
			}
		}
	}

	schema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go")))
	migration := dev539Between(schema, "-- PR-539 pricing rules use markup only.", "-- PR-539 pricing rules use markup only end.")
	if migration == "" {
		t.Fatal("catalog schema missing bounded PR-539 pricing-rule migration")
	}
	for _, forbidden := range []string{"bean_list_publications", "order_items", "orders", "final_unit_price"} {
		if strings.Contains(migration, forbidden) {
			t.Fatalf("PR-539 migration must not rewrite frozen history table/field %q", forbidden)
		}
	}

	rootRequirements := string(readOrderAppFileForTest(t, filepath.Join("..", "REQUIREMENTS.md")))
	docsRequirements := string(readOrderAppFileForTest(t, filepath.Join("docs", "REQUIREMENTS.md")))
	if got, want := dev539MarkdownSectionBody(rootRequirements, "PR-539-PRICING-MARKUP-DRAWER"), dev539MarkdownSectionBody(docsRequirements, "PR-539-PRICING-MARKUP-DRAWER"); got != want {
		t.Fatal("root and orderapp-remote/docs PR-539 requirement bodies must stay mirrored")
	}

	rootAcceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "ACCEPTANCE_TESTS.md")))
	docsAcceptance := string(readOrderAppFileForTest(t, filepath.Join("docs", "ACCEPTANCE_TESTS.md")))
	if got, want := dev539MarkdownSectionBody(rootAcceptance, "PR-539-PRICING-MARKUP-DRAWER"), dev539MarkdownSectionBody(docsAcceptance, "PR-539-PRICING-MARKUP-DRAWER"); got != want {
		t.Fatal("root and orderapp-remote/docs PR-539 acceptance bodies must stay mirrored")
	}
}

func requireDev539SeedRow(t *testing.T, src, table, code, status string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s", table, code, status)
	}
}

func dev539Between(src, startMarker, endMarker string) string {
	start := strings.Index(src, startMarker)
	if start < 0 {
		return ""
	}
	end := strings.Index(src[start+len(startMarker):], endMarker)
	if end < 0 {
		return ""
	}
	return src[start : start+len(startMarker)+end+len(endMarker)]
}

func dev539MarkdownSectionBody(src, marker string) string {
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
