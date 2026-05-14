package customerfulfillment

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	app "orderapp/internal/application/customerfulfillment"
	postgresauthz "orderapp/internal/infrastructure/postgres/authz"
	postgrescompany "orderapp/internal/infrastructure/postgres/company"
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
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
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

func TestApplyProcessingImportReimportCustomerSKUExternalKeyUpdatesExistingProduct(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正客户SKU客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsedSKU := func(name, roast string) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "SKU", RowNo: 2, RowType: "customer_sku", ExternalKey: "customer_sku:YGS-HK-227", Payload: map[string]any{"sku_code": "YGS-HK-227", "sku_name": name, "spec": "227g", "roast_degree": roast}},
			},
			Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, CustomerSKUs: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "customer-sku-original.xlsx",
		SourceSHA256:   strings.Repeat("c", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedSKU("誉观山花魁旧名227g", "浅烘"),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "customer-sku-corrected.xlsx",
		SourceSHA256:   strings.Repeat("d", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedSKU("誉观山花魁新名227g", "中烘"),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	assertCustomerFulfillmentCount(t, pool, schema, "products", "customer_id=$1 AND visibility='customer_only' AND active=true", customerID, 1)
	var productName, roastLevel string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT name, roast_level
		FROM %s.products
		WHERE customer_id=$1 AND visibility='customer_only' AND active=true
	`, schema), customerID).Scan(&productName, &roastLevel); err != nil {
		t.Fatalf("query corrected customer product: %v", err)
	}
	if productName != "誉观山花魁新名227g" || roastLevel != "中烘" {
		t.Fatalf("corrected customer product = %q/%q, want new name/roast", productName, roastLevel)
	}

	options, err := repo.CustomerFulfillmentOptions(ctx, customerID)
	if err != nil {
		t.Fatalf("CustomerFulfillmentOptions: %v", err)
	}
	matching := make([]app.CustomerSKUOption, 0)
	for _, option := range options.CustomerSKUs {
		if option.SKUCode == "YGS-HK-227" || option.ProductName == "誉观山花魁旧名227g" || option.ProductName == "誉观山花魁新名227g" {
			matching = append(matching, option)
		}
	}
	if len(matching) != 1 || matching[0].ProductName != "誉观山花魁新名227g" || matching[0].SKUCode != "YGS-HK-227" {
		t.Fatalf("customer SKU options after corrected reimport = %#v, want single corrected SKU option", matching)
	}
}

func TestApplyProcessingImportReimportWorkOrderInputsReflectLatestRawBeanSet(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('拼配投料客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsedInputs := func(rows ...app.ParsedRow) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows:       rows,
			Summary:    app.ImportSummary{TotalRows: len(rows), ValidRows: len(rows), ProcessingOrders: len(rows)},
		}
	}
	workOrderRow := func(rowNo int, bean string, qtyG int64) app.ParsedRow {
		return app.ParsedRow{
			SheetName:   "生产工单",
			RowNo:       rowNo,
			RowType:     "processing_work_order",
			ExternalKey: "processing_work_order:WO-BLEND-001",
			Payload: map[string]any{
				"date":                 "2026-03-05",
				"work_order_no":        "WO-BLEND-001",
				"product_name":         "誉观山拼配227g",
				"raw_bean_name":        bean,
				"input_quantity_g":     qtyG,
				"planned_output_units": int64(4),
				"status":               "待生产",
			},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "processing-work-order-inputs-original.xlsx",
		SourceSHA256:   strings.Repeat("w", 64),
		CreatedBy:      "Codex",
		Parsed: parsedInputs(
			workOrderRow(2, "埃塞花魁", 600),
			workOrderRow(3, "哥伦比亚慧兰", 400),
		),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}

	var inputCount, inputTotalG, headerInputG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(i.quantity_g),0), MAX(w.input_quantity_g)
		FROM %s.customer_processing_work_order_inputs i
		JOIN %s.customer_processing_work_orders w ON w.id=i.work_order_id
		WHERE w.customer_id=$1 AND w.work_order_no='WO-BLEND-001'
	`, schema, schema), customerID).Scan(&inputCount, &inputTotalG, &headerInputG); err != nil {
		t.Fatalf("load original work-order inputs: %v", err)
	}
	if inputCount != 2 || inputTotalG != 1000 || headerInputG != 1000 {
		t.Fatalf("original work-order inputs = count %d total %d header %d, want 2/1000/1000", inputCount, inputTotalG, headerInputG)
	}

	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "processing-work-order-inputs-corrected.xlsx",
		SourceSHA256:   strings.Repeat("x", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedInputs(workOrderRow(2, "埃塞花魁", 700)),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var remainingBean string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(i.quantity_g),0), MAX(w.input_quantity_g), COALESCE(MAX(i.raw_bean_name),'')
		FROM %s.customer_processing_work_order_inputs i
		JOIN %s.customer_processing_work_orders w ON w.id=i.work_order_id
		WHERE w.customer_id=$1 AND w.work_order_no='WO-BLEND-001'
	`, schema, schema), customerID).Scan(&inputCount, &inputTotalG, &headerInputG, &remainingBean); err != nil {
		t.Fatalf("load corrected work-order inputs: %v", err)
	}
	if inputCount != 1 || inputTotalG != 700 || headerInputG != 700 || remainingBean != "埃塞花魁" {
		t.Fatalf("corrected work-order inputs = count %d total %d header %d bean %q, want 1/700/700/埃塞花魁", inputCount, inputTotalG, headerInputG, remainingBean)
	}
}

func TestApplyProcessingImportReimportConversionJobNoUpdatesExistingJob(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('库存转换修正客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsedConversion := func(fromProduct, toProduct string, units int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{
					SheetName:   "库存转换工单",
					RowNo:       2,
					RowType:     "conversion_job",
					ExternalKey: fmt.Sprintf("conversion_job:CV-CORRECT-001:%s:%s", fromProduct, toProduct),
					Payload: map[string]any{
						"job_no":         "CV-CORRECT-001",
						"from_product":   fromProduct,
						"to_product":     toProduct,
						"quantity_units": units,
					},
				},
			},
			Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, ConversionJobs: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "conversion-original.xlsx",
		SourceSHA256:   strings.Repeat("y", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedConversion("誉观山花魁227g", "誉观山挂耳旧品", 2),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "conversion-corrected.xlsx",
		SourceSHA256:   strings.Repeat("z", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedConversion("誉观山花魁454g", "誉观山挂耳新品", 3),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var jobCount, quantityUnits int64
	var fromProduct, toProduct string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(MAX(from_product),''), COALESCE(MAX(to_product),''), COALESCE(MAX(quantity_units),0)
		FROM %s.customer_inventory_conversion_jobs
		WHERE customer_id=$1 AND job_no='CV-CORRECT-001'
	`, schema), customerID).Scan(&jobCount, &fromProduct, &toProduct, &quantityUnits); err != nil {
		t.Fatalf("load corrected conversion job: %v", err)
	}
	if jobCount != 1 || fromProduct != "誉观山花魁454g" || toProduct != "誉观山挂耳新品" || quantityUnits != 3 {
		t.Fatalf("corrected conversion job = count %d %q->%q qty %d, want single corrected job", jobCount, fromProduct, toProduct, quantityUnits)
	}
}

