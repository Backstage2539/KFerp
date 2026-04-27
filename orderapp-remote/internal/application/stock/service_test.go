package stock

import (
	"context"
	"testing"
)

type fakeRepo struct {
	ledgerQuery LedgerQuery
	receipt     MaterialReceiptCommand
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
func (f *fakeRepo) ReceiveMaterial(ctx context.Context, cmd MaterialReceiptCommand) (MaterialReceiptResult, error) {
	f.receipt = cmd
	return MaterialReceiptResult{ReceiptID: 7, BatchCode: "MB-0000000007"}, nil
}
func (f *fakeRepo) CreateAdjustment(ctx context.Context, cmd StockAdjustmentCommand) (StockAdjustmentResult, error) {
	return StockAdjustmentResult{AdjustmentID: 8}, nil
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
