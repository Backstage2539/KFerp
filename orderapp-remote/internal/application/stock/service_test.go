package stock

import (
	"context"
	"testing"
)

type fakeRepo struct {
	ledgerQuery      LedgerQuery
	receipt          MaterialReceiptCommand
	adjustment       StockAdjustmentCommand
	transfer         MaterialTransferCommand
	finishedTransfer FinishedProductTransferCommand
	traceQuery       StockTraceQuery
}

func (f *fakeRepo) ListLedger(ctx context.Context, query LedgerQuery) (LedgerResult, error) {
	f.ledgerQuery = query
	return LedgerResult{}, nil
}
func (f *fakeRepo) ListBatches(ctx context.Context, query BatchQuery) (BatchResult, error) {
	return BatchResult{}, nil
}
func (f *fakeRepo) ListMaterialBatches(ctx context.Context, query MaterialBatchQuery) (MaterialBatchResult, error) {
	return MaterialBatchResult{}, nil
}
func (f *fakeRepo) ListWarehouses(ctx context.Context) ([]WarehouseRow, error) {
	return []WarehouseRow{}, nil
}
func (f *fakeRepo) ListMaterialBatchLocations(ctx context.Context, query MaterialBatchLocationQuery) (MaterialBatchLocationResult, error) {
	return MaterialBatchLocationResult{}, nil
}
func (f *fakeRepo) ListWarehouseInventory(ctx context.Context, query WarehouseInventoryQuery) (WarehouseInventoryResult, error) {
	return WarehouseInventoryResult{}, nil
}
func (f *fakeRepo) GetStockTrace(ctx context.Context, query StockTraceQuery) (StockTraceResult, error) {
	f.traceQuery = query
	return StockTraceResult{}, nil
}
func (f *fakeRepo) ReceiveMaterial(ctx context.Context, cmd MaterialReceiptCommand) (MaterialReceiptResult, error) {
	f.receipt = cmd
	return MaterialReceiptResult{ReceiptID: 7, BatchCode: "MB-0000000007"}, nil
}
func (f *fakeRepo) CreateAdjustment(ctx context.Context, cmd StockAdjustmentCommand) (StockAdjustmentResult, error) {
	f.adjustment = cmd
	return StockAdjustmentResult{AdjustmentID: 8}, nil
}
func (f *fakeRepo) TransferMaterial(ctx context.Context, cmd MaterialTransferCommand) (MaterialTransferResult, error) {
	f.transfer = cmd
	return MaterialTransferResult{TransferID: 9, TransferNo: "MT-0000000009"}, nil
}
func (f *fakeRepo) TransferFinishedProduct(ctx context.Context, cmd FinishedProductTransferCommand) (FinishedProductTransferResult, error) {
	f.finishedTransfer = cmd
	return FinishedProductTransferResult{TransferID: 10, TransferNo: "FT-0000000010"}, nil
}

func TestListLedgerNormalizesLimitAndFilters(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.ListLedger(context.Background(), LedgerQuery{Q: "  豆  ", Limit: 999, Offset: -2})
	if err != nil {
		t.Fatalf("ListLedger: %v", err)
	}

	if repo.ledgerQuery.Q != "豆" {
		t.Fatalf("query = %q, want trimmed", repo.ledgerQuery.Q)
	}
	if repo.ledgerQuery.Limit != 100 || repo.ledgerQuery.Offset != 0 {
		t.Fatalf("limit/offset = %d/%d, want 100/0", repo.ledgerQuery.Limit, repo.ledgerQuery.Offset)
	}
}

func TestReceiveMaterialRequiresPositiveQuantity(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.ReceiveMaterial(context.Background(), MaterialReceiptCommand{
		MaterialID: 1,
		QtyG:       0,
		UnitCost:   12.5,
		Operator:   "jj",
	})
	if err == nil {
		t.Fatal("expected quantity validation error")
	}
}

func TestReceiveMaterialDefaultsOperator(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	_, err := svc.ReceiveMaterial(context.Background(), MaterialReceiptCommand{
		MaterialID: 1,
		QtyG:       1000,
		UnitCost:   12.5,
	})
	if err != nil {
		t.Fatalf("ReceiveMaterial: %v", err)
	}
	if repo.receipt.Operator != "stock" {
		t.Fatalf("operator = %q, want stock", repo.receipt.Operator)
	}
}