func TestApplyProcessingImportReimportPackagingJobWorkOrderNoUpdatesExistingJob(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('包装子工单修正客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsedPackagingJob := func(productName, packagingName string, units int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{
					SheetName:   "生产子工单-包装",
					RowNo:       2,
					RowType:     "packaging_job",
					ExternalKey: fmt.Sprintf("packaging_job:PK-CORRECT-001:%s:%s", productName, packagingName),
					Payload: map[string]any{
						"work_order_no":  "PK-CORRECT-001",
						"product_name":   productName,
						"packaging_name": packagingName,
						"quantity_units": units,
					},
				},
			},
			Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, PackagingJobs: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "packaging-job-original.xlsx",
		SourceSHA256:   strings.Repeat("1", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedPackagingJob("誉观山挂耳旧品", "旧挂耳袋", 20),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "packaging-job-corrected.xlsx",
		SourceSHA256:   strings.Repeat("2", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedPackagingJob("誉观山挂耳新品", "新挂耳袋", 30),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var jobCount, quantityUnits int64
	var productName, packagingName string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(MAX(product_name),''), COALESCE(MAX(packaging_name),''), COALESCE(MAX(quantity_units),0)
		FROM %s.customer_processing_packaging_jobs
		WHERE customer_id=$1 AND work_order_no='PK-CORRECT-001'
	`, schema), customerID).Scan(&jobCount, &productName, &packagingName, &quantityUnits); err != nil {
		t.Fatalf("load corrected packaging job: %v", err)
	}
	if jobCount != 1 || productName != "誉观山挂耳新品" || packagingName != "新挂耳袋" || quantityUnits != 30 {
		t.Fatalf("corrected packaging job = count %d %q/%q qty %d, want single corrected job", jobCount, productName, packagingName, quantityUnits)
	}
}

func TestApplyProcessingImportReimportSameCustodyMovementDoesNotDoubleBalance(t *testing.T) {
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
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsed := app.ParsedWorkbook{
		ImportType: app.ImportTypeProcessingWorkbook,
		Rows: []app.ParsedRow{
			{SheetName: "生豆入库表", RowNo: 2, RowType: "raw_bean_receipt", ExternalKey: "raw_bean_receipt:IN-REIMPORT:埃塞花魁", Payload: map[string]any{"receipt_no": "IN-REIMPORT", "raw_bean_name": "埃塞花魁", "quantity_g": int64(1500)}},
		},
		Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, RawBeanReceipts: 1},
	}
	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "誉观山生产工单&物料库存.xlsx",
		SourceSHA256:   strings.Repeat("6", 64),
		CreatedBy:      "Codex",
		Parsed:         parsed,
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "誉观山生产工单&物料库存-重传.xlsx",
		SourceSHA256:   strings.Repeat("7", 64),
		CreatedBy:      "Codex",
		Parsed:         parsed,
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport second batch: %v", err)
	}

	assertCustomerFulfillmentCount(t, pool, schema, "customer_custody_ledger_entries", "customer_id=$1 AND source_external_key='raw_bean_receipt:IN-REIMPORT:埃塞花魁'", customerID, 1)
	var rawBalanceG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT quantity_g FROM %s.customer_custody_balances
		WHERE customer_id=$1 AND item_type='raw_bean' AND item_name='埃塞花魁'
	`, schema), customerID).Scan(&rawBalanceG); err != nil {
		t.Fatalf("raw bean balance: %v", err)
	}
	if rawBalanceG != 1500 {
		t.Fatalf("raw bean balance after reimport = %d, want 1500", rawBalanceG)
	}
}

