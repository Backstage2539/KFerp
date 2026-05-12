package customerfulfillment

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgrescore "orderapp/internal/infrastructure/postgres/core"
	postgrescustomerportal "orderapp/internal/infrastructure/postgres/customerportal"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreParsedImportPersistsRowsAndDeduplicatesByCustomerTypeSHA(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('誉观山') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	cmd := app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "誉观山生产工单&物料库存.xlsx",
		SourceSHA256:   strings.Repeat("a", 64),
		CreatedBy:      "Codex",
		Parsed: app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{
					SheetName:   "生豆入库表",
					RowNo:       2,
					RowType:     "raw_bean_receipt",
					ExternalKey: "raw_bean_receipt:IN-001:埃塞花魁",
					Payload: map[string]any{
						"raw_bean_name": "埃塞花魁",
						"quantity_g":    int64(1500),
					},
				},
				{
					SheetName:   "生产工单",
					RowNo:       3,
					RowType:     "processing_work_order",
					ExternalKey: "processing_work_order:WO-001",
					Payload: map[string]any{
						"work_order_no": "WO-001",
						"product_name":  "誉观山花魁227g",
					},
					Error: "投豆量无效",
				},
			},
			Summary: app.ImportSummary{TotalRows: 2, ValidRows: 1, InvalidRows: 1, RawBeanReceipts: 1, ProcessingOrders: 1},
		},
	}

	first, err := repo.StoreParsedImport(ctx, cmd)
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	second, err := repo.StoreParsedImport(ctx, cmd)
	if err != nil {
		t.Fatalf("StoreParsedImport duplicate: %v", err)
	}
	if first.ID <= 0 || second.ID != first.ID {
		t.Fatalf("duplicate batch ID = %d, want same positive ID %d", second.ID, first.ID)
	}

	var batchCount, rowCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.customer_fulfillment_import_batches
		WHERE customer_id=$1 AND import_type=$2 AND source_sha256=$3
	`, schema), customerID, app.ImportTypeProcessingWorkbook, strings.Repeat("a", 64)).Scan(&batchCount); err != nil {
		t.Fatalf("count batches: %v", err)
	}
	if batchCount != 1 {
		t.Fatalf("batch count = %d, want 1", batchCount)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*) FROM %s.customer_fulfillment_import_rows WHERE batch_id=$1
	`, schema), first.ID).Scan(&rowCount); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if rowCount != 2 {
		t.Fatalf("row count = %d, want 2", rowCount)
	}

	var status, beanName, rowError string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT status, payload->>'raw_bean_name', error
		FROM %s.customer_fulfillment_import_rows
		WHERE batch_id=$1 AND external_key='raw_bean_receipt:IN-001:埃塞花魁'
	`, schema), first.ID).Scan(&status, &beanName, &rowError); err != nil {
		t.Fatalf("load persisted payload: %v", err)
	}
	if status != "valid" || beanName != "埃塞花魁" || rowError != "" {
		t.Fatalf("persisted row status/payload/error = %q/%q/%q", status, beanName, rowError)
	}

	var invalidStatus, invalidError string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT status, error
		FROM %s.customer_fulfillment_import_rows
		WHERE batch_id=$1 AND external_key='processing_work_order:WO-001'
	`, schema), first.ID).Scan(&invalidStatus, &invalidError); err != nil {
		t.Fatalf("load invalid row: %v", err)
	}
	if invalidStatus != "invalid" || invalidError != "投豆量无效" {
		t.Fatalf("invalid row status/error = %q/%q", invalidStatus, invalidError)
	}
}

func TestApplyProcessingImportRepositoryWiring(t *testing.T) {
	src := string(readCustomerFulfillmentRepoFile(t, "internal/infrastructure/postgres/customerfulfillment/repository.go"))
	for _, want := range []string{
		"applyProcessingImportRow",
		"raw_bean_receipt",
		"raw_bean_issue",
		"raw_bean_balance",
		"customer_sku",
		"packaging_balance",
		"processing_work_order",
		"packaging_job",
		"conversion_job",
		"customer_custody_ledger_entries",
		"customer_processing_work_orders",
		"customer_processing_packaging_jobs",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository.go missing processing apply marker %q", want)
		}
	}
}

