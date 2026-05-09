package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev166OrderPriceDisplayUnitRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-166",
		"DEV-166-01",
		"DEV-166-02",
		"UT-166-01",
		"API-166-01",
		"REV-166-01",
		"106 元/kg",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 166 order price display unit seed missing %q", want)
		}
	}
}

func TestDev166OrderEntryUsesSpecDisplayPriceUnit(t *testing.T) {
	orderEntry := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js")))
	for _, want := range []string{
		"export function wholesalePriceUnit",
		"return { label: '元/kg', suffix: '/kg', unitG: 1000 }",
		"Math.round(price)",
		"rowQuantityForWholesalePriceUnit(row)",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("order-entry.js missing display price unit marker %q", want)
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"`单价（${priceUnitLabel(row)}）`",
		"wholesaleTierPriceRows(productByID(row.product_id), row)",
		"unitPriceMoney(tier.unitPrice)",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("OrderEntryView.vue missing display price unit marker %q", want)
		}
	}
}

func TestDev166SaveOrderStoresDisplayUnitPrice(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go")))
	for _, want := range []string{
		"func wholesaleDisplayUnitG",
		"return math.Round(price)",
		"wholesaleDisplayUnitPriceFromLb(pricePerLb, items[idx].specG)",
		"wholesaleLineTotalFromDisplayUnit(items[idx].unitPrice, items[idx].specG, items[idx].units)",
		"wholesaleLineTotalFromDisplayUnit(*items[idx].manualPrice, items[idx].specG, items[idx].units)",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("sales repository missing display unit price marker %q", want)
		}
	}
}

func TestDev166ManualDocumentsOrderPriceDisplayUnit(t *testing.T) {
	rels := []string{
		"docs/OP_MANUAL_ORDER_SALES.md",
		"docs/REQUIREMENTS.md",
		"docs/ACCEPTANCE_TESTS.md",
	}
	root := filepath.Join(findAncestorForTest(t, "go.mod"), "..")
	for _, name := range []string{"OP_MANUAL_ORDER_SALES.md", "REQUIREMENTS.md", "ACCEPTANCE_TESTS.md"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			rels = append(rels, filepath.Join("..", name))
		}
	}
	for _, rel := range rels {
		doc := string(readOrderAppFileForTest(t, rel))
		for _, want := range []string{"元/kg", "106", "3180"} {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing order price display unit manual marker %q", rel, want)
			}
		}
	}
}