func TestTransferMaterialNormalizesWarehouseAndDefaultsOperator(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.TransferMaterial(context.Background(), MaterialTransferCommand{
		MaterialID:    1,
		FromWarehouse: " raw_materials ",
		ToWarehouse:   " wip ",
		QtyG:          60000,
		Note:          "  三天生产领料  ",
	})
	if err != nil {
		t.Fatalf("TransferMaterial: %v", err)
	}

	if repo.transfer.FromWarehouse != "raw_materials" || repo.transfer.ToWarehouse != "wip" {
		t.Fatalf("warehouses = %q -> %q, want trimmed raw_materials -> wip", repo.transfer.FromWarehouse, repo.transfer.ToWarehouse)
	}
	if repo.transfer.Operator != "stock" {
		t.Fatalf("operator = %q, want stock", repo.transfer.Operator)
	}
	if repo.transfer.Note != "三天生产领料" {
		t.Fatalf("note = %q, want trimmed", repo.transfer.Note)
	}
}

func TestTransferMaterialRejectsSameWarehouse(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.TransferMaterial(context.Background(), MaterialTransferCommand{
		MaterialID:    1,
		FromWarehouse: "wip",
		ToWarehouse:   "wip",
		QtyG:          1000,
	})
	if err == nil {
		t.Fatal("expected same warehouse validation error")
	}
}

func TestTransferFinishedProductNormalizesWarehouseAndDefaultsOperator(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.TransferFinishedProduct(context.Background(), FinishedProductTransferCommand{
		ProductID:     9,
		SpecG:         454,
		FromWarehouse: " finished_goods ",
		ToWarehouse:   " finished_shop ",
		QtyUnits:      3,
		QtyLooseG:     10,
		Note:          " 门店备货 ",
	})
	if err != nil {
		t.Fatalf("TransferFinishedProduct: %v", err)
	}

	if repo.finishedTransfer.FromWarehouse != "finished_goods" || repo.finishedTransfer.ToWarehouse != "finished_shop" {
		t.Fatalf("warehouses = %q -> %q, want trimmed finished_goods -> finished_shop", repo.finishedTransfer.FromWarehouse, repo.finishedTransfer.ToWarehouse)
	}
	if repo.finishedTransfer.Operator != "stock" {
		t.Fatalf("operator = %q, want stock", repo.finishedTransfer.Operator)
	}
	if repo.finishedTransfer.Note != "门店备货" {
		t.Fatalf("note = %q, want trimmed", repo.finishedTransfer.Note)
	}
}

func TestTransferFinishedProductRejectsSameWarehouseAndZeroQuantity(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.TransferFinishedProduct(context.Background(), FinishedProductTransferCommand{
		ProductID:     9,
		SpecG:         454,
		FromWarehouse: "finished_goods",
		ToWarehouse:   "finished_goods",
		QtyUnits:      1,
	})
	if err == nil {
		t.Fatal("expected same warehouse validation error")
	}
	_, err = svc.TransferFinishedProduct(context.Background(), FinishedProductTransferCommand{
		ProductID:     9,
		SpecG:         454,
		FromWarehouse: "finished_goods",
		ToWarehouse:   "finished_shop",
	})
	if err == nil {
		t.Fatal("expected quantity validation error")
	}
}

func TestGetStockTraceRequiresBatchAndTrimsInput(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.GetStockTrace(context.Background(), StockTraceQuery{BatchCode: " FP-0000000042 "})
	if err != nil {
		t.Fatalf("GetStockTrace: %v", err)
	}
	if repo.traceQuery.BatchCode != "FP-0000000042" {
		t.Fatalf("trace batch = %q, want trimmed FP-0000000042", repo.traceQuery.BatchCode)
	}

	if _, err := svc.GetStockTrace(context.Background(), StockTraceQuery{}); err == nil {
		t.Fatal("expected batch validation error")
	}
}

func TestCreateAdjustmentAcceptsProductAliasForFinishedProduct(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.CreateAdjustment(context.Background(), StockAdjustmentCommand{
		ItemType:    " product ",
		ItemID:      9,
		SpecG:       454,
		TargetUnits: 3,
		Reason:      "门店盘点",
	})
	if err != nil {
		t.Fatalf("CreateAdjustment product alias: %v", err)
	}

	if repo.adjustment.ItemType != "finished_product" {
		t.Fatalf("item_type = %q, want finished_product", repo.adjustment.ItemType)
	}
	if repo.adjustment.Warehouse != "finished_goods" {
		t.Fatalf("warehouse = %q, want finished_goods", repo.adjustment.Warehouse)
	}
}
