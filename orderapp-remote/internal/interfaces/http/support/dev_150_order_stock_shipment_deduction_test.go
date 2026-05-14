package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestOrderStockShipmentDeductionRequirementEvidenceExists(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"库存待发货订单在回填快递单号时正式扣成品库存",
		"codex/order-stock-shipment-deduction-20260503",
		"DEV-150-01",
		"order_stock_deductions; deductOrderAllocatedStockTx; sales_order_shipment ledger",
		"DEV-150-02",
		"fetchUnproducedNeeds ship_status filter; reserved allocations excluding deductions",
		"TestProducePlanExcludesShippedOrdersWithBlankProcessStatus",
		"TestOrdersShippingTrackingAPIDeductsReservedLegacyFinishedInventoryOnce",
		"TestOrdersSingleShippingTrackingAPIDeductsReservedFinishedBatch",
		"API-150-01",
		"POST /api/orders/shipping-tracking; POST /api/orders/:id/shipping-tracking; GET /api/produce/unproduced",
		"使用库存的订单发货后库存减少且流水可查",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("order stock shipment deduction requirement evidence missing %q", want)
		}
	}
}

func TestOrderStockShipmentDeductionAPICoverageExists(t *testing.T) {
	orderAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "sales", "order_api_test.go")))
	for _, want := range []string{
		"TestOrdersShippingTrackingAPIDeductsReservedLegacyFinishedInventoryOnce",
		"SF-LEGACY-001",
		"source_doc_type='sales_order_shipment'",
		"duplicate shipment update should not double deduct",
		"TestOrdersSingleShippingTrackingAPIDeductsReservedFinishedBatch",
		"POST /api/orders/31/shipping-tracking",
		"TestOrdersShippingTrackingAPIDeductsOrderSourceWarehouseWithoutAllocation",
		"SOURCE-WH:cust_147_processing",
	} {
		if !strings.Contains(orderAPITest, want) {
			t.Fatalf("order_api_test.go missing shipment stock deduction coverage %q", want)
		}
	}

	produceAPITest := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "production", "produce_plan_api_test.go")))
	for _, want := range []string{
		"TestProducePlanExcludesShippedOrdersWithBlankProcessStatus",
		"SO-SHIPPED-BLANK",
		"SO-DECLINE-STOCK",
		"已发货",
	} {
		if !strings.Contains(produceAPITest, want) {
			t.Fatalf("produce_plan_api_test.go missing shipped-order production-plan guard %q", want)
		}
	}
}

func TestOrderStockShipmentDeductionRepositoryWiringExists(t *testing.T) {
	shipment := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "shipment.go")))
	for _, want := range []string{
		"func (r Repository) FillShipmentTracking(",
		"func (r Repository) FillShipmentTrackingByOrderNo(",
		"func (r Repository) FillOrderTracking(",
	} {
		if !strings.Contains(shipment, want) {
			t.Fatalf("shipment.go missing shipping tracking entrypoint %q", want)
		}
	}
	if got := strings.Count(shipment, "deductOrderAllocatedStockTx(ctx, tx,"); got < 3 {
		t.Fatalf("shipment.go calls deductOrderAllocatedStockTx %d times, want at least 3 shipping tracking paths", got)
	}

	deductions := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "order_stock_deductions.go")))
	for _, want := range []string{
		"SELECT EXISTS(SELECT 1 FROM %s.order_stock_deductions WHERE order_id=$1)",
		"if alreadyDeducted",
		"deductFinishedBatchAllocationTx",
		"deductLegacyFinishedInventoryAllocationTx",
		"deductOrderSourceWarehouseItemsTx",
		"salesOrderShipmentStockSource",
		"INSERT INTO %s.stock_ledger_entries",
		"INSERT INTO %s.order_stock_deductions",
		"ON CONFLICT(order_id,product_id,spec_g,batch_code) DO NOTHING",
	} {
		if !strings.Contains(deductions, want) {
			t.Fatalf("order_stock_deductions.go missing deduction/idempotency marker %q", want)
		}
	}
}

func TestOrderStockShipmentDeductionManualEvidenceExists(t *testing.T) {
	orderManual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_ORDER_SALES.md")))
	for _, want := range []string{
		"库存待发货订单生成 Excel 时不扣库存",
		"回填快递单号并标记已发货时，才扣减",
		"`FP-...` 批次或 `LEGACY-FP-...` 库存余额",
		"并写库存流水",
	} {
		if !strings.Contains(orderManual, want) {
			t.Fatalf("OP_MANUAL_ORDER_SALES.md missing shipment stock deduction manual marker %q", want)
		}
	}

	productionManual := string(readOrderAppFileForTest(t, filepath.Join("..", "OP_MANUAL_PRODUCTION.md")))
	for _, want := range []string{
		"选择使用库存后订单进入库存待发货",
		"生产计划中的订单只按库存缺口进入工单",
		"库存待发货订单可在订单列表直接发货",
	} {
		if !strings.Contains(productionManual, want) {
			t.Fatalf("OP_MANUAL_PRODUCTION.md missing stock-ready production manual marker %q", want)
		}
	}
}

func TestOrderStockShipmentDeductionUIEvidenceExists(t *testing.T) {
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))
	for _, want := range []string{
		"ORDER_STOCK_SHIPMENT_DEDUCTION_UI_CLICK_OK",
		"app=http://127.0.0.1:18142",
		"pg=55612",
		"drawer_tracking_click_twice",
		"inventory_2_units_deductions_1_ledger_1_tracking_1",
		"SO-PR150-STOCK-UI",
		"SF-PR150-UI-001",
		"已发货",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance missing order stock shipment deduction UI evidence %q", want)
		}
	}
}
