package stock

import (
	"context"
	"strings"
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

func TestCreateStockDocumentRejectsNonManufacturingPurposeBoundToWorkOrder(t *testing.T) {
	for _, purpose := range []string{
		PurposeMaterialReceipt,
		PurposeMaterialTransfer,
	} {
		t.Run(purpose, func(t *testing.T) {
			repo := &fakeStockDocumentRepository{fakeRepo: &fakeRepo{}}
			svc := NewService(repo)

			_, err := svc.CreateStockDocumentDraft(context.Background(), StockDocumentCommand{
				Purpose:     purpose,
				WorkOrderID: 88,
				Items: []StockDocumentItemCommand{{
					MaterialID: 1, QtyG: 1000,
				}},
			})
			if err == nil {
				t.Fatalf("purpose %q error = nil, want work-order purpose rejection", purpose)
			}
			if repo.draftCommand.Purpose != "" {
				t.Fatalf("purpose %q reached repository with command %+v", purpose, repo.draftCommand)
			}
		})
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

func TestStockDocumentRejectsMixedPositiveAndNegativeQuantities(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		qtyG     int64
		qtyUnits int64
	}{
		{name: "positive weight cannot hide negative count", qtyG: 60000, qtyUnits: -100},
		{name: "positive count cannot hide negative weight", qtyG: -100, qtyUnits: 60000},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeStockDocumentRepository{fakeRepo: &fakeRepo{}}
			_, err := NewService(repo).CreateStockDocumentDraft(context.Background(), StockDocumentCommand{
				Purpose: PurposeMaterialTransfer,
				Items: []StockDocumentItemCommand{{
					MaterialID: 1, QtyG: testCase.qtyG, QtyUnits: testCase.qtyUnits,
					FromWarehouse: "raw_materials", ToWarehouse: "wip",
				}},
			})
			if err == nil {
				t.Fatal("mixed positive and negative quantity must be rejected")
			}
			if !strings.Contains(err.Error(), "数量不能为负数") {
				t.Fatalf("error = %q, want Chinese negative-quantity explanation", err)
			}
			if len(repo.draftCommand.Items) != 0 {
				t.Fatalf("invalid quantity reached repository: %+v", repo.draftCommand)
			}
		})
	}
}

func TestStockDocumentAllowsNormal60KgMaterialTransfer(t *testing.T) {
	repo := &fakeStockDocumentRepository{fakeRepo: &fakeRepo{}}
	_, err := NewService(repo).CreateStockDocumentDraft(context.Background(), StockDocumentCommand{
		Purpose: PurposeMaterialTransfer,
		Items: []StockDocumentItemCommand{{
			MaterialID: 1, QtyG: 60000,
			FromWarehouse: "raw_materials", ToWarehouse: "wip",
		}},
	})
	if err != nil {
		t.Fatalf("normal 60Kg material transfer: %v", err)
	}
	if len(repo.draftCommand.Items) != 1 || repo.draftCommand.Items[0].QtyG != 60000 || repo.draftCommand.Items[0].QtyUnits != 0 {
		t.Fatalf("repository command = %+v", repo.draftCommand)
	}
}
