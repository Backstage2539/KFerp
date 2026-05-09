package customerfulfillment

import (
	"bytes"
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestServiceParseImportRequiresCustomerAndFile(t *testing.T) {
	svc := NewService(&fakeCustomerFulfillmentRepository{})
	if _, err := svc.ParseImport(context.Background(), ParseImportCommand{
		ImportType: ImportTypeDirectShipWorkbook,
		Reader:     strings.NewReader("not an xlsx"),
	}); err == nil || !strings.Contains(err.Error(), "customer") {
		t.Fatalf("ParseImport without customer error = %v, want customer validation", err)
	}
	if _, err := svc.ParseImport(context.Background(), ParseImportCommand{
		CustomerID: 1,
		ImportType: ImportTypeDirectShipWorkbook,
	}); err == nil || !strings.Contains(err.Error(), "file") {
		t.Fatalf("ParseImport without reader error = %v, want file validation", err)
	}
}

func TestServiceParseImportStoresParsedRowsWithSHA(t *testing.T) {
	wb := directShipWorkbookForServiceTest(t)
	repo := &fakeCustomerFulfillmentRepository{
		storeResult: ImportBatch{ID: 9, CustomerID: 42, ImportType: ImportTypeDirectShipWorkbook, SourceFilename: "誉观山&口加-代发.xlsx"},
	}
	svc := NewService(repo)

	got, err := svc.ParseImport(context.Background(), ParseImportCommand{
		CustomerID:     42,
		ImportType:     ImportTypeDirectShipWorkbook,
		SourceFilename: "  誉观山&口加-代发.xlsx  ",
		Reader:         bytes.NewReader(mustWorkbookBytes(t, wb)),
		CreatedBy:      "Codex",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != 9 {
		t.Fatalf("batch ID = %d, want 9", got.ID)
	}
	if repo.storeCmd.CustomerID != 42 || repo.storeCmd.ImportType != ImportTypeDirectShipWorkbook {
		t.Fatalf("store command customer/import = %d/%s", repo.storeCmd.CustomerID, repo.storeCmd.ImportType)
	}
	if repo.storeCmd.SourceFilename != "誉观山&口加-代发.xlsx" {
		t.Fatalf("SourceFilename = %q, want trimmed filename", repo.storeCmd.SourceFilename)
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(repo.storeCmd.SourceSHA256) {
		t.Fatalf("SourceSHA256 = %q, want lowercase sha256 hex", repo.storeCmd.SourceSHA256)
	}
	if repo.storeCmd.Parsed.Summary.DirectShipOrders != 1 || repo.storeCmd.Parsed.Summary.DirectShipItems != 1 {
		t.Fatalf("stored parsed summary = %#v", repo.storeCmd.Parsed.Summary)
	}
}

func TestServiceApplyImportDelegatesToRepository(t *testing.T) {
	repo := &fakeCustomerFulfillmentRepository{
		applyResult: ApplyResult{BatchID: 7, AppliedRows: 3},
	}
	svc := NewService(repo)
	if _, err := svc.ApplyImport(context.Background(), ApplyImportCommand{Actor: "Codex"}); err == nil || !strings.Contains(err.Error(), "batch") {
		t.Fatalf("ApplyImport without batch error = %v, want batch validation", err)
	}

	got, err := svc.ApplyImport(context.Background(), ApplyImportCommand{BatchID: 7, Actor: "Codex"})
	if err != nil {
		t.Fatal(err)
	}
	if got.AppliedRows != 3 || repo.applyCmd.BatchID != 7 || repo.applyCmd.Actor != "Codex" {
		t.Fatalf("apply result/cmd = %#v/%#v", got, repo.applyCmd)
	}
}

func TestServiceSubmitCustomerProcessingWorkOrderRequiresBoundEmployeeAndFields(t *testing.T) {
	repo := &fakeCustomerFulfillmentRepository{
		customerProcessingResult: ProcessingOrderSummary{WorkOrderNo: "CP-20260508-0001", ProductName: "誉观山冷萃豆", Status: "submitted", QuantityG: 5000, Units: 50},
	}
	svc := NewService(repo)
	if _, err := svc.SubmitCustomerProcessingWorkOrder(context.Background(), SubmitCustomerProcessingWorkOrderCommand{
		ProductName:        "誉观山冷萃豆",
		InputQuantityG:     5000,
		PlannedOutputUnits: 50,
	}); err == nil || !strings.Contains(err.Error(), "employee") {
		t.Fatalf("SubmitCustomerProcessingWorkOrder without employee error = %v, want employee validation", err)
	}
	if _, err := svc.SubmitCustomerProcessingWorkOrder(context.Background(), SubmitCustomerProcessingWorkOrderCommand{
		EmployeeID:         23,
		InputQuantityG:     5000,
		PlannedOutputUnits: 50,
	}); err == nil || !strings.Contains(err.Error(), "product") {
		t.Fatalf("SubmitCustomerProcessingWorkOrder without product error = %v, want product validation", err)
	}
	if _, err := svc.SubmitCustomerProcessingWorkOrder(context.Background(), SubmitCustomerProcessingWorkOrderCommand{
		EmployeeID:         23,
		ProductName:        "誉观山冷萃豆",
		PlannedOutputUnits: 50,
	}); err == nil || !strings.Contains(err.Error(), "input") {
		t.Fatalf("SubmitCustomerProcessingWorkOrder without input qty error = %v, want input validation", err)
	}

	got, err := svc.SubmitCustomerProcessingWorkOrder(context.Background(), SubmitCustomerProcessingWorkOrderCommand{
		EmployeeID:         23,
		ProductName:        "  誉观山冷萃豆  ",
		RawBeanName:        "  埃塞花魁  ",
		InputQuantityG:     5000,
		PlannedOutputUnits: 50,
		ExpectedDate:       "2026-05-20",
		Note:               "  急单  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.WorkOrderNo != "CP-20260508-0001" {
		t.Fatalf("work order = %#v", got)
	}
	if repo.customerProcessingCmd.EmployeeID != 23 || repo.customerProcessingCmd.ProductName != "誉观山冷萃豆" || repo.customerProcessingCmd.RawBeanName != "埃塞花魁" || repo.customerProcessingCmd.Note != "急单" {
		t.Fatalf("processing cmd = %#v", repo.customerProcessingCmd)
	}
}

func TestServiceSubmitCustomerDirectShipOrderRequiresRecipientAndItem(t *testing.T) {
	repo := &fakeCustomerFulfillmentRepository{
		customerDirectShipResult: DirectShipOrderSummary{OrderNo: "CDS-20260508-0001", Status: "submitted", ItemCount: 1},
	}
	svc := NewService(repo)
	if _, err := svc.SubmitCustomerDirectShipOrder(context.Background(), SubmitCustomerDirectShipOrderCommand{
		ReceiverName:    "张三",
		ReceiverPhone:   "13800000000",
		ReceiverAddress: "杭州",
		ProductName:     "誉观山冷萃豆",
		QuantityUnits:   1,
	}); err == nil || !strings.Contains(err.Error(), "employee") {
		t.Fatalf("SubmitCustomerDirectShipOrder without employee error = %v, want employee validation", err)
	}
	if _, err := svc.SubmitCustomerDirectShipOrder(context.Background(), SubmitCustomerDirectShipOrderCommand{
		EmployeeID:      23,
		ReceiverPhone:   "13800000000",
		ReceiverAddress: "杭州",
		ProductName:     "誉观山冷萃豆",
		QuantityUnits:   1,
	}); err == nil || !strings.Contains(err.Error(), "receiver_name") {
		t.Fatalf("SubmitCustomerDirectShipOrder without receiver name error = %v, want receiver validation", err)
	}
	if _, err := svc.SubmitCustomerDirectShipOrder(context.Background(), SubmitCustomerDirectShipOrderCommand{
		EmployeeID:      23,
		ReceiverName:    "张三",
		ReceiverPhone:   "13800000000",
		ReceiverAddress: "杭州",
		QuantityUnits:   1,
	}); err == nil || !strings.Contains(err.Error(), "product") {
		t.Fatalf("SubmitCustomerDirectShipOrder without product error = %v, want product validation", err)
	}

	got, err := svc.SubmitCustomerDirectShipOrder(context.Background(), SubmitCustomerDirectShipOrderCommand{
		EmployeeID:      23,
		ReceiverName:    " 张三 ",
		ReceiverPhone:   " 13800000000 ",
		ReceiverAddress: " 浙江杭州 ",
		ProductName:     " 誉观山冷萃豆 ",
		Spec:            "100g",
		QuantityUnits:   2,
		Note:            " 门卫代收 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.OrderNo != "CDS-20260508-0001" {
		t.Fatalf("direct ship order = %#v", got)
	}
	if repo.customerDirectShipCmd.EmployeeID != 23 || repo.customerDirectShipCmd.ReceiverName != "张三" || repo.customerDirectShipCmd.ProductName != "誉观山冷萃豆" || repo.customerDirectShipCmd.Note != "门卫代收" {
		t.Fatalf("direct ship cmd = %#v", repo.customerDirectShipCmd)
	}
}

func TestServiceSubmitInternalCustomerWorkOrderAcceptsExplicitCustomerWithoutBinding(t *testing.T) {
	repo := &fakeCustomerFulfillmentRepository{
		customerProcessingResult: ProcessingOrderSummary{WorkOrderNo: "CP-20260509-0007", Status: "submitted", ProductName: "誉观山冷萃豆", QuantityG: 5000, Units: 50},
		customerDirectShipResult: DirectShipOrderSummary{OrderNo: "CDS-20260509-0008", Status: "submitted", ItemCount: 1},
	}
	svc := NewService(repo)

	if _, err := svc.SubmitCustomerProcessingWorkOrder(context.Background(), SubmitCustomerProcessingWorkOrderCommand{
		CustomerID:         149,
		ProductName:        "誉观山冷萃豆",
		RawBeanName:        "埃塞花魁",
		InputQuantityG:     5000,
		PlannedOutputUnits: 50,
	}); err != nil {
		t.Fatalf("SubmitCustomerProcessingWorkOrder internal err = %v", err)
	}
	if repo.customerProcessingCmd.CustomerID != 149 || repo.customerProcessingCmd.EmployeeID != 0 {
		t.Fatalf("processing cmd = %#v, want explicit customer without employee", repo.customerProcessingCmd)
	}

	if _, err := svc.SubmitCustomerDirectShipOrder(context.Background(), SubmitCustomerDirectShipOrderCommand{
		CustomerID:      149,
		ReceiverName:    "张三",
		ReceiverPhone:   "13800000000",
		ReceiverAddress: "浙江杭州",
		ProductName:     "誉观山冷萃豆",
		QuantityUnits:   1,
	}); err != nil {
		t.Fatalf("SubmitCustomerDirectShipOrder internal err = %v", err)
	}
	if repo.customerDirectShipCmd.CustomerID != 149 || repo.customerDirectShipCmd.EmployeeID != 0 {
		t.Fatalf("direct ship cmd = %#v, want explicit customer without employee", repo.customerDirectShipCmd)
	}
}

func TestServiceAdjustCustodyInventoryRequiresInternalCustomerAndDelta(t *testing.T) {
	repo := &fakeCustomerFulfillmentRepository{
		custodyAdjustmentResult: CustodyBalance{ItemType: "raw_bean", ItemName: "埃塞花魁", QuantityG: 12000},
	}
	svc := NewService(repo)
	if _, err := svc.AdjustCustodyInventory(context.Background(), AdjustCustodyInventoryCommand{
		ItemType:       "raw_bean",
		ItemName:       "埃塞花魁",
		QuantityGDelta: 1000,
	}); err == nil || !strings.Contains(err.Error(), "customer") {
		t.Fatalf("AdjustCustodyInventory without customer error = %v, want customer validation", err)
	}
	if _, err := svc.AdjustCustodyInventory(context.Background(), AdjustCustodyInventoryCommand{
		CustomerID: 149,
		ItemType:   "raw_bean",
		ItemName:   "埃塞花魁",
	}); err == nil || !strings.Contains(err.Error(), "quantity") {
		t.Fatalf("AdjustCustodyInventory without quantity delta error = %v, want quantity validation", err)
	}

	got, err := svc.AdjustCustodyInventory(context.Background(), AdjustCustodyInventoryCommand{
		CustomerID:         149,
		ItemType:           " raw_bean ",
		ItemName:           " 埃塞花魁 ",
		QuantityGDelta:     1000,
		QuantityUnitsDelta: 0,
		Note:               " 手工补录 ",
		Actor:              " Van ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.QuantityG != 12000 {
		t.Fatalf("adjustment result = %#v", got)
	}
	if repo.custodyAdjustmentCmd.CustomerID != 149 || repo.custodyAdjustmentCmd.ItemType != "raw_bean" || repo.custodyAdjustmentCmd.ItemName != "埃塞花魁" || repo.custodyAdjustmentCmd.Actor != "Van" {
		t.Fatalf("adjustment cmd = %#v", repo.custodyAdjustmentCmd)
	}
}

func TestServiceListImportRowsValidatesAndClampsQuery(t *testing.T) {
	repo := &fakeCustomerFulfillmentRepository{
		listImportRowsResult: []ImportRow{{BatchID: 7, SheetName: "生产工单", RowNo: 5, RowType: "processing_work_order", Status: "invalid", Error: "投豆量无效"}},
	}
	svc := NewService(repo)
	if _, err := svc.ListImportRows(context.Background(), ListImportRowsQuery{Status: "invalid"}); err == nil || !strings.Contains(err.Error(), "batch") {
		t.Fatalf("ListImportRows without batch error = %v, want batch validation", err)
	}

	got, err := svc.ListImportRows(context.Background(), ListImportRowsQuery{BatchID: 7, Status: " invalid ", Limit: 999, Offset: -1})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Error != "投豆量无效" {
		t.Fatalf("ListImportRows result = %#v", got)
	}
	if repo.listImportRowsQuery.BatchID != 7 || repo.listImportRowsQuery.Status != "invalid" || repo.listImportRowsQuery.Limit != 500 || repo.listImportRowsQuery.Offset != 0 {
		t.Fatalf("ListImportRows query = %#v, want normalized invalid limit 500 offset 0", repo.listImportRowsQuery)
	}
}

func TestServiceImportPreviewSummarizesBatchWithoutApplying(t *testing.T) {
	repo := &fakeCustomerFulfillmentRepository{
		importBatchResult: ImportBatch{
			ID:             7,
			CustomerID:     42,
			ImportType:     ImportTypeProcessingWorkbook,
			SourceFilename: "誉观山生产工单&物料库存.xlsx",
			Status:         "parsed",
			Summary:        ImportSummary{ValidRows: 44, InvalidRows: 194, ProcessingOrders: 136, RawBeanReceipts: 8},
		},
	}
	svc := NewService(repo)
	if _, err := svc.ImportPreview(context.Background(), ImportPreviewQuery{}); err == nil || !strings.Contains(err.Error(), "batch") {
		t.Fatalf("ImportPreview without batch error = %v, want batch validation", err)
	}

	got, err := svc.ImportPreview(context.Background(), ImportPreviewQuery{BatchID: 7})
	if err != nil {
		t.Fatal(err)
	}
	if got.Batch.ID != 7 || got.Batch.ImportType != ImportTypeProcessingWorkbook {
		t.Fatalf("preview batch = %#v", got.Batch)
	}
	if repo.importBatchID != 7 {
		t.Fatalf("import batch id = %d, want 7", repo.importBatchID)
	}
	wantLabels := []string{"将应用有效行", "需先处理错误行", "托管生豆入库", "加工工单"}
	if len(got.Effects) != len(wantLabels) {
		t.Fatalf("effects = %#v, want %d effects", got.Effects, len(wantLabels))
	}
	for i, label := range wantLabels {
		if got.Effects[i].Label != label {
			t.Fatalf("effect[%d] = %#v, want label %q", i, got.Effects[i], label)
		}
	}
	if !strings.Contains(got.Warning, "不写入业务数据") {
		t.Fatalf("preview warning = %q, want non-mutating warning", got.Warning)
	}
}

func TestServiceCreateSettlementRequiresPeriod(t *testing.T) {
	repo := &fakeCustomerFulfillmentRepository{
		settlementResult: SettlementResult{BatchID: 11, FeeItems: 4},
	}
	svc := NewService(repo)
	for _, cmd := range []CreateSettlementCommand{
		{CustomerID: 1, PeriodTo: "2026-03-31", CreatedBy: "Codex"},
		{CustomerID: 1, PeriodFrom: "2026-03-01", CreatedBy: "Codex"},
		{CustomerID: 1, PeriodFrom: "2026/03/01", PeriodTo: "2026-03-31", CreatedBy: "Codex"},
	} {
		if _, err := svc.CreateSettlement(context.Background(), cmd); err == nil {
			t.Fatalf("CreateSettlement(%#v) error = nil, want period validation", cmd)
		}
	}

	got, err := svc.CreateSettlement(context.Background(), CreateSettlementCommand{
		CustomerID: 1,
		PeriodFrom: "2026-03-01",
		PeriodTo:   "2026-03-31",
		CreatedBy:  "Codex",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.BatchID != 11 || repo.settlementCmd.PeriodFrom != "2026-03-01" || repo.settlementCmd.PeriodTo != "2026-03-31" {
		t.Fatalf("settlement result/cmd = %#v/%#v", got, repo.settlementCmd)
	}
}

func directShipWorkbookForServiceTest(t *testing.T) *excelize.File {
	t.Helper()
	wb := excelize.NewFile()
	mustSetRows(t, wb, "代发信息", [][]any{
		{"时间", "序号", "订单编号", "收货地址", "商品标题", "属性", "商品规格", "数量", "磨粉服务", "备注", "运单号", "发货日期", "状态"},
		{"2026-03-04", "1", "YGS20260304001", "张三 13800000000 浙江杭州", "誉观山花魁", "浅烘", "100g", "1", "不磨粉", "加急", "", "", "待发货"},
	})
	return wb
}

type fakeCustomerFulfillmentRepository struct {
	storeCmd                   StoreParsedImportCommand
	storeResult                ImportBatch
	importBatchID              int64
	importBatchResult          ImportBatch
	applyCmd                   ApplyImportCommand
	applyResult                ApplyResult
	customerContextEmployeeID  int64
	customerContextResult      CustomerERPContext
	customerOverviewEmployeeID int64
	customerOverviewResult     CustomerPortalOverview
	customerProcessingCmd      SubmitCustomerProcessingWorkOrderCommand
	customerProcessingResult   ProcessingOrderSummary
	customerDirectShipCmd      SubmitCustomerDirectShipOrderCommand
	customerDirectShipResult   DirectShipOrderSummary
	custodyAdjustmentCmd       AdjustCustodyInventoryCommand
	custodyAdjustmentResult    CustodyBalance
	erpBindingCmd              UpsertCustomerERPBindingCommand
	erpBindingResult           CustomerERPBinding
	listERPBindingsCustomerID  int64
	listERPBindingsResult      []CustomerERPBinding
	settlementCmd              CreateSettlementCommand
	settlementResult           SettlementResult
	overviewQuery              OverviewQuery
	overviewResult             Overview
	listImportsQuery           ListImportsQuery
	listImportsResult          []ImportBatch
	listImportRowsQuery        ListImportRowsQuery
	listImportRowsResult       []ImportRow
}

func (r *fakeCustomerFulfillmentRepository) StoreParsedImport(ctx context.Context, cmd StoreParsedImportCommand) (ImportBatch, error) {
	r.storeCmd = cmd
	return r.storeResult, nil
}

func (r *fakeCustomerFulfillmentRepository) ApplyImport(ctx context.Context, cmd ApplyImportCommand) (ApplyResult, error) {
	r.applyCmd = cmd
	return r.applyResult, nil
}

func (r *fakeCustomerFulfillmentRepository) CustomerPortalContext(ctx context.Context, employeeID int64) (CustomerERPContext, error) {
	r.customerContextEmployeeID = employeeID
	return r.customerContextResult, nil
}

func (r *fakeCustomerFulfillmentRepository) CustomerPortalOverview(ctx context.Context, employeeID int64) (CustomerPortalOverview, error) {
	r.customerOverviewEmployeeID = employeeID
	return r.customerOverviewResult, nil
}

func (r *fakeCustomerFulfillmentRepository) SubmitCustomerProcessingWorkOrder(ctx context.Context, cmd SubmitCustomerProcessingWorkOrderCommand) (ProcessingOrderSummary, error) {
	r.customerProcessingCmd = cmd
	return r.customerProcessingResult, nil
}

func (r *fakeCustomerFulfillmentRepository) SubmitCustomerDirectShipOrder(ctx context.Context, cmd SubmitCustomerDirectShipOrderCommand) (DirectShipOrderSummary, error) {
	r.customerDirectShipCmd = cmd
	return r.customerDirectShipResult, nil
}

func (r *fakeCustomerFulfillmentRepository) AdjustCustodyInventory(ctx context.Context, cmd AdjustCustodyInventoryCommand) (CustodyBalance, error) {
	r.custodyAdjustmentCmd = cmd
	return r.custodyAdjustmentResult, nil
}

func (r *fakeCustomerFulfillmentRepository) UpsertCustomerERPBinding(ctx context.Context, cmd UpsertCustomerERPBindingCommand) (CustomerERPBinding, error) {
	r.erpBindingCmd = cmd
	return r.erpBindingResult, nil
}

func (r *fakeCustomerFulfillmentRepository) ListCustomerERPBindings(ctx context.Context, customerID int64) ([]CustomerERPBinding, error) {
	r.listERPBindingsCustomerID = customerID
	return r.listERPBindingsResult, nil
}

func (r *fakeCustomerFulfillmentRepository) ImportBatch(ctx context.Context, batchID int64) (ImportBatch, error) {
	r.importBatchID = batchID
	return r.importBatchResult, nil
}

func (r *fakeCustomerFulfillmentRepository) ListImportRows(ctx context.Context, query ListImportRowsQuery) ([]ImportRow, error) {
	r.listImportRowsQuery = query
	return r.listImportRowsResult, nil
}

func (r *fakeCustomerFulfillmentRepository) CreateSettlement(ctx context.Context, cmd CreateSettlementCommand) (SettlementResult, error) {
	r.settlementCmd = cmd
	return r.settlementResult, nil
}

func (r *fakeCustomerFulfillmentRepository) Overview(ctx context.Context, query OverviewQuery) (Overview, error) {
	r.overviewQuery = query
	return r.overviewResult, nil
}

func (r *fakeCustomerFulfillmentRepository) ListImports(ctx context.Context, query ListImportsQuery) ([]ImportBatch, error) {
	r.listImportsQuery = query
	return r.listImportsResult, nil
}
