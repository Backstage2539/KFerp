package production

import (
	"context"
	"testing"
)

type fakeRepo struct {
	create CreateBatchCommand
}

func (r *fakeRepo) CreateBatch(ctx context.Context, cmd CreateBatchCommand) (CreateBatchResult, error) {
	r.create = cmd
	return CreateBatchResult{BatchID: "P1", OrderCount: len(cmd.OrderIDs)}, nil
}

func (r *fakeRepo) ListBatches(ctx context.Context, cmd ListBatchesCommand) ([]BatchListItem, error) {
	return []BatchListItem{{BatchID: "P1"}}, nil
}

func (r *fakeRepo) Detail(ctx context.Context, batchID string) (BatchDetail, error) {
	return BatchDetail{BatchID: batchID}, nil
}

func (r *fakeRepo) PreviewDeduct(ctx context.Context, batchID string) (DeductPreview, error) {
	return DeductPreview{BatchID: batchID}, nil
}

func (r *fakeRepo) ConfirmDeduct(ctx context.Context, batchID, operator string) (DeductConfirmResult, error) {
	return DeductConfirmResult{BatchID: batchID, Status: "deducted"}, nil
}

func (r *fakeRepo) ListRunning(ctx context.Context) ([]RunningItem, error) {
	return nil, nil
}

func (r *fakeRepo) ListStartNeeds(ctx context.Context, cmd StartCommand) ([]StartNeed, error) {
	return nil, nil
}

func (r *fakeRepo) LoadProductYieldRates(ctx context.Context) (map[int64]float64, error) {
	return nil, nil
}

func (r *fakeRepo) AllocateStartBatch(ctx context.Context, needs []StartNeed, operator string) (string, error) {
	return "", nil
}

func (r *fakeRepo) SaveRunningItems(ctx context.Context, batchID string, needs []StartNeed, inputByKey map[string]int64, yieldByProductID map[int64]float64, operator string) error {
	return nil
}

func (r *fakeRepo) SetOrdersProcessStatus(ctx context.Context, needs []StartNeed, statusName string) error {
	return nil
}

func (r *fakeRepo) Finish(ctx context.Context, cmd FinishCommand) error {
	return nil
}

func (r *fakeRepo) Cancel(ctx context.Context, cmd CancelCommand) error {
	return nil
}

func TestServiceDelegatesProductionUseCases(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	res, err := svc.CreateBatch(context.Background(), CreateBatchCommand{OrderIDs: []int64{1, 2}, RequestUnitsByItemID: map[int64]int64{10: 2}})
	if err != nil || res.OrderCount != 2 {
		t.Fatalf("CreateBatch() = %+v, %v", res, err)
	}
	if repo.create.RequestUnitsByItemID[10] != 2 {
		t.Fatalf("create command = %+v", repo.create)
	}

	prev, err := svc.PreviewDeduct(context.Background(), "P1")
	if err != nil || prev.BatchID != "P1" {
		t.Fatalf("PreviewDeduct() = %+v, %v", prev, err)
	}
	conf, err := svc.ConfirmDeduct(context.Background(), "P1", "op")
	if err != nil || conf.Status != "deducted" {
		t.Fatalf("ConfirmDeduct() = %+v, %v", conf, err)
	}
	rows, err := svc.ListBatches(context.Background(), ListBatchesCommand{})
	if err != nil || len(rows) != 1 || rows[0].BatchID != "P1" {
		t.Fatalf("ListBatches() = %+v, %v", rows, err)
	}
	detail, err := svc.Detail(context.Background(), " P1 ")
	if err != nil || detail.BatchID != "P1" {
		t.Fatalf("Detail() = %+v, %v", detail, err)
	}
}
