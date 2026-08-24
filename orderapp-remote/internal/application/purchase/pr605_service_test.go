package purchase

import (
	"context"
	"testing"
)

type pr605AtomicRepo struct {
	fakeRepo
	atomicCmd CreatePurchaseReceiptCommand
}

func (r *pr605AtomicRepo) CreatePurchaseReceiptAtomic(_ context.Context, cmd CreatePurchaseReceiptCommand) (PurchaseReceipt, error) {
	r.atomicCmd = cmd
	return PurchaseReceipt{ID: 605, ReceiptNo: "PRC-605", Qty: cmd.Qty, QtyUnits: cmd.QtyUnits, UnitCode: cmd.UnitCode, TargetWarehouse: cmd.TargetWarehouse}, nil
}

func TestPR605PurchaseOrderAndReceiptAcceptDiscreteInventoryUnit(t *testing.T) {
	repo := &pr605AtomicRepo{}
	svc := NewService(repo, nil)

	if _, err := svc.CreatePurchaseOrder(context.Background(), CreatePurchaseOrderCommand{
		SupplierID: 2, MaterialID: 7, Qty: 120, UnitCode: "袋", QtyUnits: 120, UnitCost: 1.25, TargetWarehouse: "packaging",
	}); err != nil {
		t.Fatalf("CreatePurchaseOrder discrete unit: %v", err)
	}
	receipt, err := svc.CreatePurchaseReceipt(context.Background(), CreatePurchaseReceiptCommand{
		PurchaseOrderID: 3, SupplierID: 2, MaterialID: 7, Qty: 118, UnitCode: "袋", QtyUnits: 118,
		UnitCost: 1.3, TargetWarehouse: "packaging", Operator: "warehouse",
	})
	if err != nil {
		t.Fatalf("CreatePurchaseReceipt discrete unit: %v", err)
	}
	if receipt.QtyUnits != 118 || receipt.UnitCode != "袋" || receipt.TargetWarehouse != "packaging" {
		t.Fatalf("receipt = %+v", receipt)
	}
	if repo.atomicCmd.QtyUnits != 118 || repo.atomicCmd.TargetWarehouse != "packaging" {
		t.Fatalf("atomic command = %+v", repo.atomicCmd)
	}

	if _, err := svc.CreatePurchaseReceipt(context.Background(), CreatePurchaseReceiptCommand{
		SupplierID: 2, MaterialID: 7, Qty: 8, UnitCode: "袋", QtyUnits: 9, UnitCost: 1.3,
	}); err == nil {
		t.Fatal("mismatched qty and qty_units must be rejected")
	}
}
