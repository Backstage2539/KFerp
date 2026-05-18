package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev169ProductSettingsSkuListRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-169",
		"DEV-169-01",
		"DEV-169-02",
		"UT-169-01",
		"API-169-01",
		"REV-169-01",
		"客户SKU列表",
		"默认展示公共SKU",
		"履约客户",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 169 product settings SKU list seed missing %q", want)
		}
	}
}

func TestDev169ProductSettingsShowsUnifiedSkuList(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue")))
	for _, want := range []string{
		"客户SKU列表",
		"公共SKU",
		"selectedCustomerSkuCustomerID",
		"customerSkuCustomerOptions",
		"customerSkuCustomers",
		"/api/customer-fulfillment/customers?limit=200",
		"displaySkuRows",
		"v-for=\"row in displaySkuRows\"",
		":disabled=\"!displaySkuRows.length\"",
		":options=\"customerSkuCustomers\"",
		"Number(product.customer_id || 0) === 0",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("ProductSettingsView.vue missing unified SKU list marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"class=\"customer-sku-list\"",
		"客户专属 SKU 列表",
	} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("ProductSettingsView.vue should merge the separate customer SKU list into the unified SKU list, still found %q", forbidden)
		}
	}
}

func TestDev169ManualsDocumentUnifiedSkuListOperation(t *testing.T) {
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
			"客户SKU列表",
			"公共SKU",
			"履约客户",
		} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing unified SKU list manual marker %q", rel, want)
			}
		}
	}
}