func TestImportRowsRepositoryWiring(t *testing.T) {
	src := string(readCustomerFulfillmentRepoFile(t, "internal/infrastructure/postgres/customerfulfillment/repository.go"))
	for _, want := range []string{
		"func (r *Repository) ImportBatch",
		"func (r *Repository) ListImportRows",
		"customer_fulfillment_import_rows",
		"sheet_name, row_no, row_type, external_key, status, error",
		"WHERE batch_id=$1%s",
		"ORDER BY row_no, id",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository.go missing import row lookup marker %q", want)
		}
	}
}

func TestApplyProcessingImportCreatesCustodyAndWorkOrdersIdempotently(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('誉观山') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	batch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "誉观山生产工单&物料库存.xlsx",
		SourceSHA256:   strings.Repeat("b", 64),
		CreatedBy:      "Codex",
		Parsed: app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "SKU", RowNo: 2, RowType: "customer_sku", ExternalKey: "customer_sku:YGS-HK-227", Payload: map[string]any{"sku_code": "YGS-HK-227", "sku_name": "誉观山花魁227g", "spec": "227g", "roast_degree": "浅烘"}},
				{SheetName: "生豆入库表", RowNo: 2, RowType: "raw_bean_receipt", ExternalKey: "raw_bean_receipt:IN-001:埃塞花魁", Payload: map[string]any{"receipt_no": "IN-001", "raw_bean_name": "埃塞花魁", "quantity_g": int64(1500)}},
				{SheetName: "生豆出库表", RowNo: 2, RowType: "raw_bean_issue", ExternalKey: "raw_bean_issue:OUT-001:埃塞花魁", Payload: map[string]any{"issue_no": "OUT-001", "raw_bean_name": "埃塞花魁", "quantity_g": int64(500)}},
				{SheetName: "生豆库存表", RowNo: 2, RowType: "raw_bean_balance", ExternalKey: "raw_bean_balance:埃塞花魁", Payload: map[string]any{"raw_bean_name": "埃塞花魁", "quantity_g": int64(1000)}},
				{SheetName: "生产工单", RowNo: 2, RowType: "processing_work_order", ExternalKey: "processing_work_order:WO-001", Payload: map[string]any{"date": "2026-03-05", "work_order_no": "WO-001", "product_name": "誉观山花魁227g", "raw_bean_name": "埃塞花魁", "input_quantity_g": int64(1000), "planned_output_units": int64(4), "status": "已完成"}},
				{SheetName: "生产子工单-包装", RowNo: 2, RowType: "packaging_job", ExternalKey: "packaging_job:WO-001:誉观山花魁227g:227g袋", Payload: map[string]any{"work_order_no": "WO-001", "product_name": "誉观山花魁227g", "packaging_name": "227g袋", "quantity_units": int64(4)}},
				{SheetName: "耗材库存（预估）", RowNo: 2, RowType: "packaging_balance", ExternalKey: "packaging_balance:227g袋", Payload: map[string]any{"packaging_name": "227g袋", "quantity_units": int64(96)}},
				{SheetName: "库存转换工单", RowNo: 2, RowType: "conversion_job", ExternalKey: "conversion_job:CV-001:誉观山花魁227g:誉观山挂耳", Payload: map[string]any{"job_no": "CV-001", "from_product": "誉观山花魁227g", "to_product": "誉观山挂耳", "quantity_units": int64(2)}},
			},
			Summary: app.ImportSummary{TotalRows: 8, ValidRows: 8, CustomerSKUs: 1, RawBeanReceipts: 1, RawBeanIssues: 1, RawBeanBalances: 1, ProcessingOrders: 1, PackagingJobs: 1, PackagingBalances: 1, ConversionJobs: 1},
		},
	})
	if err != nil {
		t.Fatalf("StoreParsedImport: %v", err)
	}

	first, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "Codex"})
	if err != nil {
		t.Fatalf("ApplyImport first: %v", err)
	}
	second, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "Codex"})
	if err != nil {
		t.Fatalf("ApplyImport second: %v", err)
	}
	if first.AppliedRows != 8 || second.AppliedRows != 0 {
		t.Fatalf("applied rows first/second = %d/%d, want 8/0", first.AppliedRows, second.AppliedRows)
	}

	assertCustomerFulfillmentCount(t, pool, schema, "products", "customer_id=$1 AND name='誉观山花魁227g' AND visibility='customer_only'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_custody_items", "customer_id=$1 AND item_type='raw_bean' AND item_name='埃塞花魁'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_custody_items", "customer_id=$1 AND item_type='packaging' AND item_name='227g袋'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_custody_ledger_entries", "customer_id=$1", customerID, 4)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_processing_work_orders", "customer_id=$1 AND work_order_no='WO-001'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_processing_work_order_inputs", "raw_bean_name='埃塞花魁' AND quantity_g=1000", 0, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_processing_packaging_jobs", "customer_id=$1 AND work_order_no='WO-001'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_inventory_conversion_jobs", "customer_id=$1 AND job_no='CV-001'", customerID, 1)

	var rawBalanceG, packagingBalanceUnits int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT quantity_g FROM %s.customer_custody_balances
		WHERE customer_id=$1 AND item_type='raw_bean' AND item_name='埃塞花魁'
	`, schema), customerID).Scan(&rawBalanceG); err != nil {
		t.Fatalf("raw bean balance: %v", err)
	}
	if rawBalanceG != 1000 {
		t.Fatalf("raw bean balance = %d, want 1000", rawBalanceG)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT quantity_units FROM %s.customer_custody_balances
		WHERE customer_id=$1 AND item_type='packaging' AND item_name='227g袋'
	`, schema), customerID).Scan(&packagingBalanceUnits); err != nil {
		t.Fatalf("packaging balance: %v", err)
	}
	if packagingBalanceUnits != 96 {
		t.Fatalf("packaging balance = %d, want 96", packagingBalanceUnits)
	}
}

