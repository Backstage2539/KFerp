package production

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

type fakeFlowRepo struct {
	start          StartCommand
	startNeeds     []StartNeed
	startExecution StartExecutionCommand
	finish         FinishCommand
	cancel         CancelCommand

	createPlan          CreateProductionPlanCommand
	submitPlan          SubmitProductionPlanCommand
	submitPlans         []SubmitProductionPlanCommand
	startWorkOrder      WorkOrderStartCommand
	completeWorkOrder   WorkOrderCompleteCommand
	productionPlanQuery ProductionPlanQuery
	productionPlan      ProductionPlanDetail
	submittedPlan       ProductionPlanSubmitResult
	submitPlanByID      map[int64]ProductionPlanSubmitResult
	submitPlanErrByID   map[int64]error
	workOrderStarted    WorkOrderStartResult
	workOrderCompleted  WorkOrderCompleteResult
	scheduleAssignment  ScheduleAssignmentCommand
	capacityCalendar    CapacityCalendarCommand
	scheduleQuery       ScheduleBoardQuery
	stockEntry          StockEntryCommand
	stockEntryQuery     StockEntryQuery
	stockEntryID        int64
	jobCardAction       JobCardActionCommand
	jobCardActionResult JobCardActionResult

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

func (r *fakeFlowRepo) CreateProductionPlan(ctx context.Context, cmd CreateProductionPlanCommand) (ProductionPlanDetail, error) {
	r.createPlan = cmd
	if r.productionPlan.ID == 0 {
		r.productionPlan = ProductionPlanDetail{
			ID:     41,
			PlanNo: "PP-0000000041",
			Status: "draft",
			Items:  []ProductionPlanItem{{ID: 51, ProductID: 1, ProductName: "计划拼配", SpecG: 227, PlannedG: 600, GapG: 454, OrderNos: "SO-PLAN-1"}},
		}
	}
	return r.productionPlan, nil
}

func (r *fakeFlowRepo) ListProductionPlans(ctx context.Context, query ProductionPlanQuery) ([]ProductionPlanRow, error) {
	r.productionPlanQuery = query
	return []ProductionPlanRow{{ID: r.productionPlan.ID, PlanNo: r.productionPlan.PlanNo, Status: r.productionPlan.Status}}, nil
}

func (r *fakeFlowRepo) GetProductionPlan(ctx context.Context, id int64) (ProductionPlanDetail, error) {
	return r.productionPlan, nil
}

func (r *fakeFlowRepo) SaveProductionPlanOperationSplits(ctx context.Context, cmd SaveProductionPlanOperationSplitsCommand) ([]ProductionPlanOperationSplit, error) {
	return cmd.Items, nil
}

func (r *fakeFlowRepo) SubmitProductionPlan(ctx context.Context, cmd SubmitProductionPlanCommand) (ProductionPlanSubmitResult, error) {
	r.submitPlan = cmd
	r.submitPlans = append(r.submitPlans, cmd)
	if err := r.submitPlanErrByID[cmd.ID]; err != nil {
		return ProductionPlanSubmitResult{}, err
	}
	if res, ok := r.submitPlanByID[cmd.ID]; ok {
		return res, nil
	}
	if r.submittedPlan.Plan.ID == 0 {
		r.submittedPlan = ProductionPlanSubmitResult{
			Plan: ProductionPlanDetail{ID: cmd.ID, PlanNo: "PP-0000000041", Status: "submitted"},
			WorkOrders: []WorkOrderRow{{
				ID: 88, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "released", RunningItemID: 0, ProductID: 1, ProductName: "计划拼配", SpecG: 227, PlannedG: 600,
			}},
			JobCards: []JobCardRow{{ID: 91, WorkOrderID: 88, SequenceNo: 1, Operation: "烘焙", Workstation: "烘焙机", Status: "pending"}},
		}
	}
	return r.submittedPlan, nil
}

func (r *fakeFlowRepo) StartWorkOrder(ctx context.Context, cmd WorkOrderStartCommand) (WorkOrderStartResult, error) {
	r.startWorkOrder = cmd
	if r.workOrderStarted.WorkOrder.ID == 0 {
		r.workOrderStarted = WorkOrderStartResult{
			BatchID:       "BATCH-WO-88",
			RunningItemID: 99,
			WorkOrder:     WorkOrderRow{ID: cmd.ID, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "running", RunningItemID: 99},
		}
	}
	return r.workOrderStarted, nil
}

func (r *fakeFlowRepo) CompleteWorkOrder(ctx context.Context, cmd WorkOrderCompleteCommand) (WorkOrderCompleteResult, error) {
	r.completeWorkOrder = cmd
	if r.workOrderCompleted.WorkOrder.ID == 0 {
		r.workOrderCompleted = WorkOrderCompleteResult{
			WorkOrder:    WorkOrderRow{ID: cmd.ID, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "completed", RunningItemID: 99, ActualCost: 48.75},
			StockEntries: []StockEntryRow{{ID: 7, EntryNo: "SE-0000000007", EntryType: "finished_receipt", WorkOrderID: cmd.ID, RunningItemID: 99, Status: "submitted"}},
			Cost:         BatchCostRow{RunningItemID: 99, MaterialCost: 36.25, OperationCost: 12.5, TotalCost: 48.75},
		}
	}
	return r.workOrderCompleted, nil
}

func (r *fakeFlowRepo) SaveScheduleAssignment(ctx context.Context, cmd ScheduleAssignmentCommand) (ScheduleAssignmentResult, error) {
	r.scheduleAssignment = cmd
	return ScheduleAssignmentResult{WorkOrder: WorkOrderRow{ID: cmd.WorkOrderID, PlannedStartAt: cmd.PlannedStartAt, PlannedEndAt: cmd.PlannedEndAt}, JobCard: JobCardRow{ID: cmd.JobCardID, WorkOrderID: cmd.WorkOrderID}}, nil
}

func (r *fakeFlowRepo) SaveCapacityCalendar(ctx context.Context, cmd CapacityCalendarCommand) (CapacityCalendarRow, error) {
	r.capacityCalendar = cmd
	return CapacityCalendarRow{ID: cmd.ID, WorkCenter: cmd.WorkCenter, WorkDate: cmd.WorkDate, ShiftCode: cmd.ShiftCode, AvailableMinutes: cmd.AvailableMinutes, DowntimeMinutes: cmd.DowntimeMinutes}, nil
}

func (r *fakeFlowRepo) ScheduleBoard(ctx context.Context, query ScheduleBoardQuery) (ScheduleBoardResult, error) {
	r.scheduleQuery = query
	return ScheduleBoardResult{}, nil
}

func (r *fakeFlowRepo) CreateStockEntry(ctx context.Context, cmd StockEntryCommand) (StockEntryDetail, error) {
	r.stockEntry = cmd
	items := make([]StockEntryItemRow, 0, len(cmd.Items))
	for i, item := range cmd.Items {
		items = append(items, StockEntryItemRow{
			ID:            int64(i + 1),
			MaterialID:    item.MaterialID,
			ProductID:     item.ProductID,
			ItemType:      item.ItemType,
			ItemName:      item.ItemName,
			FromWarehouse: item.FromWarehouse,
			ToWarehouse:   item.ToWarehouse,
			QtyG:          item.QtyG,
			QtyUnits:      item.QtyUnits,
		})
	}
	return StockEntryDetail{ID: 7, EntryNo: "SE-0000000007", EntryType: cmd.EntryType, Status: "submitted", WorkOrderID: cmd.WorkOrderID, JobCardID: cmd.JobCardID, RunningItemID: cmd.RunningItemID, Operator: cmd.Operator, Note: cmd.Note, Items: items}, nil
}

func (r *fakeFlowRepo) ListStockEntries(ctx context.Context, query StockEntryQuery) ([]StockEntryRow, error) {
	r.stockEntryQuery = query
	return []StockEntryRow{{ID: 7, EntryNo: "SE-0000000007", EntryType: query.EntryType, WorkOrderID: query.WorkOrderID, Status: "submitted"}}, nil
}

func (r *fakeFlowRepo) GetStockEntry(ctx context.Context, id int64) (StockEntryDetail, error) {
	r.stockEntryID = id
	return StockEntryDetail{ID: id, EntryNo: "SE-0000000007", EntryType: "material_issue_to_wip", Status: "submitted", Items: []StockEntryItemRow{{ID: 1, MaterialID: 10, ItemType: "material", QtyG: 60000}}}, nil
}

func (r *fakeFlowRepo) TransitionJobCard(ctx context.Context, cmd JobCardActionCommand) (JobCardActionResult, error) {
	r.jobCardAction = cmd
	if r.jobCardActionResult.JobCard.ID != 0 && r.jobCardActionResult.JobCard.ID != cmd.ID {
		return r.jobCardActionResult, nil
	}
	return JobCardActionResult{
		JobCard:   JobCardRow{ID: cmd.ID, WorkOrderID: 88, Status: actionResultStatus(cmd.Action), ActualInputQty: cmd.ActualInputQty, ActualOutputQty: cmd.ActualOutputQty, ActualLossQty: cmd.ActualLossQty, ActualLossRate: cmd.ActualLossRate, ExceptionReason: cmd.ExceptionReason, Operator: cmd.Operator},
		WorkOrder: WorkOrderRow{ID: 88, Status: "running"},
	}, nil
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

func (r *fakeFlowRepo) MRPSuggestions(ctx context.Context, query MRPSuggestionQuery) (MRPSuggestionResult, error) {
	return MRPSuggestionResult{}, nil
}

func (r *fakeFlowRepo) ProductionTraceAnalytics(ctx context.Context, query ProductionTraceAnalyticsQuery) (ProductionTraceAnalyticsResult, error) {
	return ProductionTraceAnalyticsResult{}, nil
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

func TestServiceOwnsFormalProductionPlanWorkOrderLifecycle(t *testing.T) {
	repo := &fakeFlowRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	plan, err := svc.CreateProductionPlan(ctx, CreateProductionPlanCommand{
		From:       "2026-06-11",
		To:         "2026-06-12",
		Selected:   map[string]bool{"1-227": true},
		InputByKey: map[string]int64{"1-227": 600},
		Operator:   "计划员",
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != "draft" || len(plan.Items) != 1 {
		t.Fatalf("CreateProductionPlan() = %+v, want draft plan with one item", plan)
	}
	if repo.createPlan.Operator != "计划员" || !repo.createPlan.Selected["1-227"] || repo.createPlan.InputByKey["1-227"] != 600 {
		t.Fatalf("create plan command = %+v", repo.createPlan)
	}

	submitted, err := svc.SubmitProductionPlan(ctx, SubmitProductionPlanCommand{ID: plan.ID, Operator: "计划员"})
	if err != nil {
		t.Fatal(err)
	}
	if submitted.Plan.Status != "submitted" || len(submitted.WorkOrders) != 1 || submitted.WorkOrders[0].Status != "released" {
		t.Fatalf("SubmitProductionPlan() = %+v, want submitted plan and released work order", submitted)
	}
	if len(submitted.JobCards) != 1 || submitted.JobCards[0].Status != "pending" {
		t.Fatalf("SubmitProductionPlan job cards = %+v, want pending job cards", submitted.JobCards)
	}

	started, err := svc.StartWorkOrder(ctx, WorkOrderStartCommand{ID: submitted.WorkOrders[0].ID, Operator: "开工员"})
	if err != nil {
		t.Fatal(err)
	}
	if started.RunningItemID != 99 || started.WorkOrder.Status != "running" || repo.startWorkOrder.Operator != "开工员" {
		t.Fatalf("StartWorkOrder() = %+v command=%+v, want running work order", started, repo.startWorkOrder)
	}
}

func TestCreateProductionPlanDefaultsSelectedNeedInputWhenPlanRowHasNoEditableRoastInput(t *testing.T) {
	repo := &fakeFlowRepo{
		productionPlan: ProductionPlanDetail{
			ID:     42,
			PlanNo: "PP-0000000042",
			Status: "draft",
			Items:  []ProductionPlanItem{{ID: 52, ProductID: 539, ProductName: "PR439-20260606182321 工厂量单商品", SpecG: 454, PlannedG: 554, GapG: 454, OrderNos: "SO-PR439"}},
		},
	}
	svc := NewService(repo)

	plan, err := svc.CreateProductionPlan(context.Background(), CreateProductionPlanCommand{
		Selected:   map[string]bool{"539-454": true},
		InputByKey: map[string]int64{},
		Operator:   "计划员",
	})
	if err != nil {
		t.Fatalf("selected formal production plan row without explicit input should use default input: %v", err)
	}
	if plan.ID != 42 || len(plan.Items) != 1 || plan.Items[0].ProductID != 539 {
		t.Fatalf("CreateProductionPlan() = %+v, want PR439 draft plan", plan)
	}
	if !repo.createPlan.Selected["539-454"] || repo.createPlan.InputByKey == nil || repo.createPlan.Operator != "计划员" {
		t.Fatalf("repo.CreateProductionPlan command = %+v", repo.createPlan)
	}
}

func TestListProductionPlansNormalizesFiltersAndDefaultLimit(t *testing.T) {
	repo := &fakeFlowRepo{}
	svc := NewService(repo)

	if _, err := svc.ListProductionPlans(context.Background(), ProductionPlanQuery{
		Status:    " submitted ",
		TimeField: " submitted_at ",
		From:      " 2026-06-01 ",
		To:        " 2026-06-11 ",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.productionPlanQuery.Status != "submitted" ||
		repo.productionPlanQuery.TimeField != "submitted_at" ||
		repo.productionPlanQuery.From != "2026-06-01" ||
		repo.productionPlanQuery.To != "2026-06-11" ||
		repo.productionPlanQuery.Limit != 50 {
		t.Fatalf("production plan query = %+v, want normalized submitted_at date filter with default limit 50", repo.productionPlanQuery)
	}

	if _, err := svc.ListProductionPlans(context.Background(), ProductionPlanQuery{TimeField: "updated_at", Limit: 999}); err != nil {
		t.Fatal(err)
	}
	if repo.productionPlanQuery.TimeField != "created_at" || repo.productionPlanQuery.Limit != 500 {
		t.Fatalf("fallback production plan query = %+v, want created_at with capped limit 500", repo.productionPlanQuery)
	}
}

func TestSubmitProductionPlansBatchesDraftPlansAndReportsFailures(t *testing.T) {
	repo := &fakeFlowRepo{
		submitPlanByID: map[int64]ProductionPlanSubmitResult{
			41: {
				Plan:       ProductionPlanDetail{ID: 41, PlanNo: "PP-0000000041", Status: "submitted"},
				WorkOrders: []WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "released"}},
				JobCards:   []JobCardRow{{ID: 91, WorkOrderID: 88, Status: "pending"}},
			},
		},
		submitPlanErrByID: map[int64]error{
			42: fmt.Errorf("production plan must be draft to submit"),
		},
	}
	svc := NewService(repo)

	result, err := svc.SubmitProductionPlans(context.Background(), SubmitProductionPlansCommand{
		IDs:      []int64{41, 42, 41},
		Operator: " 计划员 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Success) != 1 || result.Success[0].Plan.ID != 41 || result.WorkOrderCount != 1 || result.JobCardCount != 1 {
		t.Fatalf("batch submit result = %+v, want one submitted plan and counts", result)
	}
	if len(result.Failed) != 2 || result.Failed[0].ID != 42 || result.Failed[1].ID != 41 {
		t.Fatalf("batch submit failures = %+v, want non-draft and duplicate detail", result.Failed)
	}
	if len(repo.submitPlans) != 2 || repo.submitPlans[0].ID != 41 || repo.submitPlans[1].ID != 42 || repo.submitPlans[0].Operator != "计划员" {
		t.Fatalf("submit commands = %+v, want unique ids with trimmed operator", repo.submitPlans)
	}
}

func TestServiceRejectsInvalidProductionPlanAndWorkOrderCommands(t *testing.T) {
	svc := NewService(&fakeFlowRepo{})
	ctx := context.Background()

	if _, err := svc.CreateProductionPlan(ctx, CreateProductionPlanCommand{Selected: map[string]bool{}, InputByKey: map[string]int64{}, Operator: "计划员"}); err == nil {
		t.Fatal("CreateProductionPlan should reject empty selection")
	}
	if _, err := svc.SubmitProductionPlan(ctx, SubmitProductionPlanCommand{ID: 0, Operator: "计划员"}); err == nil {
		t.Fatal("SubmitProductionPlan should reject empty id")
	}
	if _, err := svc.SubmitProductionPlans(ctx, SubmitProductionPlansCommand{IDs: []int64{}, Operator: "计划员"}); err == nil {
		t.Fatal("SubmitProductionPlans should reject empty ids")
	}
	if _, err := svc.StartWorkOrder(ctx, WorkOrderStartCommand{ID: 0, Operator: "开工员"}); err == nil {
		t.Fatal("StartWorkOrder should reject empty id")
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

func TestServiceOwnsManufacturingPhase2StockEntriesAndExecutionActions(t *testing.T) {
	repo := &fakeFlowRepo{}
	svc := NewService(repo)
	ctx := context.Background()

	entry, err := svc.CreateStockEntry(ctx, StockEntryCommand{
		EntryType:   "  material_issue_to_wip ",
		WorkOrderID: 88,
		Operator:    " 仓管 ",
		Note:        " 二期领料 ",
		Items: []StockEntryItemCommand{{
			MaterialID:    10,
			ItemType:      "material",
			ItemName:      "卡蒂姆水洗",
			FromWarehouse: "raw_materials",
			ToWarehouse:   "wip",
			QtyG:          60000,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.EntryNo == "" || repo.stockEntry.EntryType != "material_issue_to_wip" || repo.stockEntry.Operator != "仓管" || len(repo.stockEntry.Items) != 1 {
		t.Fatalf("CreateStockEntry() entry=%+v command=%+v", entry, repo.stockEntry)
	}
	if repo.stockEntry.Items[0].FromWarehouse != "raw_materials" || repo.stockEntry.Items[0].ToWarehouse != "wip" {
		t.Fatalf("stock entry warehouses = %+v", repo.stockEntry.Items[0])
	}

	rows, err := svc.ListStockEntries(ctx, StockEntryQuery{EntryType: " material_issue_to_wip ", WorkOrderID: 88, Limit: 999})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || repo.stockEntryQuery.Limit != 200 || repo.stockEntryQuery.EntryType != "material_issue_to_wip" || repo.stockEntryQuery.WorkOrderID != 88 {
		t.Fatalf("ListStockEntries rows=%+v query=%+v", rows, repo.stockEntryQuery)
	}
	if _, err := svc.GetStockEntry(ctx, 7); err != nil || repo.stockEntryID != 7 {
		t.Fatalf("GetStockEntry err=%v id=%d", err, repo.stockEntryID)
	}

	started, err := svc.StartJobCard(ctx, JobCardActionCommand{ID: 91, Operator: " 操作员 "})
	if err != nil {
		t.Fatal(err)
	}
	if started.JobCard.Status != "running" || repo.jobCardAction.Action != "start" || repo.jobCardAction.Operator != "操作员" {
		t.Fatalf("StartJobCard result=%+v command=%+v", started, repo.jobCardAction)
	}

	completed, err := svc.CompleteJobCard(ctx, JobCardActionCommand{ID: 91, Operator: " 操作员 ", ActualInputQty: 600, ActualOutputQty: 540, ExceptionReason: " 正常损耗 "})
	if err != nil {
		t.Fatal(err)
	}
	if completed.JobCard.Status != "completed" || repo.jobCardAction.Action != "complete" || repo.jobCardAction.ActualLossQty != 60 || repo.jobCardAction.ActualLossRate != 0.1 || repo.jobCardAction.ExceptionReason != "正常损耗" {
		t.Fatalf("CompleteJobCard result=%+v command=%+v", completed, repo.jobCardAction)
	}

	closed, err := svc.CompleteWorkOrder(ctx, WorkOrderCompleteCommand{ID: 88, FinishedUnits: 2, FinishedLooseG: 10, ConsumedInputG: 600, Warehouse: " finished_goods ", Operator: " 主管 "})
	if err != nil {
		t.Fatal(err)
	}
	if closed.WorkOrder.Status != "completed" || repo.completeWorkOrder.Warehouse != "finished_goods" || repo.completeWorkOrder.Operator != "主管" || len(closed.StockEntries) == 0 || closed.Cost.TotalCost <= 0 {
		t.Fatalf("CompleteWorkOrder result=%+v command=%+v", closed, repo.completeWorkOrder)
	}
}

func TestServiceRejectsInvalidManufacturingPhase2ExecutionCommands(t *testing.T) {
	svc := NewService(&fakeFlowRepo{})
	ctx := context.Background()
	if _, err := svc.CreateStockEntry(ctx, StockEntryCommand{EntryType: "unknown", Operator: "测试", Items: []StockEntryItemCommand{{MaterialID: 1, QtyG: 1}}}); err == nil {
		t.Fatal("CreateStockEntry should reject unknown entry type")
	}
	if _, err := svc.CreateStockEntry(ctx, StockEntryCommand{EntryType: "material_issue_to_wip", Operator: "测试"}); err == nil {
		t.Fatal("CreateStockEntry should reject empty items")
	}
	if _, err := svc.StartJobCard(ctx, JobCardActionCommand{}); err == nil {
		t.Fatal("StartJobCard should reject empty id")
	}
	if _, err := svc.PauseJobCard(ctx, JobCardActionCommand{ID: 91}); err == nil {
		t.Fatal("PauseJobCard should require operator")
	}
	if _, err := svc.CompleteWorkOrder(ctx, WorkOrderCompleteCommand{ID: 88, Operator: "主管"}); err == nil {
		t.Fatal("CompleteWorkOrder should require finished output")
	}
}

func actionResultStatus(action string) string {
	switch action {
	case "complete":
		return "completed"
	case "pause":
		return "paused"
	default:
		return "running"
	}
}
