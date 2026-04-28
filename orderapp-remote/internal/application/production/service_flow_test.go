package production

import (
	"context"
	"testing"
)

type fakeFlowRepo struct {
	start          StartCommand
	startNeeds     []StartNeed
	startExecution StartExecutionCommand
	finish         FinishCommand
	cancel         CancelCommand

	materialPlanQuery  MaterialPlanQuery
	materialPlanResult MaterialPlanResult
	qualityCommand     QualityInspectionCommand
	qualityQuery       QualityInspectionQuery
	qualityRows        []QualityInspectionRow
}

func (r *fakeFlowRepo) CreateBatch(ctx context.Context, cmd CreateBatchCommand) (CreateBatchResult, error) {
	return CreateBatchResult{}, nil
}

func (r *fakeFlowRepo) ListBatches(ctx context.Context, cmd ListBatchesCommand) ([]BatchListItem, error) {
	return nil, nil
}

func (r *fakeFlowRepo) Detail(ctx context.Context, batchID string) (BatchDetail, error) {
	return BatchDetail{}, nil
}

func (r *fakeFlowRepo) PreviewDeduct(ctx context.Context, batchID string) (DeductPreview, error) {
	return DeductPreview{}, nil
}

func (r *fakeFlowRepo) ConfirmDeduct(ctx context.Context, batchID, operator string) (DeductConfirmResult, error) {
	return DeductConfirmResult{}, nil
}

func (r *fakeFlowRepo) ListRunning(ctx context.Context) ([]RunningItem, error) {
	return []RunningItem{{ID: 7, BatchID: "PB-7", ProductName: "橘皮乌龙", InputG: 600}}, nil
}

func (r *fakeFlowRepo) ListStartNeeds(ctx context.Context, cmd StartCommand) ([]StartNeed, error) {
	r.start = cmd
	return r.startNeeds, nil
}

func (r *fakeFlowRepo) Start(ctx context.Context, cmd StartExecutionCommand) (StartResult, error) {
	r.startExecution = StartExecutionCommand{
		Needs:      append([]StartNeed(nil), cmd.Needs...),
		InputByKey: cmd.InputByKey,
		Operator:   cmd.Operator,
	}
	return StartResult{BatchID: "PB-9"}, nil
}

func (r *fakeFlowRepo) Finish(ctx context.Context, cmd FinishCommand) error {
	r.finish = cmd
	return nil
}

func (r *fakeFlowRepo) Cancel(ctx context.Context, cmd CancelCommand) error {
	r.cancel = cmd
	return nil
}

func (r *fakeFlowRepo) ListMachines(ctx context.Context, activeOnly bool) ([]RoastMachine, error) {
	return nil, nil
}

func (r *fakeFlowRepo) SaveMachine(ctx context.Context, cmd RoastMachineCommand) error {
	return nil
}

func (r *fakeFlowRepo) PlanSummary(ctx context.Context, query PlanSummaryQuery) (PlanSummaryData, error) {
	return PlanSummaryData{}, nil
}

func (r *fakeFlowRepo) ListProductionLogs(ctx context.Context, query ProductionLogsQuery) (ProductionLogsResult, error) {
	return ProductionLogsResult{}, nil
}
func (r *fakeFlowRepo) ListWorkOrders(ctx context.Context, query WorkOrderQuery) ([]WorkOrderRow, error) {
	return nil, nil
}
func (r *fakeFlowRepo) ListJobCards(ctx context.Context, query JobCardQuery) ([]JobCardRow, error) {
	return nil, nil
}
func (r *fakeFlowRepo) ListBatchCosts(ctx context.Context, query BatchCostQuery) ([]BatchCostRow, error) {
	return nil, nil
}

func (r *fakeFlowRepo) MaterialPlan(ctx context.Context, query MaterialPlanQuery) (MaterialPlanResult, error) {
	r.materialPlanQuery = query
	return r.materialPlanResult, nil
}

func (r *fakeFlowRepo) CreateQualityInspection(ctx context.Context, cmd QualityInspectionCommand) (QualityInspectionRow, error) {
	r.qualityCommand = cmd
	return QualityInspectionRow{
		ID:            1,
		Scope:         cmd.Scope,
		ReferenceType: cmd.ReferenceType,
		ReferenceNo:   cmd.ReferenceNo,
		ItemName:      cmd.ItemName,
		Result:        cmd.Result,
		MetricsJSON:   cmd.MetricsJSON,
		Note:          cmd.Note,
		Operator:      cmd.Operator,
		CreatedAt:     "2026-04-28 10:00",
	}, nil
}

func (r *fakeFlowRepo) ListQualityInspections(ctx context.Context, query QualityInspectionQuery) ([]QualityInspectionRow, error) {
	r.qualityQuery = query
	return r.qualityRows, nil
}