func TestApplyProcessingImportReimportCorrectedCustodyMovementAdjustsBalanceDelta(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正托管流水客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsedWithQuantities := func(receiptG, issueG int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "生豆入库表", RowNo: 2, RowType: "raw_bean_receipt", ExternalKey: "raw_bean_receipt:IN-CORRECT:埃塞花魁", Payload: map[string]any{"receipt_no": "IN-CORRECT", "raw_bean_name": "埃塞花魁", "quantity_g": receiptG}},
				{SheetName: "生豆出库表", RowNo: 2, RowType: "raw_bean_issue", ExternalKey: "raw_bean_issue:OUT-CORRECT:埃塞花魁", Payload: map[string]any{"issue_no": "OUT-CORRECT", "raw_bean_name": "埃塞花魁", "quantity_g": issueG}},
			},
			Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, RawBeanReceipts: 1, RawBeanIssues: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "processing-custody-original.xlsx",
		SourceSHA256:   strings.Repeat("7", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedWithQuantities(2000, 500),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "processing-custody-corrected.xlsx",
		SourceSHA256:   strings.Repeat("8", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedWithQuantities(1500, 300),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var rawBalanceG int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT quantity_g FROM %s.customer_custody_balances
		WHERE customer_id=$1 AND item_type='raw_bean' AND item_name='埃塞花魁'
	`, schema), customerID).Scan(&rawBalanceG); err != nil {
		t.Fatalf("raw bean balance: %v", err)
	}
	if rawBalanceG != 1200 {
		t.Fatalf("raw bean balance after corrected reimport = %d, want 1200", rawBalanceG)
	}

	var receiptDelta, issueDelta int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT qty_g_delta
		FROM %s.customer_custody_ledger_entries
		WHERE customer_id=$1 AND source_external_key='raw_bean_receipt:IN-CORRECT:埃塞花魁'
	`, schema), customerID).Scan(&receiptDelta); err != nil {
		t.Fatalf("receipt ledger delta: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT qty_g_delta
		FROM %s.customer_custody_ledger_entries
		WHERE customer_id=$1 AND source_external_key='raw_bean_issue:OUT-CORRECT:埃塞花魁'
	`, schema), customerID).Scan(&issueDelta); err != nil {
		t.Fatalf("issue ledger delta: %v", err)
	}
	if receiptDelta != 1500 || issueDelta != -300 {
		t.Fatalf("corrected ledger deltas = %d/%d, want 1500/-300", receiptDelta, issueDelta)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_custody_ledger_entries", "customer_id=$1", customerID, 2)
}

func TestApplyProcessingImportReimportCorrectedCustodyMovementRawBeanNameMovesLedgerAndBalance(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正生豆名称客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsedWithBean := func(beanName string, receiptG, issueG int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "生豆入库表", RowNo: 2, RowType: "raw_bean_receipt", ExternalKey: fmt.Sprintf("raw_bean_receipt:IN-BEAN-NAME-CORRECT:%s", beanName), Payload: map[string]any{"receipt_no": "IN-BEAN-NAME-CORRECT", "raw_bean_name": beanName, "quantity_g": receiptG}},
				{SheetName: "生豆出库表", RowNo: 2, RowType: "raw_bean_issue", ExternalKey: fmt.Sprintf("raw_bean_issue:OUT-BEAN-NAME-CORRECT:%s", beanName), Payload: map[string]any{"issue_no": "OUT-BEAN-NAME-CORRECT", "raw_bean_name": beanName, "quantity_g": issueG}},
			},
			Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, RawBeanReceipts: 1, RawBeanIssues: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "processing-custody-bean-original.xlsx",
		SourceSHA256:   strings.Repeat("s", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedWithBean("埃塞花魁", 2000, 500),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "processing-custody-bean-corrected.xlsx",
		SourceSHA256:   strings.Repeat("t", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedWithBean("肯尼亚AA", 1500, 300),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var oldBeanBalance, latestBeanBalance int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN item_name='埃塞花魁' THEN quantity_g ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN item_name='肯尼亚AA' THEN quantity_g ELSE 0 END),0)
		FROM %s.customer_custody_balances
		WHERE customer_id=$1 AND item_type='raw_bean'
	`, schema), customerID).Scan(&oldBeanBalance, &latestBeanBalance); err != nil {
		t.Fatalf("query raw bean balances: %v", err)
	}

	var ledgerCount, ledgerSum int64
	var ledgerItemNames string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(l.qty_g_delta),0),
			COALESCE(STRING_AGG(DISTINCT i.item_name, ',' ORDER BY i.item_name),'')
		FROM %s.customer_custody_ledger_entries l
		JOIN %s.customer_custody_items i ON i.id=l.item_id
		WHERE l.customer_id=$1
		  AND l.source_type IN ('raw_bean_receipt','raw_bean_issue')
		  AND l.source_external_key LIKE '%%BEAN-NAME-CORRECT%%'
	`, schema, schema), customerID).Scan(&ledgerCount, &ledgerSum, &ledgerItemNames); err != nil {
		t.Fatalf("query corrected raw bean ledger: %v", err)
	}
	if oldBeanBalance != 0 || latestBeanBalance != 1200 || ledgerCount != 2 || ledgerSum != 1200 || ledgerItemNames != "肯尼亚AA" {
		t.Fatalf("corrected raw bean name balances/ledger = old %d latest %d count %d sum %d names %q, want old 0 latest 1200 count 2 sum 1200 names 肯尼亚AA", oldBeanBalance, latestBeanBalance, ledgerCount, ledgerSum, ledgerItemNames)
	}
}

func TestApplyProcessingImportReimportCorrectedCustodyBalanceUpdatesLedgerDelta(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正余额客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsedBalance := func(rawBalanceG, packagingUnits int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "生豆入库表", RowNo: 2, RowType: "raw_bean_receipt", ExternalKey: "raw_bean_receipt:IN-BALANCE-CORRECT:埃塞花魁", Payload: map[string]any{"receipt_no": "IN-BALANCE-CORRECT", "raw_bean_name": "埃塞花魁", "quantity_g": int64(1500)}},
				{SheetName: "生豆库存表", RowNo: 3, RowType: "raw_bean_balance", ExternalKey: "raw_bean_balance:埃塞花魁", Payload: map[string]any{"raw_bean_name": "埃塞花魁", "quantity_g": rawBalanceG}},
				{SheetName: "耗材库存（预估）", RowNo: 4, RowType: "packaging_balance", ExternalKey: "packaging_balance:227g袋", Payload: map[string]any{"packaging_name": "227g袋", "quantity_units": packagingUnits}},
			},
			Summary: app.ImportSummary{TotalRows: 3, ValidRows: 3, RawBeanReceipts: 1, RawBeanBalances: 1, PackagingBalances: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "balance-original.xlsx",
		SourceSHA256:   strings.Repeat("q", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedBalance(1000, 100),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "balance-corrected.xlsx",
		SourceSHA256:   strings.Repeat("r", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedBalance(1200, 80),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var rawBalance, rawLedgerSum int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.quantity_g, COALESCE(SUM(l.qty_g_delta),0)
		FROM %s.customer_custody_balances b
		JOIN %s.customer_custody_ledger_entries l ON l.customer_id=b.customer_id AND l.item_id=b.item_id AND l.item_type='raw_bean'
		WHERE b.customer_id=$1 AND b.item_type='raw_bean' AND b.item_name='埃塞花魁'
		GROUP BY b.quantity_g
	`, schema, schema), customerID).Scan(&rawBalance, &rawLedgerSum); err != nil {
		t.Fatalf("query corrected raw balance and ledger: %v", err)
	}
	if rawBalance != 1200 || rawLedgerSum != 1200 {
		t.Fatalf("raw bean corrected balance/ledger sum = %d/%d, want 1200/1200", rawBalance, rawLedgerSum)
	}

	var packagingBalance, packagingLedgerSum int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT b.quantity_units, COALESCE(SUM(l.qty_units_delta),0)
		FROM %s.customer_custody_balances b
		JOIN %s.customer_custody_ledger_entries l ON l.customer_id=b.customer_id AND l.item_id=b.item_id AND l.item_type='packaging'
		WHERE b.customer_id=$1 AND b.item_type='packaging' AND b.item_name='227g袋'
		GROUP BY b.quantity_units
	`, schema, schema), customerID).Scan(&packagingBalance, &packagingLedgerSum); err != nil {
		t.Fatalf("query corrected packaging balance and ledger: %v", err)
	}
	if packagingBalance != 80 || packagingLedgerSum != 80 {
		t.Fatalf("packaging corrected balance/ledger sum = %d/%d, want 80/80", packagingBalance, packagingLedgerSum)
	}
}

func TestApplyProcessingImportReimportCorrectedCustodyBalanceItemNameMovesLedgerAndBalance(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正余额物料名称客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'processing',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert processing capability: %v", err)
	}

	parsedBalance := func(rawBeanName, packagingName string, rawBalanceG, packagingUnits int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeProcessingWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "生豆库存表", RowNo: 3, RowType: "raw_bean_balance", ExternalKey: fmt.Sprintf("raw_bean_balance:%s", rawBeanName), Payload: map[string]any{"raw_bean_name": rawBeanName, "quantity_g": rawBalanceG}},
				{SheetName: "耗材库存（预估）", RowNo: 4, RowType: "packaging_balance", ExternalKey: fmt.Sprintf("packaging_balance:%s", packagingName), Payload: map[string]any{"packaging_name": packagingName, "quantity_units": packagingUnits}},
			},
			Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, RawBeanBalances: 1, PackagingBalances: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "balance-name-original.xlsx",
		SourceSHA256:   strings.Repeat("u", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedBalance("埃塞花魁", "227g袋", 1000, 100),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeProcessingWorkbook,
		SourceFilename: "balance-name-corrected.xlsx",
		SourceSHA256:   strings.Repeat("v", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedBalance("肯尼亚AA", "挂耳袋", 1200, 80),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var oldRawBalance, latestRawBalance, oldPackagingBalance, latestPackagingBalance int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT
			COALESCE(SUM(CASE WHEN item_type='raw_bean' AND item_name='埃塞花魁' THEN quantity_g ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN item_type='raw_bean' AND item_name='肯尼亚AA' THEN quantity_g ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN item_type='packaging' AND item_name='227g袋' THEN quantity_units ELSE 0 END),0),
			COALESCE(SUM(CASE WHEN item_type='packaging' AND item_name='挂耳袋' THEN quantity_units ELSE 0 END),0)
		FROM %s.customer_custody_balances
		WHERE customer_id=$1
	`, schema), customerID).Scan(&oldRawBalance, &latestRawBalance, &oldPackagingBalance, &latestPackagingBalance); err != nil {
		t.Fatalf("query corrected balance item names: %v", err)
	}

	var rawLedgerCount, rawLedgerSum, packagingLedgerCount, packagingLedgerSum int64
	var rawLedgerNames, packagingLedgerNames string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(l.qty_g_delta),0),
			COALESCE(STRING_AGG(DISTINCT i.item_name, ',' ORDER BY i.item_name),'')
		FROM %s.customer_custody_ledger_entries l
		JOIN %s.customer_custody_items i ON i.id=l.item_id
		WHERE l.customer_id=$1 AND l.source_type='raw_bean_balance'
	`, schema, schema), customerID).Scan(&rawLedgerCount, &rawLedgerSum, &rawLedgerNames); err != nil {
		t.Fatalf("query corrected raw balance ledger: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(SUM(l.qty_units_delta),0),
			COALESCE(STRING_AGG(DISTINCT i.item_name, ',' ORDER BY i.item_name),'')
		FROM %s.customer_custody_ledger_entries l
		JOIN %s.customer_custody_items i ON i.id=l.item_id
		WHERE l.customer_id=$1 AND l.source_type='packaging_balance'
	`, schema, schema), customerID).Scan(&packagingLedgerCount, &packagingLedgerSum, &packagingLedgerNames); err != nil {
		t.Fatalf("query corrected packaging balance ledger: %v", err)
	}

	if oldRawBalance != 0 || latestRawBalance != 1200 || rawLedgerCount != 1 || rawLedgerSum != 1200 || rawLedgerNames != "肯尼亚AA" {
		t.Fatalf("corrected raw balance item = old %d latest %d ledger count %d sum %d names %q, want old 0 latest 1200 count 1 sum 1200 names 肯尼亚AA", oldRawBalance, latestRawBalance, rawLedgerCount, rawLedgerSum, rawLedgerNames)
	}
	if oldPackagingBalance != 0 || latestPackagingBalance != 80 || packagingLedgerCount != 1 || packagingLedgerSum != 80 || packagingLedgerNames != "挂耳袋" {
		t.Fatalf("corrected packaging balance item = old %d latest %d ledger count %d sum %d names %q, want old 0 latest 80 count 1 sum 80 names 挂耳袋", oldPackagingBalance, latestPackagingBalance, packagingLedgerCount, packagingLedgerSum, packagingLedgerNames)
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
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
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

func TestApplyDirectShipImportReimportSameExternalOrderDoesNotDuplicateItems(t *testing.T) {
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
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
	}

	parsed := app.ParsedWorkbook{
		ImportType: app.ImportTypeDirectShipWorkbook,
		Rows: []app.ParsedRow{
			{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS20260304002", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": "1", "order_no": "YGS20260304002", "receiver_address": "李四 13900000000 浙江宁波鄞州区", "waybill_no": "SF456", "status": "待发货"}},
			{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS20260304002:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS20260304002", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(1), "waybill_no": "SF456"}},
			{SheetName: "代发信息", RowNo: 3, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS20260304002:3:誉观山拼配", Payload: map[string]any{"order_no": "YGS20260304002", "product_title": "誉观山拼配", "spec": "227g", "quantity_units": int64(2), "waybill_no": "SF456"}},
		},
		Summary: app.ImportSummary{TotalRows: 3, ValidRows: 3, DirectShipOrders: 1, DirectShipItems: 2},
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "誉观山&口加-代发.xlsx",
		SourceSHA256:   strings.Repeat("e", 64),
		CreatedBy:      "Codex",
		Parsed:         parsed,
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "誉观山&口加-代发-重传.xlsx",
		SourceSHA256:   strings.Repeat("f", 64),
		CreatedBy:      "Codex",
		Parsed:         parsed,
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport second batch: %v", err)
	}

	assertCustomerFulfillmentCount(t, pool, schema, "customer_direct_ship_import_orders", "customer_id=$1 AND external_order_no='YGS20260304002'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_direct_ship_import_order_items", "customer_id=$1 AND product_title IN ('誉观山花魁','誉观山拼配')", customerID, 2)
	assertCustomerFulfillmentCount(t, pool, schema, "orders", "customer_id=$1 AND order_no='YGS20260304002'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "order_items", "order_id IN (SELECT order_id FROM "+schema+".customer_direct_ship_import_orders WHERE customer_id=$1 AND external_order_no='YGS20260304002')", customerID, 2)
}

func TestApplyDirectShipImportReimportCorrectedOrderHeaderUpdatesERPOrderSnapshot(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正代发订单头客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
	}

	parsedHeader := func(orderDate, receiver, remark string) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeDirectShipWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS-HEADER-001", Payload: map[string]any{"order_date": orderDate, "sequence_no": "1", "order_no": "YGS-HEADER-001", "receiver_address": receiver, "waybill_no": "SF-HEADER-001", "status": "待发货", "remark": remark}},
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS-HEADER-001:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS-HEADER-001", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(1), "waybill_no": "SF-HEADER-001"}},
			},
			Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, DirectShipOrders: 1, DirectShipItems: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "direct-ship-header-original.xlsx",
		SourceSHA256:   strings.Repeat("s", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedHeader("2026-03-04", "孙一 13200000000 浙江杭州西湖区", "原备注"),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "direct-ship-header-corrected.xlsx",
		SourceSHA256:   strings.Repeat("t", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedHeader("2026-03-06", "周二 13300000000 浙江杭州滨江区", "修正备注"),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var orderDate time.Time
	var receiverName, receiverPhone, receiverAddress, notes string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT order_date, receiver_name, receiver_phone, receiver_address, notes
		FROM %s.orders
		WHERE customer_id=$1 AND order_no='YGS-HEADER-001'
	`, schema), customerID).Scan(&orderDate, &receiverName, &receiverPhone, &receiverAddress, &notes); err != nil {
		t.Fatalf("load corrected ERP order header: %v", err)
	}
	if got := orderDate.Format("2006-01-02"); got != "2026-03-06" {
		t.Fatalf("ERP direct ship order date after corrected reimport = %s, want 2026-03-06", got)
	}
	if receiverName != "周二" || receiverPhone != "13300000000" || receiverAddress != "浙江杭州滨江区" || notes != "修正备注" {
		t.Fatalf("ERP direct ship order header after corrected reimport = %q/%q/%q/%q, want corrected snapshot", receiverName, receiverPhone, receiverAddress, notes)
	}
}

func TestApplyDirectShipImportReimportCorrectedSequenceNoUpdatesExistingOrder(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正代发序号客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
	}

	parsedSequence := func(sequenceNo, receiver string) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeDirectShipWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS-SEQ-001", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": sequenceNo, "order_no": "YGS-SEQ-001", "receiver_address": receiver, "status": "待发货", "remark": "序号修正"}},
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS-SEQ-001:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS-SEQ-001", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(1)}},
			},
			Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, DirectShipOrders: 1, DirectShipItems: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "direct-ship-sequence-original.xlsx",
		SourceSHA256:   strings.Repeat("w", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedSequence("1", "吴一 13100000000 浙江杭州西湖区"),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "direct-ship-sequence-corrected.xlsx",
		SourceSHA256:   strings.Repeat("x", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedSequence("2", "吴二 13100000001 浙江杭州滨江区"),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var importOrderCount, erpOrderCount, orderItemCount int64
	var externalSeq, receiverName, receiverPhone, receiverAddress string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(MAX(external_seq), '')
		FROM %s.customer_direct_ship_import_orders
		WHERE customer_id=$1 AND external_order_no='YGS-SEQ-001'
	`, schema), customerID).Scan(&importOrderCount, &externalSeq); err != nil {
		t.Fatalf("count corrected direct ship import order: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(MAX(receiver_name), ''), COALESCE(MAX(receiver_phone), ''), COALESCE(MAX(receiver_address), '')
		FROM %s.orders
		WHERE customer_id=$1 AND order_no='YGS-SEQ-001'
	`, schema), customerID).Scan(&erpOrderCount, &receiverName, &receiverPhone, &receiverAddress); err != nil {
		t.Fatalf("count corrected ERP order: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s.order_items
		WHERE order_id IN (
			SELECT id FROM %s.orders WHERE customer_id=$1 AND order_no='YGS-SEQ-001'
		)
	`, schema, schema), customerID).Scan(&orderItemCount); err != nil {
		t.Fatalf("count corrected ERP order items: %v", err)
	}
	if importOrderCount != 1 || erpOrderCount != 1 || orderItemCount != 1 || externalSeq != "2" || receiverName != "吴二" || receiverPhone != "13100000001" || receiverAddress != "浙江杭州滨江区" {
		t.Fatalf("corrected direct ship sequence = import orders %d seq %q ERP orders %d items %d receiver %q/%q/%q, want one latest order seq 2", importOrderCount, externalSeq, erpOrderCount, orderItemCount, receiverName, receiverPhone, receiverAddress)
	}
}