func TestApplyDirectShipImportRepositoryWiring(t *testing.T) {
	src := string(readCustomerFulfillmentRepoFile(t, "internal/infrastructure/postgres/customerfulfillment/repository.go"))
	for _, want := range []string{
		"applyDirectShipImportRow",
		"direct_ship_order",
		"direct_ship_item",
		"customer_direct_ship_import_orders",
		"customer_direct_ship_import_order_items",
		"portal_service_code",
		"direct_ship",
		"processing_warehouse_code",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository.go missing direct-ship apply marker %q", want)
		}
	}
}

func TestDirectShipImportLeavesERPProductIDNullWhenNoProductMatches(t *testing.T) {
	src := string(readCustomerFulfillmentRepoFile(t, "internal/infrastructure/postgres/customerfulfillment/repository.go"))
	if strings.Contains(src, "productID, _ := r.findProductForDirectShipTx") {
		t.Fatalf("direct ship import must not discard missing-product lookup errors and insert product_id=0")
	}
	for _, want := range []string{
		"var productID any",
		"errors.Is(productErr, pgx.ErrNoRows)",
		"productID = matchedProductID",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository.go missing nullable direct-ship product marker %q", want)
		}
	}
}

func TestApplyDirectShipImportCreatesOrdersAndSnapshotsIdempotently(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('誉观山') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, processing_warehouse_code)
		VALUES($1,'YGS-CUST-WH')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}

	batch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "誉观山&口加-代发.xlsx",
		SourceSHA256:   strings.Repeat("c", 64),
		CreatedBy:      "Codex",
		Parsed: app.ParsedWorkbook{
			ImportType: app.ImportTypeDirectShipWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS20260304001", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": "1", "order_no": "YGS20260304001", "receiver_address": "张三 13800000000 浙江杭州西湖区", "waybill_no": "SF123", "status": "待发货"}},
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS20260304001:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS20260304001", "receiver_address": "张三 13800000000 浙江杭州西湖区", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(1), "waybill_no": "SF123"}},
				{SheetName: "代发信息", RowNo: 3, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS20260304001:3:誉观山拼配", Payload: map[string]any{"order_no": "YGS20260304001", "receiver_address": "张三 13800000000 浙江杭州西湖区", "product_title": "誉观山拼配", "spec": "227g", "quantity_units": int64(2), "waybill_no": "SF123"}},
			},
			Summary: app.ImportSummary{TotalRows: 3, ValidRows: 3, DirectShipOrders: 1, DirectShipItems: 2},
		},
	})
	if err != nil {
		t.Fatalf("StoreParsedImport: %v", err)
	}
	first, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "Codex"})
	if err != nil {
		t.Fatalf("ApplyImport first: %v", err)
	}
	second, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "Codex"})
	if err != nil {
		t.Fatalf("ApplyImport second: %v", err)
	}
	if first.AppliedRows != 3 || first.DirectShipOrders != 1 || second.AppliedRows != 0 {
		t.Fatalf("direct ship apply first/second = %#v/%#v, want 3 rows, 1 order, then 0", first, second)
	}

	assertCustomerFulfillmentCount(t, pool, schema, "customer_direct_ship_import_orders", "customer_id=$1 AND external_order_no='YGS20260304001'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_direct_ship_import_order_items", "customer_id=$1", customerID, 2)
	assertCustomerFulfillmentCount(t, pool, schema, "orders", "customer_id=$1 AND portal_service_code='direct_ship'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "order_items", "order_id IN (SELECT order_id FROM "+schema+".customer_direct_ship_import_orders WHERE customer_id=$1)", customerID, 2)

	var receiverName, receiverPhone, receiverAddress, trackingNo, warehouse string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT receiver_name, receiver_phone, receiver_address, ship_tracking_no, source_warehouse
		FROM %s.orders
		WHERE customer_id=$1 AND portal_service_code='direct_ship'
	`, schema), customerID).Scan(&receiverName, &receiverPhone, &receiverAddress, &trackingNo, &warehouse); err != nil {
		t.Fatalf("load created order: %v", err)
	}
	if receiverName != "张三" || receiverPhone != "13800000000" || receiverAddress != "浙江杭州西湖区" || trackingNo != "SF123" || warehouse != "YGS-CUST-WH" {
		t.Fatalf("order snapshot = %q/%q/%q/%q/%q", receiverName, receiverPhone, receiverAddress, trackingNo, warehouse)
	}
}

func TestOverviewIncludesCustomerPortalDirectShipOrders(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, shipStatusID, orderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('岩师傅') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id FROM %s.ship_statuses WHERE name='未发货' ORDER BY id LIMIT 1
	`, schema)).Scan(&shipStatusID); err != nil {
		t.Fatalf("load ship status: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.orders(
			order_no, order_date, customer_id, portal_service_code,
			receiver_name, receiver_phone, receiver_address, ship_status_id
		) VALUES ('CP-DS-20260512-0001','2026-05-12',$1,'direct_ship','张三','13800000000','浙江杭州西湖区',$2)
		RETURNING id
	`, schema), customerID, shipStatusID).Scan(&orderID); err != nil {
		t.Fatalf("insert portal direct ship order: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_items(order_id,line_no,item_name,qty,unit)
		VALUES ($1,1,'誉观山冷萃豆',1,'件'), ($1,2,'誉观山花魁',2,'件')
	`, schema), orderID); err != nil {
		t.Fatalf("insert order items: %v", err)
	}

	got, err := repo.Overview(ctx, app.OverviewQuery{CustomerID: customerID})
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if len(got.DirectShipOrders) != 1 {
		t.Fatalf("direct ship orders = %#v, want one customer portal order", got.DirectShipOrders)
	}
	order := got.DirectShipOrders[0]
	if order.OrderNo != "CP-DS-20260512-0001" || order.ItemCount != 2 || order.Status != "未发货" {
		t.Fatalf("direct ship summary = %#v", order)
	}
	if !strings.Contains(order.ReceiverAddress, "张三") || !strings.Contains(order.ReceiverAddress, "浙江杭州西湖区") {
		t.Fatalf("receiver summary = %q", order.ReceiverAddress)
	}
}