func TestServiceOwnsRunningProductionUseCases(t *testing.T) {
	repo := &fakeFlowRepo{
		startNeeds: []StartNeed{
			{ProductID: 1, ProductName: "橘皮乌龙", SpecG: 227, GapG: 4540, OrderNos: "SO-1"},
			{ProductID: 2, ProductName: "库存充足", SpecG: 227, GapG: 0, OrderNos: "SO-2"},
		},
	}
	svc := NewService(repo)
	ctx := context.Background()

	rows, err := svc.ListRunning(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].BatchID != "PB-7" {
		t.Fatalf("ListRunning() = %+v", rows)
	}

	started, err := svc.Start(ctx, StartCommand{
		Selected:   map[string]bool{"1-227": true},
		InputByKey: map[string]int64{"1-227": 600},
		Operator:   "测试员",
	})
	if err != nil {
		t.Fatal(err)
	}
	if started.BatchID != "PB-9" {
		t.Fatalf("Start() = %+v", started)
	}
	if len(repo.startExecution.Needs) != 1 || repo.startExecution.Needs[0].ProductID != 1 || repo.startExecution.InputByKey["1-227"] != 600 || repo.startExecution.Operator != "测试员" {
		t.Fatalf("start execution = %+v", repo.startExecution)
	}

	if err := svc.Finish(ctx, FinishCommand{ID: 7, FinishedUnits: 2, HasFinishedInput: true, Operator: "测试员"}); err != nil {
		t.Fatal(err)
	}
	if repo.finish.ID != 7 || repo.finish.FinishedUnits != 2 || !repo.finish.HasFinishedInput {
		t.Fatalf("Finish command = %+v", repo.finish)
	}

	if err := svc.Cancel(ctx, CancelCommand{ID: 7, Operator: "测试员"}); err != nil {
		t.Fatal(err)
	}
	if repo.cancel.ID != 7 {
		t.Fatalf("Cancel command = %+v", repo.cancel)
	}
}

func TestServiceOwnsManufacturingGapUseCases(t *testing.T) {
	repo := &fakeFlowRepo{
		materialPlanResult: MaterialPlanResult{Rows: []MaterialPlanRow{{
			MaterialID:          10,
			MaterialName:        "孟连水洗5T批次",
			Unit:                "g",
			RequiredG:           60000,
			WIPG:                20000,
			RawG:                30000,
			ReservedG:           5000,
			ShortageG:           15000,
			PurchaseSuggestionG: 15000,
		}}},
		qualityRows: []QualityInspectionRow{{ID: 2, Scope: "work_order", ReferenceNo: "WO-0000000020", Result: "pass"}},
	}
	svc := NewService(repo)
	ctx := context.Background()

	plan, err := svc.MaterialPlan(ctx, MaterialPlanQuery{
		From:       " 2026-04-01 ",
		To:         " 2026-04-28 ",
		Selected:   nil,
		InputByKey: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Rows) != 1 || plan.Rows[0].ShortageG != 15000 || plan.Rows[0].PurchaseSuggestionG != 15000 {
		t.Fatalf("MaterialPlan() = %+v", plan.Rows)
	}
	if repo.materialPlanQuery.From != "2026-04-01" || repo.materialPlanQuery.To != "2026-04-28" {
		t.Fatalf("MaterialPlan query was not trimmed: %+v", repo.materialPlanQuery)
	}
	if repo.materialPlanQuery.Selected == nil || repo.materialPlanQuery.InputByKey == nil {
		t.Fatalf("MaterialPlan query maps must be initialized: %+v", repo.materialPlanQuery)
	}

	if err := svc.Finish(ctx, FinishCommand{
		ID:               7,
		FinishedUnits:    1,
		HasFinishedInput: true,
		Partial:          true,
		ConsumedInputG:   300,
		Operator:         "测试员",
	}); err != nil {
		t.Fatal(err)
	}
	if !repo.finish.Partial || repo.finish.ConsumedInputG != 300 {
		t.Fatalf("partial finish command = %+v", repo.finish)
	}

	created, err := svc.CreateQualityInspection(ctx, QualityInspectionCommand{
		Scope:         " work_order ",
		ReferenceType: " work_order ",
		ReferenceNo:   " WO-0000000020 ",
		ItemName:      " 测试拼配 ",
		Result:        " PASS ",
		Operator:      " 测试员 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Scope != "work_order" || created.Result != "pass" || created.ReferenceNo != "WO-0000000020" {
		t.Fatalf("quality inspection = %+v", created)
	}
	if repo.qualityCommand.Operator != "测试员" {
		t.Fatalf("quality operator was not trimmed: %+v", repo.qualityCommand)
	}

	rows, err := svc.ListQualityInspections(ctx, QualityInspectionQuery{Scope: " work_order ", Result: " pass ", Limit: 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || repo.qualityQuery.Limit != 200 || repo.qualityQuery.Scope != "work_order" || repo.qualityQuery.Result != "pass" {
		t.Fatalf("quality list rows=%+v query=%+v", rows, repo.qualityQuery)
	}
}