func TestApplyDirectShipImportReimportCorrectedStatusUpdatesERPShipStatus(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	for _, statusName := range []string{"待发货", "已发货"} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.ship_statuses(name)
			SELECT $1
			WHERE NOT EXISTS (SELECT 1 FROM %s.ship_statuses WHERE name=$1)
		`, schema, schema), statusName); err != nil {
			t.Fatalf("seed ship status %s: %v", statusName, err)
		}
	}

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正代发发货状态客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
	}

	parsedStatus := func(shipStatus string) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeDirectShipWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS-STATUS-001", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": "1", "order_no": "YGS-STATUS-001", "receiver_address": "郑三 13400000000 浙江杭州余杭区", "waybill_no": "SF-STATUS-001", "status": shipStatus}},
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS-STATUS-001:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS-STATUS-001", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(1), "waybill_no": "SF-STATUS-001"}},
			},
			Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, DirectShipOrders: 1, DirectShipItems: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "direct-ship-status-original.xlsx",
		SourceSHA256:   strings.Repeat("u", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedStatus("待发货"),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "direct-ship-status-corrected.xlsx",
		SourceSHA256:   strings.Repeat("v", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedStatus("已发货"),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var shipStatus string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(ss.name,'')
		FROM %s.orders o
		LEFT JOIN %s.ship_statuses ss ON ss.id=o.ship_status_id
		WHERE o.customer_id=$1 AND o.order_no='YGS-STATUS-001'
	`, schema, schema), customerID).Scan(&shipStatus); err != nil {
		t.Fatalf("load corrected ERP ship status: %v", err)
	}
	if shipStatus != "已发货" {
		t.Fatalf("ERP direct ship status after corrected reimport = %q, want 已发货", shipStatus)
	}
}

func TestApplyDirectShipImportReimportShorterOrderRemovesStaleItems(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正代发客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
	}

	firstParsed := app.ParsedWorkbook{
		ImportType: app.ImportTypeDirectShipWorkbook,
		Rows: []app.ParsedRow{
			{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS-SHORT-001", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": "1", "order_no": "YGS-SHORT-001", "receiver_address": "王五 13700000000 浙江杭州滨江区", "status": "待发货"}},
			{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS-SHORT-001:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS-SHORT-001", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(1)}},
			{SheetName: "代发信息", RowNo: 3, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS-SHORT-001:3:誉观山拼配", Payload: map[string]any{"order_no": "YGS-SHORT-001", "product_title": "誉观山拼配", "spec": "227g", "quantity_units": int64(2)}},
		},
		Summary: app.ImportSummary{TotalRows: 3, ValidRows: 3, DirectShipOrders: 1, DirectShipItems: 2},
	}
	secondParsed := app.ParsedWorkbook{
		ImportType: app.ImportTypeDirectShipWorkbook,
		Rows: []app.ParsedRow{
			{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS-SHORT-001", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": "1", "order_no": "YGS-SHORT-001", "receiver_address": "王五 13700000000 浙江杭州滨江区", "status": "待发货"}},
			{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS-SHORT-001:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS-SHORT-001", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(3)}},
		},
		Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, DirectShipOrders: 1, DirectShipItems: 1},
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "direct-ship-full.xlsx",
		SourceSHA256:   strings.Repeat("1", 64),
		CreatedBy:      "Codex",
		Parsed:         firstParsed,
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "direct-ship-shorter.xlsx",
		SourceSHA256:   strings.Repeat("2", 64),
		CreatedBy:      "Codex",
		Parsed:         secondParsed,
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport shorter batch: %v", err)
	}

	assertCustomerFulfillmentCount(t, pool, schema, "customer_direct_ship_import_order_items", "customer_id=$1 AND product_title IN ('誉观山花魁','誉观山拼配')", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "order_items", "order_id IN (SELECT order_id FROM "+schema+".customer_direct_ship_import_orders WHERE customer_id=$1 AND external_order_no='YGS-SHORT-001')", customerID, 1)

	var qty int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(qty,0)::bigint
		FROM %s.order_items
		WHERE order_id IN (
			SELECT order_id
			FROM %s.customer_direct_ship_import_orders
			WHERE customer_id=$1 AND external_order_no='YGS-SHORT-001'
		)
		AND line_no=1
	`, schema, schema), customerID).Scan(&qty); err != nil {
		t.Fatalf("query retained order item qty: %v", err)
	}
	if qty != 3 {
		t.Fatalf("retained line quantity = %d, want corrected quantity 3", qty)
	}
}

func TestApplyDirectShipImportReimportCorrectedWaybillReplacesImportedTrackings(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正运单客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
	}

	parsedWithWaybill := func(hash, waybill string) app.StoreParsedImportCommand {
		return app.StoreParsedImportCommand{
			CustomerID:     customerID,
			ImportType:     app.ImportTypeDirectShipWorkbook,
			SourceFilename: "direct-ship-waybill-" + waybill + ".xlsx",
			SourceSHA256:   strings.Repeat(hash, 64),
			CreatedBy:      "Codex",
			Parsed: app.ParsedWorkbook{
				ImportType: app.ImportTypeDirectShipWorkbook,
				Rows: []app.ParsedRow{
					{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS-WAYBILL-001", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": "1", "order_no": "YGS-WAYBILL-001", "receiver_address": "赵六 13600000000 浙江杭州上城区", "waybill_no": waybill, "status": "待发货"}},
					{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS-WAYBILL-001:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS-WAYBILL-001", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(1)}},
				},
				Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, DirectShipOrders: 1, DirectShipItems: 1},
			},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, parsedWithWaybill("3", "SF-OLD-001"))
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, parsedWithWaybill("4", "SF-NEW-001"))
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected waybill batch: %v", err)
	}

	var summary string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT ship_tracking_no
		FROM %s.orders
		WHERE customer_id=$1 AND order_no='YGS-WAYBILL-001'
	`, schema), customerID).Scan(&summary); err != nil {
		t.Fatalf("query corrected waybill summary: %v", err)
	}
	if summary != "SF-NEW-001" {
		t.Fatalf("ship tracking summary = %q, want corrected waybill only", summary)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "order_shipping_trackings", "tracking_no='SF-OLD-001'", 0, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "order_shipping_trackings", "tracking_no='SF-NEW-001'", 0, 1)
}