func TestApplySettlementImportRepositoryWiring(t *testing.T) {
	src := string(readCustomerFulfillmentRepoFile(t, "internal/infrastructure/postgres/customerfulfillment/repository.go"))
	for _, want := range []string{
		"applySettlementImportRow",
		"customer_fee_items",
		"customer_fulfillment_import",
		"mapSettlementFeeType",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository.go missing settlement apply marker %q", want)
		}
	}
}

func TestApplySettlementImportCreatesFeeItemsIdempotently(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('誉观山') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	batch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "YGS-DJG-20260304.xlsx",
		SourceSHA256:   strings.Repeat("d", 64),
		CreatedBy:      "Codex",
		Parsed: app.ParsedWorkbook{
			ImportType: app.ImportTypeSettlementWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "结算单", RowNo: 3, RowType: "fee_item", ExternalKey: "fee_item:roasting:3:烘焙费", Payload: map[string]any{"fee_type": "roasting", "fee_name": "烘焙费", "amount_cents": int64(8000), "date": "2026-03-04"}},
				{SheetName: "结算单", RowNo: 5, RowType: "fee_item", ExternalKey: "fee_item:direct_ship_service:5:代发费", Payload: map[string]any{"fee_type": "direct_ship_service", "fee_name": "代发费", "amount_cents": int64(900), "date": "2026-03-04"}},
				{SheetName: "结算单", RowNo: 6, RowType: "fee_item", ExternalKey: "fee_item:storage:6:生豆仓储费", Payload: map[string]any{"fee_type": "storage", "fee_name": "生豆仓储费", "amount_cents": int64(5000), "date": "2026-03-04"}},
				{SheetName: "结算单", RowNo: 8, RowType: "fee_item", ExternalKey: "fee_item:shipping:8:物流费", Payload: map[string]any{"fee_type": "shipping", "fee_name": "物流费", "amount_cents": int64(2400), "date": "2026-03-04"}},
			},
			Summary: app.ImportSummary{TotalRows: 4, ValidRows: 4, FeeItems: 4},
		},
	})
	if err != nil {
		t.Fatalf("StoreParsedImport: %v", err)
	}
	first, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "Codex"})
	if err != nil {
		t.Fatalf("ApplyImport first: %v", err)
	}
	second, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "Codex"})
	if err != nil {
		t.Fatalf("ApplyImport second: %v", err)
	}
	if first.AppliedRows != 4 || first.FeeItems != 4 || second.AppliedRows != 0 {
		t.Fatalf("settlement apply first/second = %#v/%#v", first, second)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1", customerID, 4)

	var totalCents int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(ROUND(SUM(amount) * 100),0)::bigint
		FROM %s.customer_fee_items
		WHERE customer_id=$1
	`, schema), customerID).Scan(&totalCents); err != nil {
		t.Fatalf("sum fees: %v", err)
	}
	if totalCents != 16300 {
		t.Fatalf("total fee cents = %d, want 16300", totalCents)
	}
}

func TestCreateSettlementAggregatesUnsettledFees(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('誉观山') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_fee_items(customer_id, source_type, source_id, fee_type, amount, occurred_at, status)
		VALUES
			($1,'manual',1,'processing',80,'2026-03-04','unsettled'),
			($1,'manual',2,'shipping',24,'2026-03-05','unsettled'),
			($1,'manual',3,'shipping',99,'2026-04-01','unsettled')
	`, schema), customerID); err != nil {
		t.Fatalf("insert fees: %v", err)
	}
	got, err := repo.CreateSettlement(ctx, app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: "2026-03-01",
		PeriodTo:   "2026-03-31",
		CreatedBy:  "Codex",
	})
	if err != nil {
		t.Fatalf("CreateSettlement: %v", err)
	}
	if got.BatchID <= 0 || got.FeeItems != 2 || got.TotalAmountCents != 10400 {
		t.Fatalf("settlement result = %#v, want 2 fee rows and 10400 cents", got)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_settlement_batches", "customer_id=$1 AND settlement_no='CS-"+fmt.Sprint(customerID)+"-20260301-20260331'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND settlement_batch_id="+fmt.Sprint(got.BatchID), customerID, 2)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND settlement_batch_id=0", customerID, 1)
}

