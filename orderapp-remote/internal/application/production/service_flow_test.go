package production

import (
	"context"
	"strings"
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
	reservationQuery   WIPReservationQuery
	reservationRows    []WIPReservationRow
	adjustReservation  WIPReservationAdjustCommand
	releaseReservation WIPReservationReleaseCommand
	acceptanceRows     []AcceptanceSmokeRow
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

func (r *fakeFlowRepo) Finish(ctx context.Context, cmd FinishCommand) (FinishResult, error) {
	r.finish = cmd
	return FinishResult{RunningItemID: cmd.ID}, nil
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
func (r *fakeFlowRepo) UpdateJobCardActuals(ctx context.Context, cmd JobCardActualsCommand) error {
	return nil
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

func (r *fakeFlowRepo) ListWIPReservations(ctx context.Context, query WIPReservationQuery) (WIPReservationResult, error) {
	r.reservationQuery = query
	return WIPReservationResult{Rows: r.reservationRows, TotalRemainingG: 40000}, nil
}

func (r *fakeFlowRepo) AdjustWIPReservation(ctx context.Context, cmd WIPReservationAdjustCommand) (WIPReservationRow, error) {
	r.adjustReservation = cmd
	return WIPReservationRow{ID: cmd.ReservationID, ReservedG: cmd.ReservedG, ConsumedG: 10000, RemainingReservedG: cmd.ReservedG - 10000}, nil
}

func (r *fakeFlowRepo) ReleaseWIPReservations(ctx context.Context, cmd WIPReservationReleaseCommand) (WIPReservationReleaseResult, error) {
	r.releaseReservation = cmd
	return WIPReservationReleaseResult{ReleasedCount: 2, ReleasedG: 40000}, nil
}

func (r *fakeFlowRepo) AcceptanceSmoke(ctx context.Context) (AcceptanceSmokeResult, error) {
	return AcceptanceSmokeResult{Rows: r.acceptanceRows}, nil
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

	if _, err := svc.Finish(ctx, FinishCommand{ID: 7, FinishedUnits: 2, HasFinishedInput: true, Operator: "测试员"}); err != nil {
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

func TestStartRejectsEmptySelectionWithoutOpeningWork(t *testing.T) {
	repo := &fakeFlowRepo{
		startNeeds: []StartNeed{{ProductID: 1, ProductName: "橘皮乌龙", SpecG: 227, GapG: 454, OrderNos: "SO-1"}},
	}
	svc := NewService(repo)

	if _, err := svc.Start(context.Background(), StartCommand{Selected: map[string]bool{}, InputByKey: map[string]int64{}, Operator: "测试员"}); err == nil {
		t.Fatal("empty production selection should fail")
	}
	if len(repo.startExecution.Needs) != 0 || repo.startExecution.Operator != "" {
		t.Fatalf("repo.Start should not be called on empty selection: %+v", repo.startExecution)
	}
}

func TestStartDefaultsSelectedNeedInputWhenPlanRowHasNoEditableRoastInput(t *testing.T) {
	repo := &fakeFlowRepo{
		startNeeds: []StartNeed{{ProductID: 539, ProductName: "PR439-20260606182321 工厂量单商品", SpecG: 454, GapG: 454, OrderNos: "SO-PR439"}},
	}
	svc := NewService(repo)

	if _, err := svc.Start(context.Background(), StartCommand{
		Selected:   map[string]bool{"539-454": true},
		InputByKey: map[string]int64{},
		Operator:   "测试员",
	}); err != nil {
		t.Fatalf("selected production need without explicit input should use default input: %v", err)
	}
	if len(repo.startExecution.Needs) != 1 || repo.startExecution.Needs[0].ProductID != 539 || repo.startExecution.Operator != "测试员" {
		t.Fatalf("repo.Start should receive selected need for default input handling: %+v", repo.startExecution)
	}
	if repo.startExecution.InputByKey == nil {
		t.Fatal("repo.Start input map should be non-nil so repository can apply default input")
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

	if _, err := svc.Finish(ctx, FinishCommand{
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

	created, err = svc.CreateQualityInspection(ctx, QualityInspectionCommand{
		Scope:                    "raw_material",
		ReferenceNo:              "MB-0000000007",
		ItemName:                 "孟连水洗5T批次",
		Result:                   "pass",
		MetricsJSON:              `{"水分":"10.5%"}`,
		FactoryFlavorDescription: " 茉莉花、柑橘 ",
		Moisture:                 " 10.8% ",
		Density:                  " 780g/L ",
	})
	if err != nil {
		t.Fatalf("CreateQualityInspection inbound metrics: %v", err)
	}
	if created.Scope != "raw_material" ||
		!strings.Contains(repo.qualityCommand.MetricsJSON, `"factory_flavor_description":"茉莉花、柑橘"`) ||
		!strings.Contains(repo.qualityCommand.MetricsJSON, `"moisture":"10.8%"`) ||
		!strings.Contains(repo.qualityCommand.MetricsJSON, `"density":"780g/L"`) ||
		!strings.Contains(repo.qualityCommand.MetricsJSON, `"水分":"10.5%"`) {
		t.Fatalf("inbound quality metrics = %+v", repo.qualityCommand)
	}

	created, err = svc.CreateQualityInspection(ctx, QualityInspectionCommand{
		Scope:       "生产工单",
		ReferenceNo: " WO-0000000020 ",
		ItemName:    " 测试拼配 ",
		Result:      "不通过",
		Operator:    " 测试员 ",
	})
	if err != nil {
		t.Fatalf("CreateQualityInspection should accept Chinese failed result: %v", err)
	}
	if created.Scope != "work_order" || created.ReferenceType != "work_order" || created.Result != "reject" {
		t.Fatalf("quality Chinese failed result = %+v", created)
	}

	created, err = svc.CreateQualityInspection(ctx, QualityInspectionCommand{
		Scope:       "work_order",
		ReferenceNo: "WO-0000000020",
		Result:      "待定",
	})
	if err != nil {
		t.Fatalf("CreateQualityInspection should accept Chinese pending result: %v", err)
	}
	if created.Result != "hold" {
		t.Fatalf("quality Chinese pending result = %+v, want hold", created)
	}

	rows, err := svc.ListQualityInspections(ctx, QualityInspectionQuery{Scope: " work_order ", Result: " pass ", Limit: 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || repo.qualityQuery.Limit != 200 || repo.qualityQuery.Scope != "work_order" || repo.qualityQuery.Result != "pass" {
		t.Fatalf("quality list rows=%+v query=%+v", rows, repo.qualityQuery)
	}
}

func TestServiceOwnsWIPReservationAndAcceptanceUseCases(t *testing.T) {
	repo := &fakeFlowRepo{
		reservationRows: []WIPReservationRow{{
			ID:                 9,
			WorkOrderNo:        "WO-0000000020",
			RunningItemID:      20,
			MaterialName:       "孟连水洗5T批次",
			ReservedG:          60000,
			ConsumedG:          20000,
			RemainingReservedG: 40000,
			WIPG:               90000,
			AvailableG:         50000,
			Status:             "reserved",
		}},
		acceptanceRows: []AcceptanceSmokeRow{{Code: "work_orders", Title: "生产工单", Status: "ok", Count: 1}},
	}
	svc := NewService(repo)
	ctx := context.Background()

	reservations, err := svc.ListWIPReservations(ctx, WIPReservationQuery{Status: " reserved ", WorkOrderNo: " WO-0000000020 ", Limit: 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(reservations.Rows) != 1 || reservations.TotalRemainingG != 40000 {
		t.Fatalf("reservations = %+v", reservations)
	}
	if repo.reservationQuery.Status != "reserved" || repo.reservationQuery.WorkOrderNo != "WO-0000000020" || repo.reservationQuery.Limit != 200 {
		t.Fatalf("reservation query not normalized: %+v", repo.reservationQuery)
	}

	adjusted, err := svc.AdjustWIPReservation(ctx, WIPReservationAdjustCommand{ReservationID: 9, ReservedG: 50000, Operator: " 测试员 ", Note: " 调整 "})
	if err != nil {
		t.Fatal(err)
	}
	if adjusted.RemainingReservedG != 40000 || repo.adjustReservation.Operator != "测试员" || repo.adjustReservation.Note != "调整" {
		t.Fatalf("adjusted=%+v command=%+v", adjusted, repo.adjustReservation)
	}

	released, err := svc.ReleaseWIPReservations(ctx, WIPReservationReleaseCommand{RunningItemID: 20, WorkOrderNo: " WO-0000000020 ", Operator: " 测试员 ", Note: " 清理 "})
	if err != nil {
		t.Fatal(err)
	}
	if released.ReleasedCount != 2 || repo.releaseReservation.WorkOrderNo != "WO-0000000020" || repo.releaseReservation.Operator != "测试员" {
		t.Fatalf("release result=%+v command=%+v", released, repo.releaseReservation)
	}

	smoke, err := svc.AcceptanceSmoke(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(smoke.Rows) != 1 || smoke.Rows[0].Code != "work_orders" {
		t.Fatalf("AcceptanceSmoke() = %+v", smoke)
	}
}