func TestApplyDirectShipImportReimportBlankWaybillClearsImportedTrackings(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('清空运单客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
	}

	parsedWithWaybill := func(hash, waybill string) app.StoreParsedImportCommand {
		return app.StoreParsedImportCommand{
			CustomerID:     customerID,
			ImportType:     app.ImportTypeDirectShipWorkbook,
			SourceFilename: "direct-ship-clear-waybill-" + hash + ".xlsx",
			SourceSHA256:   strings.Repeat(hash, 64),
			CreatedBy:      "Codex",
			Parsed: app.ParsedWorkbook{
				ImportType: app.ImportTypeDirectShipWorkbook,
				Rows: []app.ParsedRow{
					{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:YGS-WAYBILL-CLEAR-001", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": "1", "order_no": "YGS-WAYBILL-CLEAR-001", "receiver_address": "钱七 13500000000 浙江杭州拱墅区", "waybill_no": waybill, "status": "待发货"}},
					{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_item", ExternalKey: "direct_ship_item:YGS-WAYBILL-CLEAR-001:2:誉观山花魁", Payload: map[string]any{"order_no": "YGS-WAYBILL-CLEAR-001", "product_title": "誉观山花魁", "spec": "100g", "quantity_units": int64(1), "waybill_no": waybill}},
				},
				Summary: app.ImportSummary{TotalRows: 2, ValidRows: 2, DirectShipOrders: 1, DirectShipItems: 1},
			},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, parsedWithWaybill("5", "SF-REMOVE-001"))
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, parsedWithWaybill("6", ""))
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport blank waybill batch: %v", err)
	}

	var summary string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT ship_tracking_no
		FROM %s.orders
		WHERE customer_id=$1 AND order_no='YGS-WAYBILL-CLEAR-001'
	`, schema), customerID).Scan(&summary); err != nil {
		t.Fatalf("query cleared waybill summary: %v", err)
	}
	if summary != "" {
		t.Fatalf("ship tracking summary after blank waybill reimport = %q, want empty", summary)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "order_shipping_trackings", "tracking_no='SF-REMOVE-001'", 0, 0)
}

func TestApplyDirectShipImportRejectsCustomerWithoutDirectShipCapability(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type) VALUES('零售商城客户','retail') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	batch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeDirectShipWorkbook,
		SourceFilename: "retail-direct-ship.xlsx",
		SourceSHA256:   strings.Repeat("e", 64),
		CreatedBy:      "Codex",
		Parsed: app.ParsedWorkbook{
			ImportType: app.ImportTypeDirectShipWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "代发信息", RowNo: 2, RowType: "direct_ship_order", ExternalKey: "direct_ship_order:RETAIL001", Payload: map[string]any{"order_date": "2026-03-04", "sequence_no": "1", "order_no": "RETAIL001", "receiver_address": "张三 13800000000 浙江杭州西湖区", "status": "待发货"}},
			},
			Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, DirectShipOrders: 1},
		},
	})
	if err != nil {
		t.Fatalf("StoreParsedImport: %v", err)
	}

	_, err = repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: batch.ID, Actor: "Codex"})
	if err == nil || !strings.Contains(err.Error(), "customer capability direct_ship unavailable") {
		t.Fatalf("ApplyImport err=%v, want direct_ship capability unavailable", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_direct_ship_import_orders", "customer_id=$1", customerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "orders", "customer_id=$1 AND portal_service_code='direct_ship'", customerID, 0)
}

func TestOverviewIncludesCustomerPortalDirectShipOrders(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID, shipStatusID, orderID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type) VALUES('岩师傅','wholesale') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('岩师傅账号','13800138201','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_erp_user_bindings(customer_id, employee_id, status)
		VALUES($1,$2,'active')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert binding: %v", err)
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

func TestSubmitCustomerDirectShipOrderCreatesERPOrder(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type) VALUES('岩师傅','wholesale') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('13800138065','13800138065','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_erp_user_bindings(customer_id, employee_id, status)
		VALUES($1,$2,'active')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert binding: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert direct ship capability: %v", err)
	}

	got, err := repo.SubmitCustomerDirectShipOrder(ctx, app.SubmitCustomerDirectShipOrderCommand{
		EmployeeID:      employeeID,
		ReceiverName:    "刘祎泊",
		ReceiverPhone:   "15302787466",
		ReceiverAddress: "云南省昆明市西山区西坝新村30号C区",
		ProductID:       12,
		ProductName:     "岩师傅冷萃豆",
		Spec:            "100g",
		QuantityUnits:   2,
		Note:            "客户门户代发",
	})
	if err != nil {
		t.Fatalf("SubmitCustomerDirectShipOrder: %v", err)
	}
	if got.OrderNo == "" {
		t.Fatal("OrderNo is empty")
	}

	var orderID, linkedOrderID int64
	var orderNo, portalServiceCode, receiverName, receiverPhone, receiverAddress string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id, order_no, portal_service_code, receiver_name, receiver_phone, receiver_address
		FROM %s.orders
		WHERE customer_id=$1 AND order_no=$2
	`, schema), customerID, got.OrderNo).Scan(&orderID, &orderNo, &portalServiceCode, &receiverName, &receiverPhone, &receiverAddress); err != nil {
		t.Fatalf("load created order: %v", err)
	}
	if orderNo != got.OrderNo || portalServiceCode != "direct_ship" || receiverName != "刘祎泊" || receiverPhone != "15302787466" || receiverAddress != "云南省昆明市西山区西坝新村30号C区" {
		t.Fatalf("created order = %d/%q/%q/%q/%q/%q", orderID, orderNo, portalServiceCode, receiverName, receiverPhone, receiverAddress)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT order_id
		FROM %s.customer_direct_ship_import_orders
		WHERE customer_id=$1 AND external_order_no=$2
	`, schema), customerID, got.OrderNo).Scan(&linkedOrderID); err != nil {
		t.Fatalf("load direct ship link: %v", err)
	}
	if linkedOrderID != orderID {
		t.Fatalf("direct ship linked order_id = %d, want %d", linkedOrderID, orderID)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "order_items", "order_id=$1 AND item_name='岩师傅冷萃豆' AND qty=2", orderID, 1)
}

func TestInternalCustomerFulfillmentSubmitRequiresCustomerCapability(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type) VALUES('未开履约能力客户','retail') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	_, err := repo.SubmitCustomerProcessingWorkOrder(ctx, app.SubmitCustomerProcessingWorkOrderCommand{
		CustomerID:         customerID,
		ProductName:        "越权加工",
		InputQuantityG:     500,
		PlannedOutputUnits: 5,
	})
	if err == nil || !strings.Contains(err.Error(), "customer capability processing unavailable") {
		t.Fatalf("internal processing submit error = %v, want processing capability rejection", err)
	}
	_, err = repo.SubmitCustomerDirectShipOrder(ctx, app.SubmitCustomerDirectShipOrderCommand{
		CustomerID:      customerID,
		ReceiverName:    "张三",
		ReceiverPhone:   "13800000000",
		ReceiverAddress: "浙江杭州",
		ProductName:     "越权代发",
		Spec:            "100g",
		QuantityUnits:   1,
	})
	if err == nil || !strings.Contains(err.Error(), "customer capability direct_ship unavailable") {
		t.Fatalf("internal direct ship submit error = %v, want direct ship capability rejection", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_processing_work_orders", "customer_id=$1", customerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_direct_ship_import_orders", "customer_id=$1", customerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "orders", "customer_id=$1 AND portal_service_code='direct_ship'", customerID, 0)
}

func TestInternalCustomerFulfillmentOverviewRequiresActiveERPBinding(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type, active) VALUES('未绑定履约客户','wholesale',true) RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'processing_fulfillment')
	`, schema), customerID); err != nil {
		t.Fatalf("insert capability profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES
			($1,'processing',true),
			($1,'direct_ship',true),
			($1,'inventory_custody',true),
			($1,'settlement',true);
	`, schema), customerID); err != nil {
		t.Fatalf("insert capabilities: %v", err)
	}

	_, err := repo.Overview(ctx, app.OverviewQuery{CustomerID: customerID})
	if !errors.Is(err, app.ErrCustomerERPBindingNotFound) {
		t.Fatalf("Overview err=%v, want ErrCustomerERPBindingNotFound for unbound fulfillment customer", err)
	}
}

func TestAdjustCustodyInventoryRequiresCustomerInventoryCapability(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type) VALUES('零售商城客户','retail') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}

	_, err := repo.AdjustCustodyInventory(ctx, app.AdjustCustodyInventoryCommand{
		CustomerID:     customerID,
		ItemType:       "raw_bean",
		ItemName:       "越权生豆",
		QuantityGDelta: 1000,
		Note:           "越权补录",
		Actor:          "Codex",
	})
	if err == nil || !strings.Contains(err.Error(), "customer capability inventory_custody unavailable") {
		t.Fatalf("AdjustCustodyInventory err=%v, want inventory custody capability rejection", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_custody_items", "customer_id=$1", customerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_custody_ledger_entries", "customer_id=$1", customerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_custody_balances", "customer_id=$1", customerID, 0)
}

func TestCustomerPortalSubmitRequiresBoundCustomerCapability(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var directCustomerID, processingCustomerID, directEmployeeID, processingEmployeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type) VALUES('公共SKU代发客户','wholesale') RETURNING id
	`, schema)).Scan(&directCustomerID); err != nil {
		t.Fatalf("insert direct customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type) VALUES('代加工客户','wholesale') RETURNING id
	`, schema)).Scan(&processingCustomerID); err != nil {
		t.Fatalf("insert processing customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('公共SKU账号','13800138101','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&directEmployeeID); err != nil {
		t.Fatalf("insert direct employee: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('代加工账号','13800138102','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&processingEmployeeID); err != nil {
		t.Fatalf("insert processing employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, status)
		VALUES($1,$2,'active'),($3,$4,'active')
	`, schema), directCustomerID, directEmployeeID, processingCustomerID, processingEmployeeID); err != nil {
		t.Fatalf("insert bindings: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'direct_ship',true),($2,'processing',true)
	`, schema), directCustomerID, processingCustomerID); err != nil {
		t.Fatalf("insert capabilities: %v", err)
	}

	_, err := repo.SubmitCustomerProcessingWorkOrder(ctx, app.SubmitCustomerProcessingWorkOrderCommand{
		EmployeeID:         directEmployeeID,
		ProductName:        "越权加工",
		InputQuantityG:     500,
		PlannedOutputUnits: 5,
	})
	if err == nil || !strings.Contains(err.Error(), "customer capability processing unavailable") {
		t.Fatalf("processing submit error = %v, want processing capability rejection", err)
	}
	_, err = repo.SubmitCustomerDirectShipOrder(ctx, app.SubmitCustomerDirectShipOrderCommand{
		EmployeeID:      processingEmployeeID,
		ReceiverName:    "张三",
		ReceiverPhone:   "13800000000",
		ReceiverAddress: "浙江杭州",
		ProductName:     "越权代发",
		Spec:            "100g",
		QuantityUnits:   1,
	})
	if err == nil || !strings.Contains(err.Error(), "customer capability direct_ship unavailable") {
		t.Fatalf("direct ship submit error = %v, want direct ship capability rejection", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_processing_work_orders", "customer_id=$1", directCustomerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_direct_ship_import_orders", "customer_id=$1", processingCustomerID, 0)
}

func TestCustomerPortalDirectShipSubmitRepositoryWiresERPOrderCreation(t *testing.T) {
	src := string(readCustomerFulfillmentRepoFile(t, "internal/infrastructure/postgres/customerfulfillment/repository.go"))
	for _, want := range []string{
		"createSubmittedDirectShipERPOrderTx",
		"backfillSubmittedDirectShipERPOrders",
		"repairSubmittedDirectShipERPOrderReceivers",
		"UPDATE %s.customer_direct_ship_import_orders\n\t\tSET order_id=$2",
		"requireCustomerCapability(ctx, customerID, \"processing\")",
		"requireCustomerCapability(ctx, customerID, \"direct_ship\")",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("repository.go missing customer portal direct ship ERP order marker %q", want)
		}
	}
}

func TestSubmittedDirectShipReceiverCorrectsAddressPhoneNameSnapshot(t *testing.T) {
	name, phone, address, company := submittedDirectShipReceiver(map[string]any{
		"receiver_name":  "云南省昆明市西山区西坝新村30号C区",
		"receiver_phone": "15302787466",
	}, "云南省昆明市西山区西坝新村30号C区 15302787466 刘祎泊")
	if name != "刘祎泊" || phone != "15302787466" || address != "云南省昆明市西山区西坝新村30号C区" || company != "" {
		t.Fatalf("receiver = %q/%q/%q/%q", name, phone, address, company)
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
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
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

func TestApplySettlementImportReimportSameFeeDoesNotDuplicateFeeItems(t *testing.T) {
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
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
	}
	parsed := app.ParsedWorkbook{
		ImportType: app.ImportTypeSettlementWorkbook,
		Rows: []app.ParsedRow{
			{SheetName: "结算单", RowNo: 3, RowType: "fee_item", ExternalKey: "fee_item:roasting:3:烘焙费", Payload: map[string]any{"fee_type": "roasting", "fee_name": "烘焙费", "amount_cents": int64(8000), "date": "2026-03-04"}},
		},
		Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, FeeItems: 1},
	}
	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "YGS-DJG-20260304.xlsx",
		SourceSHA256:   strings.Repeat("8", 64),
		CreatedBy:      "Codex",
		Parsed:         parsed,
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "YGS-DJG-20260304-重传.xlsx",
		SourceSHA256:   strings.Repeat("9", 64),
		CreatedBy:      "Codex",
		Parsed:         parsed,
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport second batch: %v", err)
	}

	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND note='烘焙费'", customerID, 1)
	var totalCents int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(ROUND(SUM(amount) * 100),0)::bigint
		FROM %s.customer_fee_items
		WHERE customer_id=$1
	`, schema), customerID).Scan(&totalCents); err != nil {
		t.Fatalf("sum fees: %v", err)
	}
	if totalCents != 8000 {
		t.Fatalf("total fee cents after settlement reimport = %d, want 8000", totalCents)
	}
}

