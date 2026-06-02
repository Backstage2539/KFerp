package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev393ProductClassificationTemplateRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-393-PRODUCT-CLASSIFICATION-TEMPLATES",
		"DEV-393-CLASSIFICATION-SCHEMA-API",
		"DEV-393-PRODUCT-ALIAS-CLASSIFICATION",
		"DEV-393-DRAWER-STACK-BOM-RETURN",
		"DEV-393-INDUSTRY-FIELD-TEMPLATE-LOCK",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-393 seed missing %q", want)
		}
	}
}

func TestDev393ProductClassificationTemplateSchemaAndAPI(t *testing.T) {
	schema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go")))
	routes := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go")))
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go")))
	for _, want := range []string{
		"product_classification_templates",
		"product_classification_template_categories",
		"product_classification_assignments",
		"customer_product_alias_classification_assignments",
		"products ADD COLUMN IF NOT EXISTS classification_template_id",
		"customer_product_aliases ADD COLUMN IF NOT EXISTS classification_template_id",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("catalog schema missing classification marker %q", want)
		}
	}
	for _, want := range []string{
		"/api/product-classification-templates",
		"/api/product-classification-template-categories",
		"/api/product-classification-assignments/products",
		"/api/product-classification-assignments/customer-aliases",
		"ClassificationTemplateID",
	} {
		if !strings.Contains(routes, want) {
			t.Fatalf("catalog routes missing classification API marker %q", want)
		}
	}
	for _, want := range []string{
		"SaveProductClassificationTemplate",
		"SaveProductClassificationCategory",
		"SaveProductClassificationAssignment",
		"SaveCustomerProductAliasClassificationAssignment",
		"ensureCustomerClassificationTemplateTx",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("catalog repository missing classification marker %q", want)
		}
	}
}

func TestDev393ProductClassificationTemplateVue(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"分类模板",
		"classification-template-list",
		"classification-category-editor",
		"productClassificationTabs",
		"aliasClassificationTabs",
		"增加分类",
		"移动到分类",
		"返回商品档案配置",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("ProductSettingsView.vue missing PR-393 marker %q", want)
		}
	}
	for _, blocked := range []string{
		"productProductionConfigForm.product_subtype_category_id",
		"classification-config-drawer",
		"drawerStack",
		"新增字段",
		"行业字段值",
	} {
		if strings.Contains(src, blocked) {
			t.Fatalf("ProductSettingsView.vue should not expose legacy PR-393 marker %q", blocked)
		}
	}
}
