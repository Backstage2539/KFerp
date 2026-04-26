package production

import (
	"context"
	"testing"
)

type fakeFlowRepo struct {
	start            StartCommand
	startNeeds       []StartNeed
	yieldByProductID map[int64]float64
	allocatedNeeds   []StartNeed
	allocatedBy      string
	savedBatchID     string
	savedNeeds       []StartNeed
	savedInputByKey  map[string]int64
	savedYieldMap    map[int64]float64
	savedBy          string
	statusNeeds      []StartNeed
	statusName       string
	finish           FinishCommand
	cancel           CancelCommand
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

func (r *fakeFlowRepo) LoadProductYieldRates(ctx context.Context) (map[int64]float64, error) {
	return r.yieldByProductID, nil
}

func (r *fakeFlowRepo) AllocateStartBatch(ctx context.Context, needs []StartNeed, operator string) (string, error) {
	r.allocatedNeeds = append([]StartNeed(nil), needs...)
	r.allocatedBy = operator
	return "PB-9", nil
}

func (r *fakeFlowRepo) SaveRunningItems(ctx context.Context, batchID string, needs []StartNeed, inputByKey map[string]int64, yieldByProductID map[int64]float64, operator string) error {
	r.savedBatchID = batchID
	r.savedNeeds = append([]StartNeed(nil), needs...)
	r.savedInputByKey = inputByKey
	r.savedYieldMap = yieldByProductID
	r.savedBy = operator
	return nil
}

func (r *fakeFlowRepo) SetOrdersProcessStatus(ctx context.Context, needs []StartNeed, statusName string) error {
	r.statusNeeds = append([]StartNeed(nil), needs...)
	r.statusName = statusName
	return nil
}

func (r *fakeFlowRepo) Finish(ctx context.Context, cmd FinishCommand) error {
	r.finish = cmd
	return nil
}

func (r *fakeFlowRepo) Cancel(ctx context.Context, cmd CancelCommand) error {
	r.cancel = cmd
	return nil
}

func TestServiceOwnsRunningProductionUseCases(t *testing.T) {
	repo := &fakeFlowRepo{
		startNeeds: []StartNeed{
			{ProductID: 1, ProductName: "橘皮乌龙", SpecG: 227, GapG: 4540, OrderNos: "SO-1"},
			{ProductID: 2, ProductName: "库存充足", SpecG: 227, GapG: 0, OrderNos: "SO-2"},
		},
		yieldByProductID: map[int64]float64{1: 0.82},
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
	if len(repo.allocatedNeeds) != 1 || repo.allocatedNeeds[0].ProductID != 1 || repo.allocatedBy != "测试员" {
		t.Fatalf("allocated needs = %+v by=%q", repo.allocatedNeeds, repo.allocatedBy)
	}
	if repo.savedBatchID != "PB-9" || len(repo.savedNeeds) != 1 || repo.savedInputByKey["1-227"] != 600 || repo.savedYieldMap[1] != 0.82 || repo.savedBy != "测试员" {
		t.Fatalf("saved running items batch=%q needs=%+v input=%+v yield=%+v by=%q", repo.savedBatchID, repo.savedNeeds, repo.savedInputByKey, repo.savedYieldMap, repo.savedBy)
	}
	if repo.statusName != "生产中" || len(repo.statusNeeds) != 1 {
		t.Fatalf("status update = %q %+v", repo.statusName, repo.statusNeeds)
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
