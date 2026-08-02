package sales

import (
	"os"
	"strings"
	"testing"
)

func TestCreateOrderShipmentLocksSortedOrdersBeforeCreatingShipment(t *testing.T) {
	source, err := os.ReadFile("shipment.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func (r Repository) CreateOrderShipment")
	if start < 0 {
		t.Fatal("CreateOrderShipment missing")
	}
	contract := text[start:]
	orderLock := strings.Index(contract, "lockShipmentOrdersTx")
	revisionCheck := strings.Index(contract, "verifyShipmentOrderRevisionsTx")
	shipmentInsert := strings.Index(contract, "INSERT INTO %s.order_shipments")
	if orderLock < 0 || revisionCheck < 0 || shipmentInsert < 0 || orderLock > revisionCheck || revisionCheck > shipmentInsert {
		t.Fatal("shipment creation must lock orders and verify export revisions before creating the shipment snapshot/association")
	}
	for _, want := range []string{"ORDER BY id", "FOR UPDATE"} {
		if !strings.Contains(text, want) {
			t.Fatalf("shipment order lock missing %q", want)
		}
	}
}
