package sales

import (
	"strings"
	"testing"
)

func TestOrderItemsInsertSQLKeepsPriceOverrideAndProductKindAligned(t *testing.T) {
	query := orderItemsInsertSQL("tenant_test")
	if strings.Contains(query, "NULLIF($16,0)") {
		t.Fatalf("order item insert must bind product_kind directly; query still shifts parameter 16: %s", query)
	}
	if !strings.Contains(query, "price_overridden,product_kind,bean_list_publication_id") {
		t.Fatalf("order item insert columns lost the canonical order: %s", query)
	}
	if !strings.Contains(query, "VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17") {
		t.Fatalf("order item insert values must map price_overridden=$15 and product_kind=$16: %s", query)
	}
}