func TestApplySettlementImportReimportCorrectedFeeUpdatesUnsettledFeeItem(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('修正结算客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
	}

	parsedWithAmount := func(amountCents int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeSettlementWorkbook,
			Rows: []app.ParsedRow{
				{SheetName: "结算单", RowNo: 3, RowType: "fee_item", ExternalKey: "fee_item:roasting:3:烘焙费", Payload: map[string]any{"fee_type": "roasting", "fee_name": "烘焙费", "amount_cents": amountCents, "date": "2026-03-04"}},
			},
			Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, FeeItems: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "settlement-original.xlsx",
		SourceSHA256:   strings.Repeat("a", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedWithAmount(8000),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "settlement-corrected.xlsx",
		SourceSHA256:   strings.Repeat("b", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedWithAmount(9500),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND note='烘焙费'", customerID, 1)
	var totalCents int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(ROUND(SUM(amount) * 100),0)::bigint
		FROM %s.customer_fee_items
		WHERE customer_id=$1
	`, schema), customerID).Scan(&totalCents); err != nil {
		t.Fatalf("sum corrected fees: %v", err)
	}
	if totalCents != 9500 {
		t.Fatalf("total fee cents after corrected settlement reimport = %d, want 9500", totalCents)
	}
}

func TestApplySettlementImportReimportCorrectedFeeNameUpdatesExistingFeeItem(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('结算费用名称修正客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
	}

	parsedFee := func(feeName string, amountCents int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeSettlementWorkbook,
			Rows: []app.ParsedRow{
				{
					SheetName:   "结算单",
					RowNo:       3,
					RowType:     "fee_item",
					ExternalKey: fmt.Sprintf("fee_item:storage:3:%s", feeName),
					Payload: map[string]any{
						"fee_type":     "storage",
						"fee_name":     feeName,
						"amount_cents": amountCents,
						"date":         "2026-03-04",
					},
				},
			},
			Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, FeeItems: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "settlement-fee-name-original.xlsx",
		SourceSHA256:   strings.Repeat("3", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedFee("仓储费旧名称", 8000),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "settlement-fee-name-corrected.xlsx",
		SourceSHA256:   strings.Repeat("4", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedFee("仓储费新名称", 9500),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var feeCount, totalCents int64
	var notes string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(ROUND(SUM(amount) * 100),0)::bigint, COALESCE(STRING_AGG(note, ',' ORDER BY id),'')
		FROM %s.customer_fee_items
		WHERE customer_id=$1 AND source_type='customer_fulfillment_import'
	`, schema), customerID).Scan(&feeCount, &totalCents, &notes); err != nil {
		t.Fatalf("sum corrected fee-name rows: %v", err)
	}
	if feeCount != 1 || totalCents != 9500 || notes != "仓储费新名称" {
		t.Fatalf("corrected settlement fee name = count %d total %d notes %q, want one latest fee", feeCount, totalCents, notes)
	}
}

func TestApplySettlementImportReimportCorrectedFeeTypeUpdatesExistingFeeItem(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('结算费用类型修正客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
	}

	parsedFee := func(feeType, feeName string, amountCents int64) app.ParsedWorkbook {
		return app.ParsedWorkbook{
			ImportType: app.ImportTypeSettlementWorkbook,
			Rows: []app.ParsedRow{
				{
					SheetName:   "结算单",
					RowNo:       3,
					RowType:     "fee_item",
					ExternalKey: fmt.Sprintf("fee_item:%s:3:%s", feeType, feeName),
					Payload: map[string]any{
						"fee_type":     feeType,
						"fee_name":     feeName,
						"amount_cents": amountCents,
						"date":         "2026-03-04",
					},
				},
			},
			Summary: app.ImportSummary{TotalRows: 1, ValidRows: 1, FeeItems: 1},
		}
	}

	firstBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "settlement-fee-type-original.xlsx",
		SourceSHA256:   strings.Repeat("5", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedFee("storage", "仓储费", 8000),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport first: %v", err)
	}
	secondBatch, err := repo.StoreParsedImport(ctx, app.StoreParsedImportCommand{
		CustomerID:     customerID,
		ImportType:     app.ImportTypeSettlementWorkbook,
		SourceFilename: "settlement-fee-type-corrected.xlsx",
		SourceSHA256:   strings.Repeat("6", 64),
		CreatedBy:      "Codex",
		Parsed:         parsedFee("shipping", "物流费", 9500),
	})
	if err != nil {
		t.Fatalf("StoreParsedImport second: %v", err)
	}

	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: firstBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport first batch: %v", err)
	}
	if _, err := repo.ApplyImport(ctx, app.ApplyImportCommand{BatchID: secondBatch.ID, Actor: "Codex"}); err != nil {
		t.Fatalf("ApplyImport corrected batch: %v", err)
	}

	var feeCount, totalCents int64
	var feeTypes, notes string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*), COALESCE(ROUND(SUM(amount) * 100),0)::bigint,
			COALESCE(STRING_AGG(fee_type, ',' ORDER BY id),''),
			COALESCE(STRING_AGG(note, ',' ORDER BY id),'')
		FROM %s.customer_fee_items
		WHERE customer_id=$1 AND source_type='customer_fulfillment_import'
	`, schema), customerID).Scan(&feeCount, &totalCents, &feeTypes, &notes); err != nil {
		t.Fatalf("sum corrected fee-type rows: %v", err)
	}
	if feeCount != 1 || totalCents != 9500 || feeTypes != "shipping" || notes != "物流费" {
		t.Fatalf("corrected settlement fee type = count %d total %d types %q notes %q, want one latest fee", feeCount, totalCents, feeTypes, notes)
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
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
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

func TestCreateSettlementDuplicatePeriodKeepsExistingBatchTotals(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('重复结算客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_fee_items(customer_id, source_type, source_id, fee_type, amount, occurred_at, status)
		VALUES
			($1,'manual',1,'processing',80,'2026-03-04','unsettled'),
			($1,'manual',2,'shipping',24,'2026-03-05','unsettled')
	`, schema), customerID); err != nil {
		t.Fatalf("insert fees: %v", err)
	}

	first, err := repo.CreateSettlement(ctx, app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: "2026-03-01",
		PeriodTo:   "2026-03-31",
		CreatedBy:  "Codex",
	})
	if err != nil {
		t.Fatalf("CreateSettlement first: %v", err)
	}
	second, err := repo.CreateSettlement(ctx, app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: "2026-03-01",
		PeriodTo:   "2026-03-31",
		CreatedBy:  "Codex",
	})
	if err != nil {
		t.Fatalf("CreateSettlement duplicate: %v", err)
	}
	if second.BatchID != first.BatchID || second.FeeItems != 2 || second.TotalAmountCents != 10400 {
		t.Fatalf("duplicate settlement result = %#v, want same batch %d with 2 fee rows and 10400 cents", second, first.BatchID)
	}
	var totalCents int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(ROUND(total_amount * 100),0)::bigint FROM %s.customer_settlement_batches WHERE id=$1`, schema), first.BatchID).Scan(&totalCents); err != nil {
		t.Fatalf("query settlement total: %v", err)
	}
	if totalCents != 10400 {
		t.Fatalf("settlement batch total cents = %d, want 10400 after duplicate create", totalCents)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND settlement_batch_id="+fmt.Sprint(first.BatchID)+" AND status='settled'", customerID, 2)
}

func TestCreateSettlementRejectsEmptyPeriodWithoutWritingBatch(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('空月结客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_fee_items(customer_id, source_type, source_id, fee_type, amount, occurred_at, status)
		VALUES($1,'manual',9,'shipping',24,'2026-04-05','unsettled')
	`, schema), customerID); err != nil {
		t.Fatalf("insert out-of-period fee: %v", err)
	}

	_, err := repo.CreateSettlement(ctx, app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: "2026-03-01",
		PeriodTo:   "2026-03-31",
		CreatedBy:  "Codex",
	})
	if err == nil || !strings.Contains(err.Error(), "no fees for settlement period") {
		t.Fatalf("CreateSettlement empty period err=%v, want no fees for settlement period", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_settlement_batches", "customer_id=$1", customerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND settlement_batch_id=0 AND status='unsettled'", customerID, 1)
}

