package stock

import (
	"context"
	"testing"
)

type fakeStockDocumentRepository struct {
	*fakeRepo
	draftCommand  StockDocumentCommand
	updateID      int64
	updateCommand StockDocumentCommand
	submitID      int64
	cancelID      int64
}

func (f *fakeStockDocumentRepository) CreateStockDocumentDraft(_ context.Context, cmd StockDocumentCommand) (StockDocumentDetail, error) {
	f.draftCommand = cmd
	return StockDocumentDetail{StockDocumentRow: StockDocumentRow{ID: 21, EntryNo: "SE-0000000021", Purpose: cmd.Purpose, IsReturn: cmd.IsReturn, Status: "draft"}}, nil
}

func (f *fakeStockDocumentRepository) UpdateStockDocumentDraft(_ context.Context, id int64, cmd StockDocumentCommand) (StockDocumentDetail, error) {
	f.updateID, f.updateCommand = id, cmd
	return StockDocumentDetail{StockDocumentRow: StockDocumentRow{ID: id, EntryNo: "SE-0000000021", Purpose: cmd.Purpose, IsReturn: cmd.IsReturn, Status: "draft"}}, nil
}

func (f *fakeStockDocumentRepository) SubmitStockDocument(_ context.Context, id int64, actor string) (StockDocumentDetail, error) {
	f.submitID = id
	return StockDocumentDetail{StockDocumentRow: StockDocumentRow{ID: id, EntryNo: "SE-0000000021", Purpose: PurposeMaterialTransferForManufacture, Status: "submitted", Operator: actor}}, nil
}

func (f *fakeStockDocumentRepository) CancelStockDocument(_ context.Context, id int64, actor string) (StockDocumentDetail, error) {
	f.cancelID = id
	return StockDocumentDetail{StockDocumentRow: StockDocumentRow{ID: id, EntryNo: "SE-0000000021", Purpose: PurposeMaterialTransferForManufacture, Status: "cancelled", Operator: actor}}, nil
}

func (f *fakeStockDocumentRepository) ListStockDocuments(_ context.Context, query StockDocumentQuery) (StockDocumentResult, error) {
	return StockDocumentResult{Rows: []StockDocumentRow{{ID: 21, EntryNo: "SE-0000000021", Purpose: query.Purpose}}}, nil
}

func (f *fakeStockDocumentRepository) GetStockDocument(_ context.Context, id int64) (StockDocumentDetail, error) {
	return StockDocumentDetail{StockDocumentRow: StockDocumentRow{ID: id, EntryNo: "SE-0000000021", Status: "draft"}}, nil
}

func (f *fakeStockDocumentRepository) CreateAndSubmitStockDocument(_ context.Context, cmd StockDocumentCommand) (StockDocumentDetail, error) {
	f.draftCommand = cmd
	return StockDocumentDetail{StockDocumentRow: StockDocumentRow{ID: 22, EntryNo: "SE-0000000022", Purpose: cmd.Purpose, IsReturn: cmd.IsReturn, Status: "submitted"}}, nil
}

func TestCreateStockDocumentNormalizesLegacyReturnPurpose(t *testing.T) {
	repo := &fakeStockDocumentRepository{fakeRepo: &fakeRepo{}}
	svc := NewService(repo)

	detail, err := svc.CreateStockDocumentDraft(context.Background(), StockDocumentCommand{
		Purpose:     " material_return_from_manufacture ",
		WorkOrderID: 88,
		Items: []StockDocumentItemCommand{{
			MaterialID:    7,
			QtyG:          1200,
			FromWarehouse: " raw_materials ",
			ToWarehouse:   " wip ",
		}},
	})
	if err != nil {
		t.Fatalf("CreateStockDocumentDraft: %v", err)
	}
	if detail.Purpose != PurposeMaterialTransferForManufacture || !detail.IsReturn {
		t.Fatalf("detail purpose/return = %q/%t", detail.Purpose, detail.IsReturn)
	}
	if repo.draftCommand.Items[0].FromWarehouse != "wip" || repo.draftCommand.Items[0].ToWarehouse != "raw_materials" {
		t.Fatalf("return warehouses = %q -> %q, want wip -> raw_materials", repo.draftCommand.Items[0].FromWarehouse, repo.draftCommand.Items[0].ToWarehouse)
	}
}

func TestCreateStockDocumentNormalizesLegacyFinishedTransferPurpose(t *testing.T) {
	repo := &fakeStockDocumentRepository{fakeRepo: &fakeRepo{}}
	svc := NewService(repo)

	if _, err := svc.CreateStockDocumentDraft(context.Background(), StockDocumentCommand{
		Purpose: "finished_transfer",
		Items: []StockDocumentItemCommand{{
			ProductID: 9, ItemType: itemTypeFinishedProduct, SpecG: 1000, QtyUnits: 2,
			FromWarehouse: "finished_goods", ToWarehouse: "sales",
		}},
	}); err != nil {
		t.Fatalf("CreateStockDocumentDraft: %v", err)
	}
	if repo.draftCommand.Purpose != PurposeMaterialTransfer {
		t.Fatalf("legacy finished transfer purpose = %q, want %q", repo.draftCommand.Purpose, PurposeMaterialTransfer)
	}
}

func TestStockDocumentLifecycleDelegatesWithoutSubmittingDraft(t *testing.T) {
	repo := &fakeStockDocumentRepository{fakeRepo: &fakeRepo{}}
	svc := NewService(repo)

	draft, err := svc.CreateStockDocumentDraft(context.Background(), StockDocumentCommand{
		Purpose: PurposeMaterialTransfer,
		Items: []StockDocumentItemCommand{{
			MaterialID: 1, QtyG: 1000, FromWarehouse: "raw_materials", ToWarehouse: "wip",
		}},
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	if draft.Status != "draft" || repo.submitID != 0 {
		t.Fatalf("draft = %+v submitID=%d, want unposted draft", draft, repo.submitID)
	}
	submitted, err := svc.SubmitStockDocument(context.Background(), draft.ID, " jj ")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != "submitted" || repo.submitID != draft.ID || submitted.Operator != "jj" {
		t.Fatalf("submitted = %+v submitID=%d", submitted, repo.submitID)
	}
	cancelled, err := svc.CancelStockDocument(context.Background(), draft.ID, " jj ")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if cancelled.Status != "cancelled" || repo.cancelID != draft.ID {
		t.Fatalf("cancelled = %+v cancelID=%d", cancelled, repo.cancelID)
	}
}

func TestStockDocumentRejectsAdjustmentAndInvalidLines(t *testing.T) {
	svc := NewService(&fakeStockDocumentRepository{fakeRepo: &fakeRepo{}})
	if _, err := svc.CreateStockDocumentDraft(context.Background(), StockDocumentCommand{
		Purpose: "stock_adjustment",
		Items:   []StockDocumentItemCommand{{MaterialID: 1, QtyG: 1}},
	}); err == nil {
		t.Fatal("stock adjustment must remain isolated from Stock Entry")
	}
	if _, err := svc.CreateStockDocumentDraft(context.Background(), StockDocumentCommand{
		Purpose: PurposeMaterialTransfer,
		Items:   []StockDocumentItemCommand{{MaterialID: 1, QtyG: 0, FromWarehouse: "raw_materials", ToWarehouse: "wip"}},
	}); err == nil {
		t.Fatal("zero quantity must be rejected")
	}
}
