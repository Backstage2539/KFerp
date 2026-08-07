package sales

import (
	"context"
	"testing"
)

func (r *fakeRepo) ListProcessingBillingCandidates(ctx context.Context, customerID int64) ([]ProcessingBillingCandidate, error) {
	r.processingBillingCustomerID = customerID
	return []ProcessingBillingCandidate{{WorkOrderID: 91, WorkOrderNo: "WO-91", CustomerID: customerID, Status: "completed"}}, nil
}

func (r *fakeRepo) ListProcessingBillingCustomerOptions(context.Context) ([]ProcessingBillingCustomerOption, error) {
	return []ProcessingBillingCustomerOption{{CustomerID: 19, CustomerName: "9.9 COFFEE LAB"}}, nil
}

func (r *fakeRepo) PreviewProcessingBilling(ctx context.Context, cmd PreviewProcessingBillingCommand) (ProcessingBillingPreview, error) {
	r.processingBillingPreviewCmd = cmd
	return ProcessingBillingPreview{CustomerID: cmd.CustomerID, TemplateVersionID: 13, TotalAmount: 88.5}, nil
}

func (r *fakeRepo) ConfirmProcessingBilling(ctx context.Context, cmd ConfirmProcessingBillingCommand) (ProcessingBillingConfirmation, error) {
	r.processingBillingConfirmCmd = cmd
	return ProcessingBillingConfirmation{SettlementBatchID: 41, SettlementNo: "CPB-41", TotalAmount: 88.5}, nil
}

func (r *fakeRepo) ListProcessingBillingRuns(ctx context.Context, query ProcessingBillingRunsQuery) ([]ProcessingBillingRun, error) {
	r.processingBillingRunsQuery = query
	return []ProcessingBillingRun{{ID: 41, CustomerID: query.CustomerID, Status: ProcessingBillingStatusConfirmed}}, nil
}

func (r *fakeRepo) PayProcessingBilling(ctx context.Context, cmd PayProcessingBillingCommand) (ProcessingBillingLifecycleResult, error) {
	r.processingBillingPayCmd = cmd
	return ProcessingBillingLifecycleResult{BillingRunID: cmd.BillingRunID, Status: ProcessingBillingStatusPaid}, nil
}

func (r *fakeRepo) ReverseProcessingBilling(ctx context.Context, cmd ReverseProcessingBillingCommand) (ProcessingBillingLifecycleResult, error) {
	r.processingBillingReverseCmd = cmd
	return ProcessingBillingLifecycleResult{BillingRunID: 42, SourceBillingRunID: cmd.BillingRunID, Status: ProcessingBillingStatusConfirmed}, nil
}

func (r *fakeRepo) AdjustProcessingBilling(ctx context.Context, cmd AdjustProcessingBillingCommand) (ProcessingBillingLifecycleResult, error) {
	r.processingBillingAdjustCmd = cmd
	return ProcessingBillingLifecycleResult{BillingRunID: 43, SourceBillingRunID: cmd.BillingRunID, Status: ProcessingBillingStatusConfirmed}, nil
}

func TestOutsourceTemplateRulesValidateAndNormalizeSupportedBillingBases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	err := svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{
		Name:  "  客户代加工模板  ",
		Actor: "  财务甲  ",
		Rules: []OutsourceTemplateRuleInput{
			{FeeType: " processing ", Name: " 烘焙费 ", Basis: " actual_input_kg ", UnitPrice: 12.5, SortOrder: 20},
			{FeeType: "packaging", Name: "包装费", Basis: "actual_units", UnitPrice: 1.2, SortOrder: 30},
			{FeeType: "processing", Name: "固定工单费", Basis: "fixed_per_work_order", UnitPrice: 20, SortOrder: 10},
		},
	})
	if err != nil {
		t.Fatalf("SaveOutsourceTemplate() error = %v", err)
	}
	got := repo.outsourceSaved
	if got.Name != "客户代加工模板" || got.Actor != "财务甲" || len(got.Rules) != 3 {
		t.Fatalf("normalized template command = %+v", got)
	}
	if got.Rules[0].FeeType != "processing" || got.Rules[0].Name != "烘焙费" || got.Rules[0].Basis != BillingBasisActualInputKG {
		t.Fatalf("normalized first rule = %+v", got.Rules[0])
	}

	for _, basis := range []string{
		BillingBasisActualInputKG,
		BillingBasisActualOutputKG,
		BillingBasisActualMinutes,
		BillingBasisActualUnits,
		BillingBasisFixedPerWorkOrder,
		BillingBasisFactoryMaterialActualCost,
	} {
		err := svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{
			Name:  "basis-" + basis,
			Rules: []OutsourceTemplateRuleInput{{FeeType: "processing", Name: basis, Basis: basis, UnitPrice: 1}},
		})
		if err != nil {
			t.Fatalf("supported basis %q rejected: %v", basis, err)
		}
	}
	for _, feeType := range []string{"roasting", "labor", "material", "packaging", "processing", "storage"} {
		err := svc.SaveOutsourceTemplate(context.Background(), SaveOutsourceTemplateCommand{
			Name:  "fee-" + feeType,
			Rules: []OutsourceTemplateRuleInput{{FeeType: feeType, Name: feeType, Basis: BillingBasisActualUnits, UnitPrice: 1}},
		})
		if err != nil {
			t.Fatalf("supported fee type %q rejected: %v", feeType, err)
		}
	}

	invalid := []SaveOutsourceTemplateCommand{
		{Name: "unknown", Rules: []OutsourceTemplateRuleInput{{FeeType: "processing", Name: "坏规则", Basis: "planned_output_kg", UnitPrice: 1}}},
		{Name: "negative", Rules: []OutsourceTemplateRuleInput{{FeeType: "processing", Name: "坏价格", Basis: BillingBasisActualMinutes, UnitPrice: -1}}},
		{Name: "fee", Rules: []OutsourceTemplateRuleInput{{FeeType: "product", Name: "不允许的费用", Basis: BillingBasisActualUnits, UnitPrice: 1}}},
	}
	for _, cmd := range invalid {
		if err := svc.SaveOutsourceTemplate(context.Background(), cmd); err == nil {
			t.Fatalf("invalid rule command %+v accepted", cmd)
		}
	}
}

