package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev171BomSkuContextRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-171",
		"DEV-171-01",
		"DEV-171-02",
		"UT-171-01",
		"API-171-01",
		"REV-171-01",
		"BOM配置按 SKU归属 过滤",
		"默认公共SKU",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 171 bom sku context seed missing %q", want)
		}
	}
}

func TestDev171BomViewFiltersBySkuContext(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue")))
	for _, want := range []string{
		"bom-sku-context-panel",
		"SKU归属",
		"selectedBomCustomerSkuCustomerID",
		"bomSkuContextLabel",
		"customBomProductCustomerIDs",
		"bomSkuCustomers",
		"bomContextProducts",
		"bomContextRows",
		"bomContextProductFilter",
		":options=\"bomContextProducts\"",
		"v-for=\"row in bomContextRows\"",
		"暂无公共SKU BOM",
		"暂无客户SKU BOM",
		"filterBomContextProducts(rows.value, 0)",
		"bomContextCustomerIDs(products.value, rows.value)",
		"apiGet('/api/customers?limit=200')",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("BomView.vue missing BOM SKU context marker %q", want)
		}
	}
	contextPanel := strings.Index(view, "bom-sku-context-panel")
	productSelect := strings.Index(view, "placeholder=\"选择商品\"")
	if contextPanel < 0 || productSelect < 0 || contextPanel > productSelect {
		t.Fatalf("BOM SKU context panel must appear above product selector: context=%d productSelect=%d", contextPanel, productSelect)
	}
}

func TestDev171BomAPICarriesProductCustomerScope(t *testing.T) {
	rels := []string{
		filepath.Join("internal", "application", "bom", "service.go"),
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"),
	}
	for _, rel := range rels {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"CustomerID", "customer_id"} {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing BOM customer scope marker %q", rel, want)
			}
		}
	}
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go")))
	for _, want := range []string{
		"COALESCE(p.customer_id,0)",
		"SELECT p.id, p.name, COALESCE(p.customer_id,0)",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("BOM repository missing customer scope query marker %q", want)
		}
	}
}

func TestDev171ManualsDocumentBomSkuContextOperation(t *testing.T) {
	rels := []string{
		"docs/REQUIREMENTS.md",
		"docs/ACCEPTANCE_TESTS.md",
		"docs/OP_MANUAL_INVENTORY_MATERIALS.md",
	}
	root := filepath.Join(findAncestorForTest(t, "go.mod"), "..")
	for _, name := range []string{"REQUIREMENTS.md", "ACCEPTANCE_TESTS.md", "OP_MANUAL_INVENTORY_MATERIALS.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			rels = append(rels, filepath.Join("..", name))
		}
	}
	for _, rel := range rels {
		doc := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{
			"BOM配置",
			"SKU归属",
			"公共SKU",
			"客户SKU",
			"BOM 配方维护",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing BOM SKU context manual marker %q", rel, want)
			}
		}
	}
}
