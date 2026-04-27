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
