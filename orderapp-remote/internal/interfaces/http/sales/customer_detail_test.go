package sales

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCustomerDetailKeepsStoredLineAmountWithoutInternalPricingTrace(t *testing.T) {
	data := customerEditDataForAPI(&OrderEditData{Items: []OrderEditItem{{LineTotal: "123.45", UnitPrice: "12.345", Qty: "10", PriceSourceJSON: `{"quantity_basis":"sales_spec_count","publication_id":51,"cost_source_snapshot":{"secret":"internal-cost"}}`}}}, true)
	raw, _ := json.Marshal(data)
	if !strings.Contains(string(raw), `"line_total":"123.45"`) {
		t.Fatal("stored line amount missing")
	}
	if strings.Contains(string(raw), "internal-cost") || strings.Contains(string(raw), "production_source_trace") || strings.Contains(string(raw), "quote_source_trace") {
		t.Fatal("internal pricing exposed")
	}
}
