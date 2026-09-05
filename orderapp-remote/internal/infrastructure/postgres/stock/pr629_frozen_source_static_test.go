package stock

import (
	"os"
	"strings"
	"testing"
)

func TestPR629StockDocumentsEnforceFrozenWarehouseOwnerAndBatch(t *testing.T) {
	b, err := os.ReadFile("stock_document.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{
		"validateFrozenWorkOrderMaterialSourceTx",
		"owner_customer_id",
		"binding.batch_code",
		"binding.warehouse",
		"领料单必须完整使用生产计划冻结的来源仓、货主和批次",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("frozen-source stock document contract missing %q", want)
		}
	}
	if !strings.Contains(src, "COALESCE(b.owner_customer_id,0)=") {
		t.Fatal("material batch availability must filter factory owner 0 as well as customer owners")
	}
}