func TestProcessingBillingLifecycleServiceValidatesAndNormalizesCommands(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	runs, err := svc.ListProcessingBillingRuns(context.Background(), ProcessingBillingRunsQuery{CustomerID: 19, Limit: 500})
	if err != nil || len(runs) != 1 || repo.processingBillingRunsQuery.Limit != 200 {
		t.Fatalf("ListProcessingBillingRuns() runs=%+v query=%+v err=%v", runs, repo.processingBillingRunsQuery, err)
	}
	if _, err := svc.ListProcessingBillingRuns(context.Background(), ProcessingBillingRunsQuery{}); err == nil {
		t.Fatal("list runs without customer accepted")
	}

	paid, err := svc.PayProcessingBilling(context.Background(), PayProcessingBillingCommand{BillingRunID: 41, Actor: " 财务甲 ", Note: " 微信收款 "})
	if err != nil || paid.Status != ProcessingBillingStatusPaid || repo.processingBillingPayCmd.Actor != "财务甲" || repo.processingBillingPayCmd.Note != "微信收款" {
		t.Fatalf("PayProcessingBilling() result=%+v cmd=%+v err=%v", paid, repo.processingBillingPayCmd, err)
	}
	if _, err := svc.PayProcessingBilling(context.Background(), PayProcessingBillingCommand{}); err == nil {
		t.Fatal("pay without billing run accepted")
	}

	reversed, err := svc.ReverseProcessingBilling(context.Background(), ReverseProcessingBillingCommand{BillingRunID: 41, Actor: " 财务甲 ", Reason: " 重复计费 "})
	if err != nil || reversed.SourceBillingRunID != 41 || repo.processingBillingReverseCmd.Reason != "重复计费" {
		t.Fatalf("ReverseProcessingBilling() result=%+v cmd=%+v err=%v", reversed, repo.processingBillingReverseCmd, err)
	}
	if _, err := svc.ReverseProcessingBilling(context.Background(), ReverseProcessingBillingCommand{BillingRunID: 41}); err == nil {
		t.Fatal("reverse without reason accepted")
	}

	adjusted, err := svc.AdjustProcessingBilling(context.Background(), AdjustProcessingBillingCommand{
		BillingRunID: 41,
		Actor:        " 财务甲 ",
		Reason:       " 补收人工费 ",
		Lines: []ProcessingBillingAdjustmentLineInput{{
			WorkOrderID: 91, FeeType: " labor ", FeeName: " 补收人工 ", Amount: 12.5,
		}},
	})
	if err != nil || adjusted.SourceBillingRunID != 41 || repo.processingBillingAdjustCmd.RequestKey == "" {
		t.Fatalf("AdjustProcessingBilling() result=%+v cmd=%+v err=%v", adjusted, repo.processingBillingAdjustCmd, err)
	}
	line := repo.processingBillingAdjustCmd.Lines[0]
	if repo.processingBillingAdjustCmd.Actor != "财务甲" || repo.processingBillingAdjustCmd.Reason != "补收人工费" || line.FeeType != "labor" || line.FeeName != "补收人工" {
		t.Fatalf("normalized adjustment cmd=%+v", repo.processingBillingAdjustCmd)
	}
	for _, cmd := range []AdjustProcessingBillingCommand{
		{BillingRunID: 41, Reason: "缺行"},
		{BillingRunID: 41, Reason: "零金额", Lines: []ProcessingBillingAdjustmentLineInput{{FeeType: "labor", FeeName: "人工", Amount: 0}}},
		{BillingRunID: 41, Reason: "错误类型", Lines: []ProcessingBillingAdjustmentLineInput{{FeeType: "product", FeeName: "商品", Amount: 1}}},
	} {
		if _, err := svc.AdjustProcessingBilling(context.Background(), cmd); err == nil {
			t.Fatalf("invalid adjustment accepted: %+v", cmd)
		}
	}
}

