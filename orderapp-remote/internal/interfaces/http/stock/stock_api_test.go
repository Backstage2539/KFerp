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
	received         stockapp.MaterialReceiptCommand
	adjustment       stockapp.StockAdjustmentCommand
	transfer         stockapp.MaterialTransferCommand
	finishedTransfer stockapp.FinishedProductTransferCommand
	traceQuery       stockapp.StockTraceQuery
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
func (f *fakeStockRepo) ListWarehouses(ctx context.Context) ([]stockapp.WarehouseRow, error) {
	return []stockapp.WarehouseRow{{Code: "raw_materials", Name: "原料仓"}, {Code: "wip", Name: "WIP在制仓"}}, nil
}
func (f *fakeStockRepo) ListMaterialBatchLocations(ctx context.Context, query stockapp.MaterialBatchLocationQuery) (stockapp.MaterialBatchLocationResult, error) {
	return stockapp.MaterialBatchLocationResult{Rows: []stockapp.MaterialBatchLocationRow{{BatchCode: "MB-0000000002", Warehouse: "wip", QtyG: 60000}}}, nil
}
func (f *fakeStockRepo) ListWarehouseInventory(ctx context.Context, query stockapp.WarehouseInventoryQuery) (stockapp.WarehouseInventoryResult, error) {
	return stockapp.WarehouseInventoryResult{Rows: []stockapp.WarehouseInventoryRow{
		{Warehouse: "raw_materials", WarehouseName: "原料仓", ItemType: "material", ItemID: 1, ItemName: "水洗豆", BatchCode: "MB-0000000002", QtyG: 1200},
		{Warehouse: "finished_goods", WarehouseName: "成品仓", ItemType: "finished_product", ItemID: 9, ItemName: "橘皮乌龙", SpecG: 454, QtyG: 908, QtyUnits: 2},
	}}, nil
}
func (f *fakeStockRepo) GetStockTrace(ctx context.Context, query stockapp.StockTraceQuery) (stockapp.StockTraceResult, error) {
	f.traceQuery = query
	return stockapp.StockTraceResult{
		FinishedBatch: stockapp.TraceFinishedBatch{BatchCode: "FP-0000000042", ProductName: "橘皮乌龙", Warehouse: "finished_goods", QtyG: 464, QtyUnits: 2},
		Production:    stockapp.TraceProduction{RunningItemID: 42, WorkOrderNo: "WO-0000000042", BatchID: "PLAN-BATCH-001"},
		Materials:     []stockapp.TraceMaterial{{MaterialName: "卡蒂姆水洗", MaterialBatchCode: "MB-0000000007", DeductG: 600}},
	}, nil
}
func (f *fakeStockRepo) ReceiveMaterial(ctx context.Context, cmd stockapp.MaterialReceiptCommand) (stockapp.MaterialReceiptResult, error) {
	f.received = cmd
	return stockapp.MaterialReceiptResult{ReceiptID: 3, BatchID: 4, BatchCode: "MB-0000000003"}, nil
}
func (f *fakeStockRepo) CreateAdjustment(ctx context.Context, cmd stockapp.StockAdjustmentCommand) (stockapp.StockAdjustmentResult, error) {
	f.adjustment = cmd
	return stockapp.StockAdjustmentResult{AdjustmentID: 5}, nil
}
func (f *fakeStockRepo) TransferMaterial(ctx context.Context, cmd stockapp.MaterialTransferCommand) (stockapp.MaterialTransferResult, error) {
	f.transfer = cmd
	return stockapp.MaterialTransferResult{TransferID: 6, TransferNo: "MT-0000000006"}, nil
}
func (f *fakeStockRepo) TransferFinishedProduct(ctx context.Context, cmd stockapp.FinishedProductTransferCommand) (stockapp.FinishedProductTransferResult, error) {
	f.finishedTransfer = cmd
	return stockapp.FinishedProductTransferResult{TransferID: 7, TransferNo: "FT-0000000007"}, nil
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

	req = httptest.NewRequest(http.MethodGet, "/api/stock/warehouses", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"wip"`)) {
		t.Fatalf("GET warehouses status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/material-batch-locations?warehouse=wip", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"warehouse":"wip"`)) {
		t.Fatalf("GET material batch locations status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/warehouse-inventory?warehouse=finished_goods", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET warehouse inventory status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"warehouse":"finished_goods"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"item_type":"finished_product"`)) {
		t.Fatalf("GET warehouse inventory missing finished stock: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/stock/material-transfers", bytes.NewBufferString(`{"material_id":1,"from_warehouse":"raw_materials","to_warehouse":"wip","qty_g":60000,"note":"三天生产领料"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST material transfer status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.transfer.MaterialID != 1 || repo.transfer.FromWarehouse != "raw_materials" || repo.transfer.ToWarehouse != "wip" || repo.transfer.QtyG != 60000 {
		t.Fatalf("transfer command = %+v", repo.transfer)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/stock/finished-transfers", bytes.NewBufferString(`{"product_id":9,"spec_g":454,"from_warehouse":"finished_goods","to_warehouse":"finished_shop","qty_units":1,"qty_loose_g":20,"note":"门店备货"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST finished transfer status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.finishedTransfer.ProductID != 9 || repo.finishedTransfer.SpecG != 454 || repo.finishedTransfer.FromWarehouse != "finished_goods" || repo.finishedTransfer.ToWarehouse != "finished_shop" || repo.finishedTransfer.QtyUnits != 1 || repo.finishedTransfer.QtyLooseG != 20 {
		t.Fatalf("finished transfer command = %+v", repo.finishedTransfer)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/trace?batch=FP-0000000042", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET trace status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.traceQuery.BatchCode != "FP-0000000042" {
		t.Fatalf("trace query = %+v", repo.traceQuery)
	}
	for _, want := range []string{`"batch_code":"FP-0000000042"`, `"work_order_no":"WO-0000000042"`, `"material_batch_code":"MB-0000000007"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("trace response missing %s: %s", want, rec.Body.String())
		}
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

func TestStockAdjustmentsAPIRecordsFinishedWarehouse(t *testing.T) {
	repo := &fakeStockRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/stock/adjustments", bytes.NewBufferString(`{"item_type":"finished_product","item_id":9,"spec_g":454,"warehouse":"finished_shop","target_g":20,"target_units":3,"reason":"门店盘点"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST finished adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.adjustment.ItemType != "finished_product" || repo.adjustment.ItemID != 9 || repo.adjustment.SpecG != 454 || repo.adjustment.Warehouse != "finished_shop" || repo.adjustment.TargetUnits != 3 || repo.adjustment.TargetG != 20 {
		t.Fatalf("finished adjustment command = %+v", repo.adjustment)
	}
}

func TestStockAdjustmentsAPIAcceptsProductAlias(t *testing.T) {
	repo := &fakeStockRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/stock/adjustments", bytes.NewBufferString(`{"item_type":"product","item_id":9,"spec_g":454,"target_units":3,"reason":"门店盘点"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST product alias adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.adjustment.ItemType != "finished_product" || repo.adjustment.Warehouse != "finished_goods" {
		t.Fatalf("adjustment command = %+v, want finished_product in finished_goods", repo.adjustment)
	}
}
