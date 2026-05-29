package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev356CustomerProductRuleTemplatesRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-356-CUSTOMER-PRODUCT-RULE-TEMPLATES",
		"DEV-356-CUSTOMER-PRODUCT-RULE-TEMPLATES",
		"UT-356-CUSTOMER-PRODUCT-RULE-TEMPLATES",
		"API-356-CUSTOMER-PRODUCT-RULE-TEMPLATES",
		"REV-356-CUSTOMER-PRODUCT-RULE-TEMPLATES",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer product rule template seed missing %q", want)
		}
	}
}

func TestDev356CustomerProductRuleTemplatesWiring(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "domain", "catalog", "product_rule_resolution.go"): {
			"ResolveProductRuleConfig",
			"CustomerOverride",
			"CustomerTemplate",
			"ProductSubtypeDefault",
			"ProductTypeDefault",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"customer_product_rule_templates",
			"customer_product_rule_template_items",
			"customer_product_rule_overrides",
		},
		filepath.Join("internal", "infrastructure", "postgres", "core", "schema.go"): {
			"customer_product_rule_template_id",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer product rule marker %q", rel, want)
			}
		}
	}
}

func TestDev356CustomerProductRuleTemplatesDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-356-CUSTOMER-PRODUCT-RULE-TEMPLATES",
			"客户产品规则模板",
			"客户专属覆盖",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-356-CUSTOMER-PRODUCT-RULE-TEMPLATES",
			"客户专属覆盖",
			"产品子类型",
		},
		filepath.Join("docs", "acceptance", "2026-05-24-customer-product-rule-templates.md"): {
			"PR-356-CUSTOMER-PRODUCT-RULE-TEMPLATES",
			"客户专属覆盖",
			"产品价格表",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer product rule docs marker %q", rel, want)
			}
		}
	}
}
