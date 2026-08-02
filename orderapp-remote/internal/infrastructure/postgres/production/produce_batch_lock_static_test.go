package production

import (
	"os"
	"strings"
	"testing"
)

func TestCreateProduceBatchLocksOrdersBeforeOrderItems(t *testing.T) {
	source, err := os.ReadFile("produce_batch.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func createProduceBatchFromOrders")
	if start < 0 {
		t.Fatal("createProduceBatchFromOrders missing")
	}
	contract := text[start:]
	orderLock := strings.Index(contract, "ORDER BY id\n\t\tFOR UPDATE")
	itemRead := strings.Index(contract, "FROM %s.order_items oi")
	if orderLock < 0 || itemRead < 0 || orderLock > itemRead {
		t.Fatal("legacy batch creation must lock sorted parent orders before reading and locking order_items")
	}
	if !strings.Contains(contract, "FOR UPDATE OF oi") {
		t.Fatal("legacy batch creation must retain the child order_items lock after the parent lock")
	}
}
