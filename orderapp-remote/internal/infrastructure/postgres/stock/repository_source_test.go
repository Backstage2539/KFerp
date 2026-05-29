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

func readStockSourceForTest(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(".", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