func TestUpsertCustomerERPBindingGrantsAppliedTemplateRole(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('客户A') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.company_employees(name, active) VALUES('客户A账号', true) RETURNING id`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'public_sku_direct_ship')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}
	got, err := repo.UpsertCustomerERPBinding(ctx, app.UpsertCustomerERPBindingCommand{
		CustomerID: customerID,
		EmployeeID: employeeID,
		Role:       "customer",
		Status:     "active",
		Actor:      "Codex",
	})
	if err != nil {
		t.Fatalf("UpsertCustomerERPBinding: %v", err)
	}
	if got.CustomerID != customerID || got.EmployeeID != employeeID || got.EmployeeName != "客户A账号" {
		t.Fatalf("binding=%+v", got)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "employee_roles", "employee_id=$1 AND role_code='customer_direct_ship_customer'", employeeID, 1)
}

func assertCustomerFulfillmentCount(t *testing.T, pool *pgxpool.Pool, schema, table, where string, customerID int64, want int) {
	t.Helper()
	args := []any{}
	if customerID > 0 {
		args = append(args, customerID)
	}
	var got int
	if err := pool.QueryRow(context.Background(), fmt.Sprintf(`SELECT COUNT(*) FROM %s.%s WHERE %s`, schema, table, where), args...).Scan(&got); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("%s count = %d, want %d", table, got, want)
	}
}

func newCustomerFulfillmentTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required for customer fulfillment postgres tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	schema := fmt.Sprintf("test_customer_fulfillment_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		pool.Close()
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	if err := postgrescore.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("core.EnsureSchema: %v", err)
	}
	if err := postgresauthz.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("authz.EnsureSchema: %v", err)
	}
	if err := postgrescustomerportal.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customerportal.EnsureSchema: %v", err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("customerfulfillment.EnsureSchema: %v", err)
	}
	return pool, schema
}
