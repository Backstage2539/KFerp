package purchase

import (
	"context"
	"testing"

	stockapp "orderapp/internal/application/stock"
)

type fakeRepo struct {
	supplierCmd     SaveSupplierCommand
	orderCmd        CreatePurchaseOrderCommand
	receiptCmd      CreatePurchaseReceiptCommand
	priceMaterialID int64
	priceUnitCost   float64
}

func (f *fakeRepo) ListSuppliers(ctx context.Context) ([]Supplier, error) {
	return []Supplier{{ID: 1, Name: "生豆供应商", Active: true}}, nil
}

func (f *fakeRepo) SaveSupplier(ctx context.Context, cmd SaveSupplierCommand) (Supplier, error) {
	f.supplierCmd = cmd
	return Supplier{ID: 2, Name: cmd.Name, Contact: cmd.Contact, Phone: cmd.Phone, Active: true}, nil
}

func (f *fakeRepo) ListPurchaseOrders(ctx context.Context) ([]PurchaseOrder, error) {
	return nil, nil
}

func (f *fakeRepo) CreatePurchaseOrder(ctx context.Context, cmd CreatePurchaseOrderCommand) (PurchaseOrder, error) {
	f.orderCmd = cmd
	return PurchaseOrder{ID: 3, OrderNo: "PO-20260428-0001", SupplierID: cmd.SupplierID, Status: "ordered"}, nil
}

func (f *fakeRepo) ListPurchaseReceipts(ctx context.Context) ([]PurchaseReceipt, error) {
	return nil, nil
}

func (f *fakeRepo) CreatePurchaseReceipt(ctx context.Context, cmd CreatePurchaseReceiptCommand, stockResult stockapp.MaterialReceiptResult) (PurchaseReceipt, error) {
	f.receiptCmd = cmd
	return PurchaseReceipt{ID: 4, ReceiptNo: "PRC-20260428-0001", PurchaseOrderID: cmd.PurchaseOrderID, MaterialID: cmd.MaterialID, StockBatchCode: stockResult.BatchCode}, nil
}

func (f *fakeRepo) UpdateMaterialPurchasePrice(ctx context.Context, materialID int64, unitCost float64) error {
	f.priceMaterialID = materialID
	f.priceUnitCost = unitCost
	return nil
}

type fakeStock struct {
	cmd stockapp.MaterialReceiptCommand
}

func (f *fakeStock) ReceiveMaterial(ctx context.Context, cmd stockapp.MaterialReceiptCommand) (stockapp.MaterialReceiptResult, error) {
	f.cmd = cmd
	return stockapp.MaterialReceiptResult{ReceiptID: 9, BatchID: 10, BatchCode: "MB-0000000009"}, nil
}

func TestPurchaseServiceCreatesSupplierAndPurchaseOrder(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo, nil)

	supplier, err := svc.SaveSupplier(context.Background(), SaveSupplierCommand{
		Name:    " 生豆供应商 ",
		Contact: " 张三 ",
		Phone:   " 13800000000 ",
		Address: " 上海 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if supplier.ID != 2 || repo.supplierCmd.Name != "生豆供应商" || repo.supplierCmd.Contact != "张三" {
		t.Fatalf("supplier=%+v cmd=%+v", supplier, repo.supplierCmd)
	}

	order, err := svc.CreatePurchaseOrder(context.Background(), CreatePurchaseOrderCommand{
		SupplierID: 2,
		MaterialID: 7,
		QtyG:       3000,
		UnitCost:   42.5,
		Operator:   " buyer ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.ID != 3 || repo.orderCmd.Operator != "buyer" || repo.orderCmd.QtyG != 3000 {
		t.Fatalf("order=%+v cmd=%+v", order, repo.orderCmd)
	}
}

func TestPurchaseReceiptReceivesStockAndUpdatesMaterialPurchasePrice(t *testing.T) {
	repo := &fakeRepo{}
	stock := &fakeStock{}
	svc := NewService(repo, stock)

	receipt, err := svc.CreatePurchaseReceipt(context.Background(), CreatePurchaseReceiptCommand{
		PurchaseOrderID: 3,
		SupplierID:      2,
		SupplierName:    " 生豆供应商 ",
		MaterialID:      7,
		QtyG:            2500,
		UnitCost:        48,
		Note:            " 首批到货 ",
		Operator:        " warehouse ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.ID != 4 || receipt.StockBatchCode != "MB-0000000009" {
		t.Fatalf("receipt=%+v", receipt)
	}
	if stock.cmd.MaterialID != 7 || stock.cmd.Supplier != "生豆供应商" || stock.cmd.QtyG != 2500 || stock.cmd.UnitCost != 48 || stock.cmd.Operator != "warehouse" {
		t.Fatalf("stock command = %+v", stock.cmd)
	}
	if repo.priceMaterialID != 7 || repo.priceUnitCost != 48 {
		t.Fatalf("price update material=%d cost=%.2f", repo.priceMaterialID, repo.priceUnitCost)
	}
	if repo.receiptCmd.Note != "首批到货" {
		t.Fatalf("receipt command = %+v", repo.receiptCmd)
	}
}
