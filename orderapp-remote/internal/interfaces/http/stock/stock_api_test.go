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
	outboundQuery    stockapp.OutboundLogQuery
	inventoryQuery   stockapp.WarehouseInventoryQuery
	warehouseBind    stockapp.BindWarehouseCustomerCommand
}

func (f *fakeStockRepo) ListLedger(ctx context.Context, query stockapp.LedgerQuery) (stockapp.LedgerResult, error) {
	return stockapp.LedgerResult{Rows: []stockapp.LedgerRow{{ID: 1, ItemType: "material", ItemName: "水洗豆", QtyChangeG: 1200}}}, nil
}
func (f *fakeStockRepo) ListBatches(ctx context.Context, query stockapp.BatchQuery) (stockapp.BatchResult, error) {
	return stockapp.BatchResult{Rows: []stockapp.BatchRow{{ID: 2, BatchCode: "MB-0000000002", ItemType: "material", ItemName: "水洗豆", QualityStatus: "pass"}}}, nil
}
func (f *fakeStockRepo) ListMaterialBatches(ctx context.Context, query stockapp.MaterialBatchQuery) (stockapp.MaterialBatchResult, error) {
	return stockapp.MaterialBatchResult{Rows: []stockapp.MaterialBatchRow{{ID: 2, BatchCode: "MB-0000000002", RemainingG: 1200, QualityStatus: "hold"}}}, nil
}
func (f *fakeStockRepo) ListWarehouses(ctx context.Context, query stockapp.WarehouseListQuery) ([]stockapp.WarehouseRow, error) {
	return []stockapp.WarehouseRow{{Code: "raw_materials", Name: "原料仓"}, {Code: "wip", Name: "WIP在制仓", CustomerID: query.CustomerID}}, nil
}
func (f *fakeStockRepo) ListMaterialBatchLocations(ctx context.Context, query stockapp.MaterialBatchLocationQuery) (stockapp.MaterialBatchLocationResult, error) {
	return stockapp.MaterialBatchLocationResult{Rows: []stockapp.MaterialBatchLocationRow{{BatchCode: "MB-0000000002", Warehouse: "wip", QtyG: 60000, QualityStatus: "pass"}}}, nil
}
func (f *fakeStockRepo) ListWarehouseInventory(ctx context.Context, query stockapp.WarehouseInventoryQuery) (stockapp.WarehouseInventoryResult, error) {
	f.inventoryQuery = query
	return stockapp.WarehouseInventoryResult{Rows: []stockapp.WarehouseInventoryRow{
		{Warehouse: "raw_materials", WarehouseName: "原料仓", ItemType: "material", ItemID: 1, ItemName: "水洗豆", BatchCode: "MB-0000000002", QtyG: 1200, QualityStatus: "pass"},
		{Warehouse: "finished_goods", WarehouseName: "成品仓", ItemType: "finished_product", ItemID: 9, ItemName: "橘皮乌龙", SpecG: 454, BatchCode: "FP-0000000042", QtyG: 908, QtyUnits: 2, QualityStatus: "reject"},
		{Warehouse: "finished_goods", WarehouseName: "成品仓", ItemType: "finished_product", ItemID: 10, ItemName: "规格商品", BomSpecID: 91, BomVariantID: 191, BomSpecName: "227g袋", InventoryUnit: "袋", QtyUnits: 12, QualityStatus: "pass"},
	}}, nil
}
func (f *fakeStockRepo) ListMaterialBalances(ctx context.Context, query stockapp.MaterialBalanceQuery) ([]stockapp.MaterialBalanceRow, error) {
	return []stockapp.MaterialBalanceRow{{
		MaterialID: 1, Warehouse: query.Warehouse, UnitCode: "kg",
		BookG: 12000, AvailableG: 10000, FrozenG: 2000,
	}}, nil
}
func (f *fakeStockRepo) ListOutboundLogs(ctx context.Context, query stockapp.OutboundLogQuery) (stockapp.OutboundLogResult, error) {
	f.outboundQuery = query
	return stockapp.OutboundLogResult{Rows: []stockapp.OutboundLogRow{{
		DocumentID:      11,
		OrderID:         22,
		OrderNo:         "SO-20260503-0001",
		CustomerName:    "上海门店",
		PostingDate:     "2026-05-03",
		SourceWarehouse: "finished_goods",
		WarehouseName:   "成品仓",
		DeliveryMethod:  "顺丰发货",
		TrackingNo:      "SF123456789",
		VersionNo:       2,
		IsLatest:        true,
		CreatedAt:       "2026-05-03 10:00",
		CreatedBy:       "stock",
		DownloadURL:     "/orders/22/delivery-notes/11.pdf",
		LatestURL:       "/orders/22/delivery-note-latest.pdf",
	}}, Total: 31}, nil
}
func (f *fakeStockRepo) GetStockTrace(ctx context.Context, query stockapp.StockTraceQuery) (stockapp.StockTraceResult, error) {
	f.traceQuery = query
	if query.BatchCode == "LEGACY-MAT-0000000001" {
		return stockapp.StockTraceResult{
			TraceType:     "material_batch",
			MaterialBatch: stockapp.TraceMaterialBatch{BatchCode: "LEGACY-MAT-0000000001", MaterialID: 1, MaterialName: "孟连水洗5T批次", QtyG: 60000, RemainingG: 60000, QualityStatus: "hold", Note: "系统升级按物料现有库存生成的期初批次"},
			MaterialLocations: []stockapp.MaterialBatchLocationRow{{
				BatchCode:     "LEGACY-MAT-0000000001",
				MaterialID:    1,
				MaterialName:  "孟连水洗5T批次",
				Warehouse:     "wip",
				WarehouseName: "WIP在制仓",
				QtyG:          60000,
				QualityStatus: "hold",
			}},
		}, nil
	}
	return stockapp.StockTraceResult{
		TraceType:     "finished_batch",
		FinishedBatch: stockapp.TraceFinishedBatch{BatchCode: "FP-0000000042", ProductName: "橘皮乌龙", Warehouse: "finished_goods", QtyG: 464, QtyUnits: 2, QualityStatus: "reject"},
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
func (f *fakeStockRepo) BindWarehouseCustomer(ctx context.Context, cmd stockapp.BindWarehouseCustomerCommand) (stockapp.WarehouseRow, error) {
	f.warehouseBind = cmd
	return stockapp.WarehouseRow{Code: cmd.WarehouseCode, Name: "门店成品仓", CustomerID: cmd.CustomerID, CustomerName: "渠道客户"}, nil
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
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("采购入库")) {
		t.Fatalf("POST retired receipt status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/material-balances?warehouse=raw_materials&material_ids=1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"book_qty":12`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"available_qty":10`)) ||
		!bytes.Contains(rec.Body.Bytes(), []byte(`"frozen_qty":2`)) {
		t.Fatalf("GET material balances status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/warehouses", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"code":"wip"`)) {
		t.Fatalf("GET warehouses status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/stock/warehouses/finished_shop/customer", bytes.NewBufferString(`{"customer_id":147}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"customer_id":147`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"customer_name":"渠道客户"`)) {
		t.Fatalf("PUT warehouse customer status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.warehouseBind.WarehouseCode != "finished_shop" || repo.warehouseBind.CustomerID != 147 {
		t.Fatalf("warehouse bind command=%+v", repo.warehouseBind)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/material-batch-locations?warehouse=wip", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"warehouse":"wip"`)) {
		t.Fatalf("GET material batch locations status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/warehouse-inventory?warehouse=finished_goods&customer_id=149", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET warehouse inventory status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"warehouse":"finished_goods"`)) || !bytes.Contains(rec.Body.Bytes(), []byte(`"item_type":"finished_product"`)) {
		t.Fatalf("GET warehouse inventory missing finished stock: %s", rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"quality_status":"reject"`)) {
		t.Fatalf("GET warehouse inventory missing quality status: %s", rec.Body.String())
	}
	for _, want := range []string{`"bom_spec_id":91`, `"bom_variant_id":191`, `"bom_spec_name":"227g袋"`, `"inventory_unit":"袋"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("GET warehouse inventory missing canonical BOM specification field %s: %s", want, rec.Body.String())
		}
	}
	if repo.inventoryQuery.CustomerID != 149 {
		t.Fatalf("warehouse inventory customer_id = %d, want 149", repo.inventoryQuery.CustomerID)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/outbound-logs?q=SO-20260503&limit=30", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET outbound logs status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.outboundQuery.Q != "SO-20260503" || repo.outboundQuery.Limit != 30 {
		t.Fatalf("outbound log query = %+v", repo.outboundQuery)
	}
	for _, want := range []string{`"order_no":"SO-20260503-0001"`, `"delivery_method":"顺丰发货"`, `"download_url":"/orders/22/delivery-notes/11.pdf"`, `"latest_url":"/orders/22/delivery-note-latest.pdf"`, `"total":31`, `"total_pages":2`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("GET outbound logs missing %s: %s", want, rec.Body.String())
		}
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
	for _, want := range []string{`"batch_code":"FP-0000000042"`, `"work_order_no":"WO-0000000042"`, `"material_batch_code":"MB-0000000007"`, `"quality_status":"reject"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("trace response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock/trace?batch=LEGACY-MAT-0000000001", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET material trace status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"trace_type":"material_batch"`, `"batch_code":"LEGACY-MAT-0000000001"`, `"warehouse":"wip"`, `"material_name":"孟连水洗5T批次"`, `"quality_status":"hold"`} {
		if !bytes.Contains(rec.Body.Bytes(), []byte(want)) {
			t.Fatalf("material trace response missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestPR605DirectMaterialReceiptIsRejected(t *testing.T) {
	repo := &fakeStockRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(repo)})
	req := httptest.NewRequest(http.MethodPost, "/api/stock/material-receipts", bytes.NewBufferString(`{"material_id":1,"qty_g":1200,"unit_cost":42.5}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !bytes.Contains(rec.Body.Bytes(), []byte("采购入库")) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.received.MaterialID != 0 {
		t.Fatalf("direct material receipt reached stock service: %+v", repo.received)
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

func TestStockAdjustmentsAPIRecordsMaterialCostAdjustment(t *testing.T) {
	repo := &fakeStockRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/stock/adjustments", bytes.NewBufferString(`{"adjustment_type":"material_cost","item_type":"material","item_id":1,"material_batch_id":7,"target_unit_cost":52.75,"reason":"入库单价录错更正"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST material cost adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.adjustment.AdjustmentType != "material_cost" || repo.adjustment.ItemType != "material" || repo.adjustment.ItemID != 1 || repo.adjustment.MaterialBatchID != 7 || repo.adjustment.TargetUnitCost != 52.75 {
		t.Fatalf("cost adjustment command = %+v", repo.adjustment)
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

func TestStockFinishedWritesKeepBOMSpecIdentityWithoutLegacySpecG(t *testing.T) {
	repo := &fakeStockRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/stock/finished-transfers", bytes.NewBufferString(`{"product_id":9,"bom_spec_id":91,"bom_variant_id":191,"from_warehouse":"finished_goods","to_warehouse":"finished_shop","qty_units":2,"note":"BOM规格转仓"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST BOM spec transfer status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.finishedTransfer.ProductID != 9 || repo.finishedTransfer.BomSpecID != 91 || repo.finishedTransfer.BomVariantID != 191 || repo.finishedTransfer.SpecG != 0 || repo.finishedTransfer.QtyUnits != 2 {
		t.Fatalf("BOM spec transfer command=%+v", repo.finishedTransfer)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/stock/adjustments", bytes.NewBufferString(`{"item_type":"finished_product","item_id":9,"bom_spec_id":91,"bom_variant_id":191,"warehouse":"finished_shop","target_units":7,"reason":"BOM规格盘点"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST BOM spec adjustment status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.adjustment.ItemID != 9 || repo.adjustment.BomSpecID != 91 || repo.adjustment.BomVariantID != 191 || repo.adjustment.SpecG != 0 || repo.adjustment.TargetUnits != 7 {
		t.Fatalf("BOM spec adjustment command=%+v", repo.adjustment)
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