func TestCreateSettlementRejectsNonDraftExistingBatchWithoutChangingFees(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name) VALUES('已结算批次客户') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_service_capabilities(customer_id, capability_code, enabled)
		VALUES($1,'settlement',true)
	`, schema), customerID); err != nil {
		t.Fatalf("insert settlement capability: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_fee_items(customer_id, source_type, source_id, fee_type, amount, occurred_at, status)
		VALUES
			($1,'manual',1,'processing',80,'2026-03-04','unsettled'),
			($1,'manual',2,'shipping',24,'2026-03-05','unsettled')
	`, schema), customerID); err != nil {
		t.Fatalf("insert initial fees: %v", err)
	}
	first, err := repo.CreateSettlement(ctx, app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: "2026-03-01",
		PeriodTo:   "2026-03-31",
		CreatedBy:  "Codex",
	})
	if err != nil {
		t.Fatalf("CreateSettlement first: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_settlement_batches SET status='settled' WHERE id=$1`, schema), first.BatchID); err != nil {
		t.Fatalf("mark settlement settled: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_fee_items(customer_id, source_type, source_id, fee_type, amount, occurred_at, status)
		VALUES($1,'manual',3,'shipping',5,'2026-03-06','unsettled')
	`, schema), customerID); err != nil {
		t.Fatalf("insert late fee: %v", err)
	}

	_, err = repo.CreateSettlement(ctx, app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: "2026-03-01",
		PeriodTo:   "2026-03-31",
		CreatedBy:  "Codex",
	})
	if err == nil || !strings.Contains(err.Error(), "settlement batch is not draft") {
		t.Fatalf("CreateSettlement non-draft err=%v, want settlement batch is not draft", err)
	}
	var totalCents int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(ROUND(total_amount * 100),0)::bigint FROM %s.customer_settlement_batches WHERE id=$1`, schema), first.BatchID).Scan(&totalCents); err != nil {
		t.Fatalf("query settlement total: %v", err)
	}
	if totalCents != 10400 {
		t.Fatalf("settlement batch total cents = %d, want unchanged 10400", totalCents)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND settlement_batch_id=0 AND status='unsettled'", customerID, 1)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND settlement_batch_id="+fmt.Sprint(first.BatchID)+" AND status='settled'", customerID, 2)
}

