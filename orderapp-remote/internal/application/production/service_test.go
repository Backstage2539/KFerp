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

func (r *fakeRepo) Start(ctx context.Context, cmd StartExecutionCommand) (StartResult, error) {
	return StartResult{BatchID: "PB-1"}, nil
}

func (r *fakeRepo) Finish(ctx context.Context, cmd FinishCommand) error {
	return nil
}

func (r *fakeRepo) Cancel(ctx context.Context, cmd CancelCommand) error {
	return nil
}

func (r *fakeRepo) ListMachines(ctx context.Context, activeOnly bool) ([]RoastMachine, error) {
	return []RoastMachine{{ID: 1, Name: "小烘焙机", CapacityG: 3000, AllowedSpecs: "1000,2000", MinRoastG: 1000, Active: true}}, nil
}

func (r *fakeRepo) SaveMachine(ctx context.Context, cmd RoastMachineCommand) error {
	return nil
}

func (r *fakeRepo) PlanSummary(ctx context.Context, query PlanSummaryQuery) (PlanSummaryData, error) {
	return PlanSummaryData{}, nil
}

func (r *fakeRepo) ListProductionLogs(ctx context.Context, query ProductionLogsQuery) (ProductionLogsResult, error) {
	return ProductionLogsResult{}, nil
}
func (r *fakeRepo) ListWorkOrders(ctx context.Context, query WorkOrderQuery) ([]WorkOrderRow, error) {
	return nil, nil
}
func (r *fakeRepo) ListJobCards(ctx context.Context, query JobCardQuery) ([]JobCardRow, error) {
	return nil, nil
}
func (r *fakeRepo) ListBatchCosts(ctx context.Context, query BatchCostQuery) ([]BatchCostRow, error) {
	return nil, nil
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
	machines, err := svc.ListMachines(context.Background(), false)
	if err != nil || len(machines) != 1 || machines[0].Name != "小烘焙机" {
		t.Fatalf("ListMachines() = %+v, %v", machines, err)
	}
}

func TestServiceNormalizesMachineCommand(t *testing.T) {
	repo := &machineFakeRepo{}
	svc := NewService(repo)
	if err := svc.SaveMachine(context.Background(), RoastMachineCommand{
		Name:         "  新设备 ",
		CapacityG:    5000,
		MinRoastG:    1000,
		AllowedSpecs: "3000,1000,3000",
		Active:       true,
	}); err != nil {
		t.Fatal(err)
	}
	if repo.machine.Name != "新设备" || repo.machine.AllowedSpecs != "1000,3000" {
		t.Fatalf("machine command = %+v", repo.machine)
	}
}

func TestServiceMachineAllowedSpecsErrorExplainsRoastLoads(t *testing.T) {
	svc := NewService(&machineFakeRepo{})
	err := svc.SaveMachine(context.Background(), RoastMachineCommand{
		Name:         "样机",
		CapacityG:    3000,
		MinRoastG:    1000,
		AllowedSpecs: "227,454",
		Active:       true,
	})
	if err == nil {
		t.Fatal("SaveMachine error = nil, want allowed_specs validation")
	}
	want := "allowed_specs must list roast load grams between min_roast_g and capacity_g"
	if err.Error() != want {
		t.Fatalf("SaveMachine error = %q, want %q", err.Error(), want)
	}
}

type machineFakeRepo struct {
	fakeRepo
	machine RoastMachineCommand
}

func (r *machineFakeRepo) SaveMachine(ctx context.Context, cmd RoastMachineCommand) error {
	r.machine = cmd
	return nil
}
