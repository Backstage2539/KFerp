package stock

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	stockapp "orderapp/internal/application/stock"

	"github.com/labstack/echo/v4"
)

type fakeStockRepo struct {
	received   stockapp.MaterialReceiptCommand
	adjustment stockapp.StockAdjustmentCommand
}

func (f *fakeStockRepo) ListLedger(ctx context.Context, query stockapp.LedgerQuery) (stockapp.LedgerResult, error) {
	return stockapp.LedgerResult{Rows: []stockapp.LedgerRow{{ID: 1, ItemType: "material", ItemName: "水洗豆", QtyChangeG: 1200}}}, nil
}
func (f *fakeStockRepo) ListBatches(ctx context.Context, query stockapp.BatchQuery) (stockapp.BatchResult, error) {
	return stockapp.BatchResult{Rows: []stockapp.BatchRow{{ID: 2, BatchCode: "MB-0000000002", ItemType: "material", ItemName: "水洗豆"}}}, nil
}
func (f *fakeStockRepo) ListMaterialBatches(ctx context.Context, query stockapp.MaterialBatchQuery) (stockapp.MaterialBatchResult, error) {
	return stockapp.MaterialBatchResult{Rows: []stockapp.MaterialBatchRow{{ID: 2, BatchCode: "MB-0000000002", RemainingG: 1200}}}, nil
}
func (f *fakeStockRepo) ReceiveMaterial(ctx context.Context, cmd stockapp.MaterialReceiptCommand) (stockapp.MaterialReceiptResult, error) {
	f.received = cmd
	return stockapp.MaterialReceiptResult{ReceiptID: 3, BatchID: 4, BatchCode: "MB-0000000003"}, nil
}
func (f *fakeStockRepo) CreateAdjustment(ctx context.Context, cmd stockapp.StockAdjustmentCommand) (stockapp.StockAdjustmentResult, error) {
	f.adjustment = cmd
	return stockapp.StockAdjustmentResult{AdjustmentID: 5}, nil
}

func TestStockAPIRoutes(t *testing.T) {
	repo := &fakeStockRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/stock/ledger?q=豆", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET ledger status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"item_name":"水洗豆"`)) {
		t.Fatalf("ledger body missing row: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/stock/material-receipts", bytes.NewBufferString(`{"material_id":1,"qty_g":1200,"unit_cost":42.5}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST receipt status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.received.MaterialID != 1 || repo.received.QtyG != 1200 {
		t.Fatalf("received command = %+v", repo.received)
	}
}

func TestStockAdjustmentsAPIRequiresReasonAndRecordsMaterialTarget(t *testing.T) {
	repo := &fakeStockRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/stock/adjustments", bytes.NewBufferString(`{"item_type":"material","item_id":1,"target_g":1200}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("reason required")) {
		t.Fatalf("POST adjustment without reason status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/stock/adjustments", bytes.NewBufferString(`{"item_type":"material","item_id":1,"target_g":1200,"target_units":3,"reason":"库存补录说明"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.adjustment.ItemType != "material" || repo.adjustment.ItemID != 1 || repo.adjustment.TargetG != 1200 || repo.adjustment.TargetUnits != 3 || repo.adjustment.Reason != "库存补录说明" {
		t.Fatalf("adjustment command = %+v", repo.adjustment)
	}
}