func TestProcessingBillingServiceValidatesPreviewAndConfirmContract(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	rows, err := svc.ListProcessingBillingCandidates(context.Background(), 19)
	if err != nil || len(rows) != 1 || repo.processingBillingCustomerID != 19 {
		t.Fatalf("ListProcessingBillingCandidates() rows=%+v customer=%d err=%v", rows, repo.processingBillingCustomerID, err)
	}
	if _, err := svc.ListProcessingBillingCandidates(context.Background(), 0); err == nil {
		t.Fatal("zero customer candidate request accepted")
	}

	preview, err := svc.PreviewProcessingBilling(context.Background(), PreviewProcessingBillingCommand{
		CustomerID:   19,
		TemplateID:   7,
		WorkOrderIDs: []int64{91, 91, 92},
	})
	if err != nil || preview.TemplateVersionID != 13 {
		t.Fatalf("PreviewProcessingBilling() preview=%+v err=%v", preview, err)
	}
	if got := repo.processingBillingPreviewCmd.WorkOrderIDs; len(got) != 2 || got[0] != 91 || got[1] != 92 {
		t.Fatalf("preview work order ids = %v, want deduplicated stable order", got)
	}
	if _, err := svc.PreviewProcessingBilling(context.Background(), PreviewProcessingBillingCommand{CustomerID: 19, TemplateID: 7}); err == nil {
		t.Fatal("preview without work orders accepted")
	}

	confirmed, err := svc.ConfirmProcessingBilling(context.Background(), ConfirmProcessingBillingCommand{
		CustomerID:        19,
		TemplateVersionID: 13,
		WorkOrderIDs:      []int64{91},
		Actor:             " 财务甲 ",
	})
	if err != nil || confirmed.SettlementBatchID != 41 {
		t.Fatalf("ConfirmProcessingBilling() result=%+v err=%v", confirmed, err)
	}
	if repo.processingBillingConfirmCmd.Actor != "财务甲" {
		t.Fatalf("confirm actor = %q", repo.processingBillingConfirmCmd.Actor)
	}
	if _, err := svc.ConfirmProcessingBilling(context.Background(), ConfirmProcessingBillingCommand{CustomerID: 19, WorkOrderIDs: []int64{91}}); err == nil {
		t.Fatal("confirm without immutable template version accepted")
	}
}

func TestProcessingBillingOptionsOnlyExposePublishedBillableTemplatesAndCustomers(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	options, err := svc.GetProcessingBillingOptions(context.Background())
	if err != nil {
		t.Fatalf("GetProcessingBillingOptions() error=%v", err)
	}
	if len(options.Customers) != 1 || options.Customers[0].CustomerID != 19 {
		t.Fatalf("customers=%+v", options.Customers)
	}
	if len(options.Templates) != 0 {
		t.Fatalf("legacy template without a published rule version leaked into billing options: %+v", options.Templates)
	}
}

func TestCalculateProcessingBillingLinesCoversAllSupportedActualBases(t *testing.T) {
	metrics := ProcessingBillingMetrics{
		WorkOrderID:               91,
		ActualInputKG:             10,
		ActualOutputKG:            8.25,
		ActualMinutes:             75,
		ActualUnits:               20,
		FactoryMaterialActualCost: 168.4,
	}
	rules := []OutsourceTemplateRule{
		{ID: 1, FeeType: "processing", Name: "投入", Basis: BillingBasisActualInputKG, UnitPrice: 2},
		{ID: 2, FeeType: "processing", Name: "产出", Basis: BillingBasisActualOutputKG, UnitPrice: 3},
		{ID: 3, FeeType: "processing", Name: "人工", Basis: BillingBasisActualMinutes, UnitPrice: .5},
		{ID: 4, FeeType: "packaging", Name: "件数", Basis: BillingBasisActualUnits, UnitPrice: 1.25},
		{ID: 5, FeeType: "processing", Name: "固定", Basis: BillingBasisFixedPerWorkOrder, UnitPrice: 9.9},
		{ID: 6, FeeType: "processing", Name: "工厂物料", Basis: BillingBasisFactoryMaterialActualCost, UnitPrice: 1.1},
	}
	lines, total, err := CalculateProcessingBillingLines(metrics, rules)
	if err != nil {
		t.Fatalf("CalculateProcessingBillingLines() error=%v", err)
	}
	if len(lines) != 6 {
		t.Fatalf("line count=%d, want 6", len(lines))
	}
	if total != 302.39 {
		t.Fatalf("total=%v, want 302.39; lines=%+v", total, lines)
	}
	if lines[5].BaseQuantity != 168.4 || lines[5].Amount != 185.24 {
		t.Fatalf("factory material line=%+v", lines[5])
	}
}
