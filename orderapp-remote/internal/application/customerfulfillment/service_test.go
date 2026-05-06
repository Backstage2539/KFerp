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
	storeCmd          StoreParsedImportCommand
	storeResult       ImportBatch
	applyCmd          ApplyImportCommand
	applyResult       ApplyResult
	settlementCmd     CreateSettlementCommand
	settlementResult  SettlementResult
	overviewQuery     OverviewQuery
	overviewResult    Overview
	listImportsQuery  ListImportsQuery
	listImportsResult []ImportBatch
}

func (r *fakeCustomerFulfillmentRepository) StoreParsedImport(ctx context.Context, cmd StoreParsedImportCommand) (ImportBatch, error) {
	r.storeCmd = cmd
	return r.storeResult, nil
}

func (r *fakeCustomerFulfillmentRepository) ApplyImport(ctx context.Context, cmd ApplyImportCommand) (ApplyResult, error) {
	r.applyCmd = cmd
	return r.applyResult, nil
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
