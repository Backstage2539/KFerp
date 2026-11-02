package stock

import (
	"context"
	"strings"
	"testing"
)

func TestOrdinaryTransferRequiresSelectedSourceBatch(t *testing.T) {
	for _, tc := range []struct {
		name string
		item StockDocumentItemCommand
	}{
		{"material", StockDocumentItemCommand{MaterialID: 1, ItemType: itemTypeMaterial, QtyG: 1000}},
		{"finished product", StockDocumentItemCommand{ProductID: 2, ItemType: itemTypeFinishedProduct, SpecG: 454, QtyUnits: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &fakeStockDocumentRepository{fakeRepo: &fakeRepo{}}
			tc.item.FromWarehouse, tc.item.ToWarehouse = "raw_materials", "wip"
			_, err := NewService(repo).CreateStockDocumentDraft(context.Background(), StockDocumentCommand{Purpose: PurposeMaterialTransfer, Items: []StockDocumentItemCommand{tc.item}})
			if err == nil || !strings.Contains(err.Error(), "批次") {
				t.Fatalf("missing source batch must be rejected with a batch-specific error, got %v", err)
			}
			if repo.draftCommand.Purpose != "" {
				t.Fatal("invalid draft reached repository")
			}
		})
	}
}

func TestLegacyTransferEntryPointsForwardSourceBatch(t *testing.T) {
	repo := &fakeStockDocumentRepository{fakeRepo: &fakeRepo{}}
	svc := NewService(repo)
	_, err := svc.TransferMaterial(context.Background(), MaterialTransferCommand{MaterialID: 1, QtyG: 1000, FromWarehouse: "raw_materials", ToWarehouse: "wip", BatchCode: " MB-2 "})
	if err != nil || repo.draftCommand.Items[0].BatchCode != "MB-2" {
		t.Fatalf("material transfer batch not forwarded: command=%+v err=%v", repo.draftCommand, err)
	}
	_, err = svc.TransferFinishedProduct(context.Background(), FinishedProductTransferCommand{ProductID: 2, SpecG: 454, QtyUnits: 1, FromWarehouse: "finished_goods", ToWarehouse: "sales", BatchCode: " FP-2 "})
	if err != nil || repo.draftCommand.Items[0].BatchCode != "FP-2" {
		t.Fatalf("finished transfer batch not forwarded: command=%+v err=%v", repo.draftCommand, err)
	}
}
