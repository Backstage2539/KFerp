package customerfulfillment

import (
	"context"
	"strings"
	"testing"
)

func TestNormalizeMiniDirectShipListQueryDefaultsAndBounds(t *testing.T) {
	got, err := normalizeMiniDirectShipListQuery(MiniDirectShipListQuery{
		CustomerID:  9,
		Q:           "  甲店   张三  ",
		ShippedFrom: " 2026-08-01 ",
		ShippedTo:   " 2026-08-07 ",
		Page:        -3,
		Limit:       1000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.CustomerID != 9 || got.Q != "甲店 张三" || got.ShippedFrom != "2026-08-01" || got.ShippedTo != "2026-08-07" {
		t.Fatalf("normalized list query = %#v", got)
	}
	if got.Page != 1 || got.Limit != 100 {
		t.Fatalf("normalized pagination page=%d limit=%d, want 1/100", got.Page, got.Limit)
	}

	defaults, err := normalizeMiniDirectShipListQuery(MiniDirectShipListQuery{CustomerID: 9})
	if err != nil {
		t.Fatal(err)
	}
	if defaults.Page != 1 || defaults.Limit != 50 {
		t.Fatalf("default pagination page=%d limit=%d, want legacy-compatible 1/50", defaults.Page, defaults.Limit)
	}
}

func TestListCustomerCentralInventoryFiltersPaginatesAndPreservesLegacyAll(t *testing.T) {
	repo := &miniDirectShipApplicationRepositoryStub{
		fakeCustomerFulfillmentRepository: &fakeCustomerFulfillmentRepository{},
		inventoryRows: []CustomerInventorySummary{
			{ProductID: 551, ProductName: "乌拉嘎 227g", SKUCode: "WLG-227", SpecG: 227},
			{ProductID: 552, ProductName: "乌拉嘎 454g", SKUCode: "WLG-454", SpecG: 454},
			{ProductID: 911, ProductName: "萨其姆 生豆", SKUCode: "SKU-000911", SpecG: 1000},
		},
	}
	svc := NewService(repo)

	page, err := svc.ListCustomerCentralInventory(context.Background(), CustomerInventoryListQuery{
		CustomerID: 9,
		Q:          " 乌 拉嘎 ",
		Page:       2,
		Limit:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.inventoryCustomerID != 9 {
		t.Fatalf("repository customer=%d, want 9", repo.inventoryCustomerID)
	}
	if page.Total != 2 || page.TotalPages != 2 || page.Page != 2 || page.Limit != 1 || page.HasNext || len(page.Rows) != 1 || page.Rows[0].ProductID != 552 {
		t.Fatalf("inventory page = %#v", page)
	}

	legacy, err := svc.ListCustomerCentralInventory(context.Background(), CustomerInventoryListQuery{
		CustomerID: 9,
		LegacyAll:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Total != 3 || legacy.TotalPages != 1 || legacy.Page != 1 || legacy.Limit != 3 || legacy.HasNext || len(legacy.Rows) != 3 {
		t.Fatalf("legacy inventory result = %#v", legacy)
	}
}

func TestListCustomerCentralInventoryBatchesPreservesCanonicalTraceVariantAndLegacyIdentity(t *testing.T) {
	repo := &miniDirectShipApplicationRepositoryStub{fakeCustomerFulfillmentRepository: &fakeCustomerFulfillmentRepository{}}
	svc := NewService(repo)

	canonical := CustomerInventoryBatchQuery{CustomerID: 9, ProductID: 550, BomSpecID: 91, BomVariantID: 191}
	rows, err := svc.ListCustomerCentralInventoryBatches(context.Background(), canonical)
	if err != nil {
		t.Fatal(err)
	}
	if repo.batchQuery != canonical || len(rows) != 1 || rows[0].BomSpecID != 91 || rows[0].BomVariantID != 191 || rows[0].SpecG != 0 {
		t.Fatalf("canonical query=%#v rows=%#v", repo.batchQuery, rows)
	}

	legacy := CustomerInventoryBatchQuery{CustomerID: 9, ProductID: 551, SpecG: 227}
	if _, err := svc.ListCustomerCentralInventoryBatches(context.Background(), legacy); err != nil {
		t.Fatal(err)
	}
	if repo.batchQuery != legacy {
		t.Fatalf("legacy query=%#v", repo.batchQuery)
	}

	for _, invalid := range []CustomerInventoryBatchQuery{
		{CustomerID: 9, ProductID: 550},
		{CustomerID: 9, ProductID: 550, BomVariantID: 191},
		{CustomerID: 9, ProductID: 550, BomSpecID: 91, SpecG: 227},
	} {
		if _, err := svc.ListCustomerCentralInventoryBatches(context.Background(), invalid); err == nil {
			t.Fatalf("invalid batch identity accepted: %#v", invalid)
		}
	}
}

func TestNormalizeMiniDirectShipListQueryRejectsInvalidShipmentDates(t *testing.T) {
	tests := []struct {
		name  string
		query MiniDirectShipListQuery
		want  string
	}{
		{name: "missing customer", query: MiniDirectShipListQuery{}, want: "customer required"},
		{name: "invalid from", query: MiniDirectShipListQuery{CustomerID: 9, ShippedFrom: "2026-02-30"}, want: "shipped_from invalid"},
		{name: "invalid to", query: MiniDirectShipListQuery{CustomerID: 9, ShippedTo: "08/07/2026"}, want: "shipped_to invalid"},
		{name: "reversed", query: MiniDirectShipListQuery{CustomerID: 9, ShippedFrom: "2026-08-08", ShippedTo: "2026-08-07"}, want: "shipment date range invalid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeMiniDirectShipListQuery(tc.query)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestNormalizeMiniDirectShipItemsMergesSameSKUAndSpec(t *testing.T) {
	got, err := normalizeMiniDirectShipItems([]MiniDirectShipItemCommand{
		{ProductID: 911, SpecG: 1000, Qty: 2},
		{ProductID: 912, SpecG: 227, Qty: 1},
		{ProductID: 911, SpecG: 1000, Qty: 3},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProductID != 911 || got[0].Qty != 5 || got[1].ProductID != 912 || got[1].Qty != 1 {
		t.Fatalf("normalized items = %#v", got)
	}
}

func TestNormalizeMiniDirectShipItemsUsesParentAndBOMSpecIdentityOneToOne(t *testing.T) {
	got, err := normalizeMiniDirectShipItems([]MiniDirectShipItemCommand{
		{ProductID: 91, BomSpecID: 801, BomVariantID: 901, InventoryUnit: "袋", Qty: 2},
		{ProductID: 91, BomSpecID: 801, BomVariantID: 901, InventoryUnit: "袋", Qty: 3},
		{ProductID: 91, BomSpecID: 802, BomVariantID: 902, InventoryUnit: "盒", Qty: 1},
	})
	if err != nil {
		t.Fatalf("normalize canonical items: %v", err)
	}
	if len(got) != 2 || got[0].Qty != 5 || got[0].SpecG != 0 || got[0].BomSpecID != 801 || got[0].BomVariantID != 901 || got[0].InventoryUnit != "袋" {
		t.Fatalf("canonical normalized items=%#v", got)
	}
}

func TestNormalizeMiniDirectShipItemsAllowsServerResolvedVariantAndRejectsInvalidIdentity(t *testing.T) {
	got, err := normalizeMiniDirectShipItems([]MiniDirectShipItemCommand{{ProductID: 91, BomSpecID: 801, Qty: 1}})
	if err != nil || len(got) != 1 || got[0].BomSpecID != 801 || got[0].BomVariantID != 0 {
		t.Fatalf("server-resolved identity=%#v err=%v", got, err)
	}
	for _, item := range []MiniDirectShipItemCommand{
		{ProductID: 91, BomVariantID: 901, Qty: 1},
		{ProductID: 91, BomSpecID: 801, BomVariantID: 901, SpecG: 227, InventoryUnit: "袋", Qty: 1},
	} {
		if _, err := normalizeMiniDirectShipItems([]MiniDirectShipItemCommand{item}); err == nil {
			t.Fatalf("partial/mixed identity unexpectedly accepted: %#v", item)
		}
	}
}

func TestNormalizeMiniDirectShipItemsRejectsInvalidConcreteSKU(t *testing.T) {
	for _, items := range [][]MiniDirectShipItemCommand{
		nil,
		{{ProductID: 0, SpecG: 1000, Qty: 1}},
		{{ProductID: 911, SpecG: 0, Qty: 1}},
		{{ProductID: 911, SpecG: 1000, Qty: 0}},
	} {
		if _, err := normalizeMiniDirectShipItems(items); err == nil {
			t.Fatalf("items %#v should be rejected", items)
		}
	}
}

type miniDirectShipApplicationRepositoryStub struct {
	*fakeCustomerFulfillmentRepository
	inventoryRows       []CustomerInventorySummary
	inventoryCustomerID int64
	batchQuery          CustomerInventoryBatchQuery
}

func (r *miniDirectShipApplicationRepositoryStub) MiniDirectShipCatalog(context.Context, MiniDirectShipCatalogQuery) (MiniDirectShipCatalog, error) {
	return MiniDirectShipCatalog{}, nil
}

func (r *miniDirectShipApplicationRepositoryStub) PreviewMiniDirectShip(context.Context, MiniDirectShipCommand) (MiniDirectShipPreview, error) {
	return MiniDirectShipPreview{}, nil
}

func (r *miniDirectShipApplicationRepositoryStub) SubmitMiniDirectShip(context.Context, MiniDirectShipCommand) (MiniDirectShipRequest, error) {
	return MiniDirectShipRequest{}, nil
}

func (r *miniDirectShipApplicationRepositoryStub) ListMiniDirectShipRequests(context.Context, MiniDirectShipListQuery) (MiniDirectShipListResult, error) {
	return MiniDirectShipListResult{}, nil
}

func (r *miniDirectShipApplicationRepositoryStub) GetMiniDirectShipRequest(context.Context, int64, int64) (MiniDirectShipRequest, error) {
	return MiniDirectShipRequest{}, nil
}

func (r *miniDirectShipApplicationRepositoryStub) CancelMiniDirectShipRequest(context.Context, int64, int64, string) (MiniDirectShipRequest, error) {
	return MiniDirectShipRequest{}, nil
}

func (r *miniDirectShipApplicationRepositoryStub) ListCustomerCentralInventory(_ context.Context, customerID int64) ([]CustomerInventorySummary, error) {
	r.inventoryCustomerID = customerID
	return append([]CustomerInventorySummary(nil), r.inventoryRows...), nil
}

func (r *miniDirectShipApplicationRepositoryStub) ListCustomerCentralInventoryBatches(_ context.Context, query CustomerInventoryBatchQuery) ([]CustomerInventoryBatch, error) {
	r.batchQuery = query
	return []CustomerInventoryBatch{{ProductID: query.ProductID, BomSpecID: query.BomSpecID, BomVariantID: query.BomVariantID, SpecG: query.SpecG}}, nil
}
