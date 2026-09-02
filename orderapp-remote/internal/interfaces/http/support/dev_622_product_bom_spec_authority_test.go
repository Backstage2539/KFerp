package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev622RequirementAndManualContracts(t *testing.T) {
	for rel, markers := range map[string][]string{
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-622-PRODUCT-BOM-SPEC-AUTHORITY", "DEV-622-BOM-SPEC-ONLY", "DEV-622-DUAL-ENV-CLEANUP",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"商品规格权威完全收敛到生产 BOM", "product_output_requires_bom_spec", "product_bom_spec_not_configured",
		},
		filepath.Join("docs", "OP_MANUAL_PRODUCTION.md"): {
			"商品 BOM 统一规格组（PR-622", "全局单位字典", "不允许直接商品组件",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"商品档案与 BOM 规格", "未配置 BOM 规格", "商品档案不再保存销售规格模板",
		},
	} {
		text := string(readOrderAppFileForTest(t, rel))
		for _, marker := range markers {
			if !strings.Contains(text, marker) {
				t.Fatalf("%s missing PR-622 marker %q", rel, marker)
			}
		}
	}
}

func TestDev622RetiredSpecificationRoutesAreAbsent(t *testing.T) {
	text := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go")))
	for _, route := range []string{
		`e.POST("/api/product-settings/unit-templates"`,
		`e.POST("/api/product-settings/skus"`,
		`e.PUT("/api/product-settings/products/:id/default-sku"`,
	} {
		if strings.Contains(text, route) {
			t.Fatalf("retired product specification route remains registered: %s", route)
		}
	}

	migrationAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "productspecmigration", "api.go")))
	for _, route := range []string{"/bom-spec-migration", "/prepare", "/readiness", "/cutover"} {
		if strings.Contains(migrationAPI, route) {
			t.Fatalf("retired BOM specification migration route remains: %s", route)
		}
	}
}
