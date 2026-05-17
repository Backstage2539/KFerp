package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev287OrderFeeKgRepairRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-287-ORDER-LIST-FEE-KG-SUBMISSION-REPAIR",
		"DEV-287-CUSTOMER-FULFILLMENT-KG-DISPLAY-PRICING",
		"DEV-287-ERP-ORDER-LIST-FEE-STACK-PARITY",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 287 order fee kg repair seed missing %q", want)
		}
	}
}

func TestDev287OrderFeeKgRepairSourceWiring(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository.go"): {
			"customerFulfillmentTierQuantityForSpec",
			"customerFulfillmentDisplayUnitPriceFromLb",
			"customerFulfillmentLineTotalFromDisplayUnit(unitPrice, specG, item.QuantityUnits)",
		},
		filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go"): {
			"TestCustomerFulfillmentSubmittedPricingUsesKgDisplayUnitAndTotals",
			"TestSubmitCustomerDirectShipOrderUsesKgTierPriceAsDisplayUnit",
			"1000g x 25",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "OrdersView.vue"): {
			"customerFulfillmentOrderFees(row)",
			"emphasized",
			"委外合计",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "view-routing.test.js"): {
			"customerFulfillmentOrderFees",
			"emphasized",
		},
	}

	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 287 marker %q", rel, want)
			}
		}
	}
}

func TestDev287OrderFeeKgRepairManualsAndAcceptanceDocs(t *testing.T) {
	checks := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-287-ORDER-LIST-FEE-KG-SUBMISSION-REPAIR",
			"按该行总 KG 匹配梯度",
			"履约运营台订单费用一致",
		},
		filepath.Join("..", "REQUIREMENTS.md"): {
			"PR-287-ORDER-LIST-FEE-KG-SUBMISSION-REPAIR",
			"按该行总 KG 匹配梯度",
			"履约运营台订单费用一致",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-287-ORDER-LIST-FEE-KG-SUBMISSION-REPAIR",
			"82 元/kg",
			"应收 2109",
		},
		filepath.Join("..", "ACCEPTANCE_TESTS.md"): {
			"PR-287-ORDER-LIST-FEE-KG-SUBMISSION-REPAIR",
			"82 元/kg",
			"应收 2109",
		},
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"): {
			"履约运营台订单费用的同一结构",
			"1000g × 25 袋",
		},
		filepath.Join("..", "OP_MANUAL_ORDER_SALES.md"): {
			"履约运营台订单费用的同一结构",
			"1000g × 25 袋",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"): {
			"1000g × 25 袋",
			"同一套订单费用结构",
		},
		filepath.Join("..", "OP_MANUAL_CUSTOMER_FULFILLMENT.md"): {
			"1000g × 25 袋",
			"同一套订单费用结构",
		},
	}

	for rel, wants := range checks {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing dev 287 manual marker %q", rel, want)
			}
		}
	}
}
