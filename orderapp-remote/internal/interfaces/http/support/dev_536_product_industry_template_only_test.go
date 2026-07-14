package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev536ProductIndustryTemplateOnlyContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY",
			"DEV-536-FRONTEND-TEMPLATE-PROJECTION",
			"DEV-536-BACKEND-TEMPLATE-CONSTRAINT",
			"DEV-536-LEGACY-FIELD-CLEANUP",
		},
		filepath.Join("docs", "REQUIREMENTS.md"):                                            {"PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", "无行业字段模板时字段必须为空"},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"):                                        {"PR-536-PRODUCT-INDUSTRY-TEMPLATE-ONLY", "模板外历史字段"},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"):                           {"取消行业字段模板会清空商品行业字段"},
		filepath.Join("docs", "acceptance", "2026-07-14-product-industry-template-only.md"): {"PR-536 商品行业字段仅来源于模板验收"},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-536 marker %q", rel, want)
			}
		}
	}
}
