package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev310OrderEntryUnitAmountDiscountRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-310-ORDER-ENTRY-UNIT-AMOUNT-DISCOUNT",
		"DEV-310-ORDER-ENTRY-UNIT-AMOUNT-DISCOUNT",
		"UT-310-ORDER-ENTRY-UNIT-AMOUNT-DISCOUNT",
		"API-310-ORDER-ENTRY-UNIT-AMOUNT-DISCOUNT",
		"REV-310-ORDER-ENTRY-UNIT-AMOUNT-DISCOUNT",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("dev 309 seed missing %q", want)
		}
	}
}

func TestDev310OrderEntrySupportsUnitAmountDiscount(t *testing.T) {
	view := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "OrderEntryView.vue")))
	for _, want := range []string{
		`<option value="unit_amount">单价优惠</option>`,
		"discountValuePlaceholder(row)",
		"每条商品明细可选择减免数额、单价优惠、折扣或免费",
	} {
		if !strings.Contains(view, want) {
			t.Fatalf("OrderEntryView.vue missing unit amount discount marker %q", want)
		}
	}

	orderEntry := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "lib", "order-entry.js")))
	for _, want := range []string{
		"export function rowUnitDiscountUnits",
		"type === 'unit_amount'",
		"['amount', 'unit_amount', 'percent'].includes(row.discount_type)",
	} {
		if !strings.Contains(orderEntry, want) {
			t.Fatalf("order-entry.js missing unit amount discount marker %q", want)
		}
	}
}

func TestDev310OrderSaveSupportsUnitAmountDiscount(t *testing.T) {
	repo := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go")))
	for _, want := range []string{
		`case "unit_amount", "unit", "unit_discount", "per_unit", "unit_price":`,
		"func orderItemUnitDiscountUnits",
		`case "unit_amount":`,
		"discountValue*unitCount",
	} {
		if !strings.Contains(repo, want) {
			t.Fatalf("sales repository missing unit amount discount marker %q", want)
		}
	}
}

func TestDev310OrderManualsDocumentUnitAmountDiscount(t *testing.T) {
	for _, rel := range []string{
		filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md"),
		filepath.Join("docs", "REQUIREMENTS.md"),
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"),
	} {
		doc := string(readOrderAppFileForTest(t, rel))
		if !strings.Contains(doc, "单价优惠") {
			t.Fatalf("%s missing unit amount discount documentation", rel)
		}
		if rel != filepath.Join("docs", "OP_MANUAL_ORDER_SALES.md") && !strings.Contains(doc, "PR-310-ORDER-ENTRY-UNIT-AMOUNT-DISCOUNT") {
			t.Fatalf("%s missing PR-310 unit amount discount marker", rel)
		}
	}
}
