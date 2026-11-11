package production

import (
	app "orderapp/internal/application/production"
	"testing"
)

func TestReconcileDirectTransferDrafts(t *testing.T) {
	desired := []app.PlanMaterialTransferItem{{SourceID: 3, Warehouse: "raw", OwnerCustomerID: 7, Batches: []app.PlanMaterialTransferBatch{{BatchCode: "B1", QtyG: 10000}}}}
	selected := map[int64]bool{3: true}
	key := directTransferKey{source: 3, warehouse: "raw", owner: 7, batch: "B1"}
	t.Run("matching partial", func(t *testing.T) {
		out, ids, err := reconcileDirectTransferDrafts(desired, selected, []directPendingLine{{document: 10, key: key, grams: 4000}})
		if err != nil || len(ids) != 1 || len(out) != 1 || out[0].Batches[0].QtyG != 6000 {
			t.Fatalf("out=%+v ids=%v err=%v", out, ids, err)
		}
	})
	t.Run("mixed document rejected", func(t *testing.T) {
		_, _, err := reconcileDirectTransferDrafts(desired, selected, []directPendingLine{{document: 10, key: key, grams: 4000}, {document: 10, key: directTransferKey{source: 4}, grams: 1}})
		if err == nil {
			t.Fatal("mixed document accepted")
		}
	})
	t.Run("different batch rejected", func(t *testing.T) {
		_, _, err := reconcileDirectTransferDrafts(desired, selected, []directPendingLine{{document: 10, key: directTransferKey{source: 3, warehouse: "raw", owner: 7, batch: "B2"}, grams: 1000}})
		if err == nil {
			t.Fatal("different batch accepted")
		}
	})
	t.Run("too much rejected", func(t *testing.T) {
		_, _, err := reconcileDirectTransferDrafts(desired, selected, []directPendingLine{{document: 10, key: key, grams: 11000}})
		if err == nil {
			t.Fatal("over transfer accepted")
		}
	})
}
