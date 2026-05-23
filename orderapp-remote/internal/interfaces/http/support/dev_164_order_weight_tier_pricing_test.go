package support

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDev164OrderWeightTierPricingRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-164",
		"DEV-164-01",
		"DEV-164-02",
		"UT-164-01",
		"API-164-01",
		"REV-164-01",
		"1000g × 30 袋",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 164 order weight-tier pricing seed missing %q", want)
		}
	}
}

func TestDev164OrderEntryFallsBackToBeanListWeightTiers(t *testing.T) {
	orderEntry := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js")))
	for _, want := range []string{
		"rowQuantityLb(row)",
		"matchTierByQuantityResult(tiers, rowQuantityLb(row), tierMinLb, tierMaxLb)",
		"wholesaleTierUnitPriceLb(tier)",
		"rowQuantityForWholesalePriceUnit(row)",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("order-entry.js missing weight-tier pricing marker %q", want)
		}
	}

	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		"priceUnitLabel(row)",
		"unitPriceMoney(tier.unitPrice)",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("OrderEntryView.vue missing wholesale lb price marker %q", want)
		}
	}
}

func TestDev164SaveOrderUsesBeanListWeightTierFallback(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go")))
	for _, want := range []string{
		"bean-list weight tiers",
		"COALESCE(NULLIF(min_qty_lb,0), NULLIF(min_qty_units,0) * COALESCE(NULLIF(spec_g,0),454) / 454.0, 0) <= $2",
		"wholesaleLineTotalFromDisplayUnit",
		"WHERE id=$1 AND active=true",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("sales repository missing weight-tier fallback marker %q", want)
		}
	}
}

func TestDev164ManualDocumentsOrderWeightTierPricing(t *testing.T) {
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
		wants := []string{"454"}
		if strings.Contains(rel, "OP_MANUAL_ORDER_SALES") {
			wants = append(wants, "规格没有专属梯度", "元/kg", "1000g × 30")
		} else {
			wants = append(wants, "1000g", "30")
		}
		for _, want := range wants {
			if !strings.Contains(doc, want) {
				t.Fatalf("%s missing order weight-tier pricing manual marker %q", rel, want)
			}
		}
	}
}
