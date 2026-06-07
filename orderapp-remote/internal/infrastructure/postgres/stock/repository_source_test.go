package stock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWarehouseCustomerFilterIncludesGlobalWarehouses(t *testing.T) {
	src := string(readStockSourceForTest(t, "repository.go"))
	for _, want := range []string{
		"COALESCE(w.customer_id,0) IN (0, $1::bigint)",
		"($1::bigint=0 OR COALESCE(w.customer_id,0) IN (0, $1::bigint))",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("warehouse list SQL should include global warehouses for customer-scoped queries, missing %q", want)
		}
	}
}

func TestWarehouseInventoryGroupingUsesWarehouseBusinessGroupAssignments(t *testing.T) {
	repository := string(readStockSourceForTest(t, "repository.go"))
	service := string(readStockSourceForTest(t, filepath.Join("..", "..", "..", "application", "stock", "service.go")))
	combined := repository + "\n" + service
	for _, want := range []string{
		"GroupID",
		"GroupItemID",
		"GroupSource",
		"business_group_assignments",
		"lower(bga.usage_key)='warehouse_inventory'",
		"lower(bga.object_key)='warehouse'",
		"object_ref=w.code",
		"query.GroupID",
		"query.GroupItemID",
	} {
		if !strings.Contains(combined, want) {
			t.Fatalf("warehouse inventory group assignment implementation missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"object_id=w.id",
		"object_key='warehouse_inventory_row'",
		"object_key='stock_batch'",
	} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("warehouse inventory grouping must be by warehouse code, not row/batch; found %q", forbidden)
		}
	}
}

func readStockSourceForTest(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(".", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