func TestCreateSettlementRejectsCustomerWithoutSettlementCapability(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.customers(name, customer_type) VALUES('零售商城客户','retail') RETURNING id
	`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_fee_items(customer_id, source_type, source_id, fee_type, amount, occurred_at, status)
		VALUES($1,'manual',1,'shipping',24,'2026-03-05','unsettled')
	`, schema), customerID); err != nil {
		t.Fatalf("insert fees: %v", err)
	}

	_, err := repo.CreateSettlement(ctx, app.CreateSettlementCommand{
		CustomerID: customerID,
		PeriodFrom: "2026-03-01",
		PeriodTo:   "2026-03-31",
		CreatedBy:  "Codex",
	})
	if err == nil || !strings.Contains(err.Error(), "customer capability settlement unavailable") {
		t.Fatalf("CreateSettlement err=%v, want settlement capability unavailable", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_settlement_batches", "customer_id=$1", customerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "customer_fee_items", "customer_id=$1 AND settlement_batch_id=0 AND status='unsettled'", customerID, 1)
}

func TestUpsertCustomerERPBindingDoesNotGrantHiddenTemplateRoles(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name) VALUES('客户A') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('客户A账号','13900000001','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
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
	assertCustomerFulfillmentCount(t, pool, schema, "employee_roles", "employee_id=$1", employeeID, 0)
}

func TestUpsertCustomerERPBindingRejectsTemplateWithoutERPWorkbench(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name, customer_type) VALUES('零售商城客户','retail') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('零售账号','13900000002','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'retail_mall')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}

	_, err := repo.UpsertCustomerERPBinding(ctx, app.UpsertCustomerERPBindingCommand{
		CustomerID: customerID,
		EmployeeID: employeeID,
		Role:       "customer",
		Status:     "active",
		Actor:      "Codex",
	})
	if err == nil || !strings.Contains(err.Error(), "ERP workbench unavailable for capability template") {
		t.Fatalf("UpsertCustomerERPBinding err=%v, want ERP workbench template rejection", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_erp_user_bindings", "customer_id=$1 AND status='active'", customerID, 0)
	assertCustomerFulfillmentCount(t, pool, schema, "employee_roles", "employee_id=$1", employeeID, 0)
}

func TestUpsertCustomerERPBindingRejectsUnknownTemplateKey(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name, customer_type) VALUES('未知模板客户','wholesale') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('未知模板账号','13900000004','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'legacy_unknown_template')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}

	_, err := repo.UpsertCustomerERPBinding(ctx, app.UpsertCustomerERPBindingCommand{
		CustomerID: customerID,
		EmployeeID: employeeID,
		Role:       "customer",
		Status:     "active",
		Actor:      "Codex",
	})
	if err == nil || !strings.Contains(err.Error(), "ERP workbench unavailable for capability template") {
		t.Fatalf("UpsertCustomerERPBinding err=%v, want unknown template rejection", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_erp_user_bindings", "customer_id=$1 AND status='active'", customerID, 0)
}

func TestUpsertCustomerERPBindingRejectsDisabledLoginAccount(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name, customer_type) VALUES('登录停用客户','wholesale') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('登录停用账号','13900000006','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled)
		VALUES($1,'disabled-hash',true)
	`, schema), employeeID); err != nil {
		t.Fatalf("insert disabled login: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'processing_fulfillment')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}

	_, err := repo.UpsertCustomerERPBinding(ctx, app.UpsertCustomerERPBindingCommand{
		CustomerID: customerID,
		EmployeeID: employeeID,
		Role:       "customer",
		Status:     "active",
		Actor:      "Codex",
	})
	if err == nil || !strings.Contains(err.Error(), "login-enabled channel customer account required") {
		t.Fatalf("UpsertCustomerERPBinding err=%v, want login-enabled channel account rejection", err)
	}
	assertCustomerFulfillmentCount(t, pool, schema, "customer_erp_user_bindings", "customer_id=$1 AND status='active'", customerID, 0)
}

func TestCustomerPortalContextRejectsLegacyBindingWithoutERPWorkbench(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name, customer_type) VALUES('零售商城客户','retail') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('历史零售工作台账号','13900000003','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'retail_mall')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by)
		VALUES($1,$2,'customer','active','legacy')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert legacy binding: %v", err)
	}

	_, err := repo.CustomerPortalContext(ctx, employeeID)
	if !errors.Is(err, app.ErrCustomerERPBindingNotFound) {
		t.Fatalf("CustomerPortalContext err=%v, want ErrCustomerERPBindingNotFound for non-workbench template", err)
	}
	_, err = repo.CustomerPortalOverview(ctx, employeeID)
	if !errors.Is(err, app.ErrCustomerERPBindingNotFound) {
		t.Fatalf("CustomerPortalOverview err=%v, want ErrCustomerERPBindingNotFound for non-workbench template", err)
	}
}

func TestCustomerPortalContextRejectsDisabledLoginBinding(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name, customer_type) VALUES('禁用绑定客户','wholesale') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('历史禁用工作台账号','13900000007','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.employee_login_passwords(employee_id, password_hash, login_disabled)
		VALUES($1,'disabled-hash',true)
	`, schema), employeeID); err != nil {
		t.Fatalf("insert disabled login: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'processing_fulfillment')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by)
		VALUES($1,$2,'customer','active','legacy')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert legacy binding: %v", err)
	}

	_, err := repo.CustomerPortalContext(ctx, employeeID)
	if !errors.Is(err, app.ErrCustomerERPBindingNotFound) {
		t.Fatalf("CustomerPortalContext err=%v, want ErrCustomerERPBindingNotFound for disabled login account", err)
	}
	_, err = repo.CustomerPortalOverview(ctx, employeeID)
	if !errors.Is(err, app.ErrCustomerERPBindingNotFound) {
		t.Fatalf("CustomerPortalOverview err=%v, want ErrCustomerERPBindingNotFound for disabled login account", err)
	}

	bindings, err := repo.ListCustomerERPBindings(ctx, customerID)
	if err != nil {
		t.Fatalf("ListCustomerERPBindings: %v", err)
	}
	for _, binding := range bindings {
		if binding.EmployeeID == employeeID && binding.Status == "active" {
			t.Fatalf("ListCustomerERPBindings returned active disabled binding: %+v", binding)
		}
	}
}

func TestCustomerPortalContextRejectsLegacyBindingWithUnknownTemplateKey(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID, employeeID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name, customer_type) VALUES('未知模板客户','wholesale') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.company_employees(name, phone, account_type, department_id, active)
		VALUES('历史未知模板账号','13900000005','channel_customer',(SELECT id FROM %[1]s.company_departments WHERE name='销售' LIMIT 1),true)
		RETURNING id
	`, schema)).Scan(&employeeID); err != nil {
		t.Fatalf("insert employee: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'legacy_unknown_template')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.customer_erp_user_bindings(customer_id, employee_id, role, status, updated_by)
		VALUES($1,$2,'customer','active','legacy')
	`, schema), customerID, employeeID); err != nil {
		t.Fatalf("insert legacy binding: %v", err)
	}

	_, err := repo.CustomerPortalContext(ctx, employeeID)
	if !errors.Is(err, app.ErrCustomerERPBindingNotFound) {
		t.Fatalf("CustomerPortalContext err=%v, want ErrCustomerERPBindingNotFound for unknown template", err)
	}
}

func TestCustomerERPWorkbenchAvailableRejectsTemplateWithoutWorkbench(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name, customer_type) VALUES('零售商城客户','wholesale') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'retail_mall')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}

	available, err := repo.CustomerERPWorkbenchAvailable(ctx, customerID)
	if err != nil {
		t.Fatalf("CustomerERPWorkbenchAvailable err=%v", err)
	}
	if available {
		t.Fatalf("CustomerERPWorkbenchAvailable = true, want false for retail_mall template")
	}
}

func TestCustomerERPWorkbenchAvailableRejectsUnknownTemplateKey(t *testing.T) {
	ctx := context.Background()
	pool, schema := newCustomerFulfillmentTestDB(t)
	repo := NewRepository(pool, schema)

	var customerID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`INSERT INTO %s.customers(name, customer_type) VALUES('未知模板客户','wholesale') RETURNING id`, schema)).Scan(&customerID); err != nil {
		t.Fatalf("insert customer: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.customer_portal_profiles(customer_id, capability_template_key)
		VALUES($1,'legacy_unknown_template')
	`, schema), customerID); err != nil {
		t.Fatalf("insert portal profile: %v", err)
	}

	available, err := repo.CustomerERPWorkbenchAvailable(ctx, customerID)
	if err != nil {
		t.Fatalf("CustomerERPWorkbenchAvailable err=%v", err)
	}
	if available {
		t.Fatalf("CustomerERPWorkbenchAvailable = true, want false for unknown template")
	}
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
	if err := postgrescompany.EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatalf("company.EnsureSchema: %v", err)
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
