package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev170ProductSettingsCustomerContextRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-170",
		"DEV-170-01",
		"DEV-170-02",
		"DEV-170-03",
		"UT-170-01",
		"API-170-01",
		"REV-170-01",
		"PR-PRODUCT-SKU-BEANLIST-SPLIT-001",
		"DEV-PRODUCT-SKU-BEANLIST-SPLIT-001",
		"客户上下文置顶",
		"客户自己的商品分类同步切换",
		"产品豆单独立选择客户",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 170 customer context seed missing %q", want)
		}
	}
}

func TestDev170ProductSettingsLayoutUsesTopLevelCustomerContext(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"sku-context-panel",
		"SKU归属",
		"selectedSkuContextLabel",
		"categoryTreeForSkuContext",
		"contextCategorizedProductIDs",
		"skuContextProductFilter",
		"v-for=\"primary in visibleCategoryManagementTreeForSkuContext\"",
		"customer_id: selectedCustomerSkuCustomerID.value",
		"价格表生成请进入产品价格表",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("ProductSettingsView.vue missing customer context marker %q", want)
		}
	}
	if strings.Contains(view, "<CostingView") {
		t.Fatalf("ProductSettingsView.vue must not embed product bean-list workspace after SKU/product bean-list split")
	}
	contextPanel := strings.Index(view, "sku-context-panel")
	publicCreate := strings.Index(view, "product-editor-drawer")
	if contextPanel < 0 || publicCreate < 0 || contextPanel > publicCreate {
		t.Fatalf("SKU customer context panel must appear above product create panels: context=%d create=%d", contextPanel, publicCreate)
	}
}

func TestDev170CostingViewAcceptsCustomerContext(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue")))
	for _, want := range []string{
		"defineProps",
		"customerContextId",
		"customerContextLabel",
		"syncPublicationScopeFromPageContext",
		"activeBeanListCustomerID",
		"visibleCostingItems",
		"versionListScopeCustomerID(versionListScope.value)",
		"publicationScope.value = 'customer'",
		"selectedBeanListCustomerID.value = pageCustomerID",
		"filterBeanListItemsForPriceTableScope(items.value, activeCostingScope.value",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("CostingView.vue missing product settings customer context marker %q", want)
		}
	}
}

func TestDev170ProductCategoriesAreCustomerScopedInAPI(t *testing.T) {
	rels := []string{
		filepath.Join("internal", "application", "catalog", "service.go"),
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"),
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"),
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"),
	}
	for _, rel := range rels {
		src := string(readOrderAppFileForTest(t, rel))
		wants := []string{"CustomerID", "customer_id"}
		if strings.HasSuffix(rel, filepath.Join("postgres", "catalog", "schema.go")) {
			wants = []string{"customer_id"}
		}
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer-scoped category marker %q", rel, want)
			}
		}
	}
	schema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go")))
	for _, want := range []string{
		"ALTER TABLE %[1]s.product_categories ADD COLUMN IF NOT EXISTS customer_id",
		"product_categories_customer_parent_name_uniq",
		"customer_id, COALESCE(parent_id,0), lower(name)",
	} {
		if !strings.Contains(schema, want) {
			t.Fatalf("product category schema missing customer scope marker %q", want)
		}
	}
}

func TestDev170ManualsDocumentCustomerContextOperation(t *testing.T) {
	rels := []string{
		"docs/REQUIREMENTS.md",
		"docs/ACCEPTANCE_TESTS.md",
		"docs/OP_MANUAL_INVENTORY_MATERIALS.md",
		"docs/OP_MANUAL_COSTING.md",
	}
	root := filepath.Join(findAncestorForTest(t, "go.mod"), "..")
	for _, name := range []string{"REQUIREMENTS.md", "ACCEPTANCE_TESTS.md", "OP_MANUAL_INVENTORY_MATERIALS.md", "OP_MANUAL_COSTING.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			rels = append(rels, filepath.Join("..", name))
		}
	}
	for _, rel := range rels {
		doc := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"客户上下文",
			"商品管理",
			"客户自己的商品分类",
			"价格表归属",
			"产品价格表",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing customer context manual marker %q", rel, want)
			}
		}
	}
}
