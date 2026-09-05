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
	cancelPlan          CancelProductionPlanCommand
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
	stockEntryRows      []StockEntryRow
	stockEntryID        int64
	jobCardAction       JobCardActionCommand
	jobCardActionResult JobCardActionResult
	cancelWorkOrder     WorkOrderCancelCommand
	cancelledWorkOrder  WorkOrderRow
	ledgerQuery         WorkOrderLedgerQuery
	ledgerRows          []WorkOrderLedgerEntryRow

	materialPlanQuery   MaterialPlanQuery
	materialPlanResult  MaterialPlanResult
	qualityCommand      QualityInspectionCommand
	qualityQuery        QualityInspectionQuery
	qualityRows         []QualityInspectionRow
	productionLogsQuery ProductionLogsQuery
	productionLogs      ProductionLogsResult
	reservationQuery    WIPReservationQuery
	reservationRows     []WIPReservationRow
	adjustReservation   WIPReservationAdjustCommand
	releaseReservation  WIPReservationReleaseCommand
	acceptanceRows      []AcceptanceSmokeRow
	workOrders          []WorkOrderRow
	jobCards            []JobCardRow
	workOrderQuery      WorkOrderQuery
	jobCardQuery        JobCardQuery
	batchCostQuery      BatchCostQuery
	batchCosts          []BatchCostRow
	wipCoverage         ProductionWIPStatus
	stockDraft          *StockEntryCommand
	stockDraftWorkOrder int64
	stockDraftAction    string
	stockDraftID        int64
	usesFrozenSources   bool
	frozenSourceItems   []StockEntryItemCommand
	frozenSourceErr     error
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

func (r *fakeFlowRepo) PreviewProductionPlanOperationSplits(ctx context.Context, cmd PreviewProductionPlanOperationSplitsCommand) (ProductionPlanOperationSplitPreview, error) {
	return ProductionPlanOperationSplitPreview{}, nil
}

func (r *fakeFlowRepo) SaveWorkOrderOperationSplits(ctx context.Context, cmd SaveWorkOrderOperationSplitsCommand) (WorkOrderOperationSplitsResult, error) {
	return WorkOrderOperationSplitsResult{
		WorkOrder: WorkOrderRow{ID: cmd.ID, Status: "released"},
		JobCards:  []JobCardRow{{ID: 91, WorkOrderID: cmd.ID, Status: "pending"}},
	}, nil
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

func (r *fakeFlowRepo) CancelProductionPlan(ctx context.Context, cmd CancelProductionPlanCommand) (ProductionPlanDetail, error) {
	r.cancelPlan = cmd
	r.productionPlan.ID = cmd.ID
	r.productionPlan.Status = "cancelled"
	return r.productionPlan, nil
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

func (r *fakeFlowRepo) CancelWorkOrder(ctx context.Context, cmd WorkOrderCancelCommand) (WorkOrderRow, error) {
	r.cancelWorkOrder = cmd
	if r.cancelledWorkOrder.ID == 0 {
		r.cancelledWorkOrder = WorkOrderRow{ID: cmd.ID, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "cancelled"}
	}
	return r.cancelledWorkOrder, nil
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
	if len(r.stockEntryRows) > 0 {
		return r.stockEntryRows, nil
	}
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
	r.productionLogsQuery = query
	return r.productionLogs, nil
}
func (r *fakeFlowRepo) ListWorkOrders(ctx context.Context, query WorkOrderQuery) ([]WorkOrderRow, error) {
	r.workOrderQuery = query
	return r.workOrders, nil
}
func (r *fakeFlowRepo) ListJobCards(ctx context.Context, query JobCardQuery) ([]JobCardRow, error) {
	r.jobCardQuery = query
	return r.jobCards, nil
}
func (r *fakeFlowRepo) UpdateJobCardActuals(ctx context.Context, cmd JobCardActualsCommand) error {
	return nil
}
func (r *fakeFlowRepo) ListBatchCosts(ctx context.Context, query BatchCostQuery) ([]BatchCostRow, error) {
	r.batchCostQuery = query
	return r.batchCosts, nil
}

func (r *fakeFlowRepo) ListWorkOrderLedgerEntries(ctx context.Context, query WorkOrderLedgerQuery) ([]WorkOrderLedgerEntryRow, error) {
	r.ledgerQuery = query
	return r.ledgerRows, nil
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

func (r *fakeFlowRepo) GetWorkOrderWIPCoverage(ctx context.Context, workOrderID int64) (ProductionWIPStatus, error) {
	if r.wipCoverage.Status == "" && len(r.wipCoverage.Materials) == 0 {
		if len(r.reservationRows) == 0 {
			return ProductionWIPStatus{DataComplete: true, Status: "ok", Materials: []WIPReservationRow{}}, nil
		}
		return buildProductionWIPStatus(r.reservationRows), nil
	}
	return r.wipCoverage, nil
}

func (r *fakeFlowRepo) GetWorkOrderStockDocumentDraft(ctx context.Context, workOrderID int64, action string, stockDocumentID int64) (*StockEntryCommand, error) {
	r.stockDraftWorkOrder = workOrderID
	r.stockDraftAction = action
	r.stockDraftID = stockDocumentID
	if stockDocumentID > 0 && (r.stockDraft == nil || r.stockDraft.ID != stockDocumentID) {
		return nil, nil
	}
	return r.stockDraft, nil
}

func (r *fakeFlowRepo) GetWorkOrderFrozenSourceIssueItems(context.Context, int64) (bool, []StockEntryItemCommand, error) {
	return r.usesFrozenSources, append([]StockEntryItemCommand(nil), r.frozenSourceItems...), r.frozenSourceErr
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

func TestServiceCancelsProductionPlanDraftWithTrimmedAuditContext(t *testing.T) {
	repo := &fakeFlowRepo{
		productionPlan: ProductionPlanDetail{
			ID:     41,
			PlanNo: "PP-0000000041",
			Status: "draft",
		},
	}
	svc := NewService(repo)

	cancelled, err := svc.CancelProductionPlan(context.Background(), CancelProductionPlanCommand{
		ID:       41,
		Operator: " 计划员 ",
		Note:     " 订单调整 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.ID != 41 || cancelled.Status != "cancelled" {
		t.Fatalf("CancelProductionPlan() = %+v, want cancelled plan", cancelled)
	}
	if repo.cancelPlan.ID != 41 || repo.cancelPlan.Operator != "计划员" || repo.cancelPlan.Note != "订单调整" {
		t.Fatalf("cancel plan command = %+v, want trimmed audit context", repo.cancelPlan)
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
	if _, err := svc.CancelProductionPlan(ctx, CancelProductionPlanCommand{ID: 0, Operator: "计划员"}); err == nil {
		t.Fatal("CancelProductionPlan should reject empty id")
	}
	if _, err := svc.SubmitProductionPlans(ctx, SubmitProductionPlansCommand{IDs: []int64{}, Operator: "计划员"}); err == nil {
		t.Fatal("SubmitProductionPlans should reject empty ids")
	}
	if _, err := svc.StartWorkOrder(ctx, WorkOrderStartCommand{ID: 0, Operator: "开工员"}); err == nil {
		t.Fatal("StartWorkOrder should reject empty id")
	}
	if _, err := svc.SaveWorkOrderOperationSplits(ctx, SaveWorkOrderOperationSplitsCommand{ID: 0, Items: []ProductionPlanOperationSplit{{WorkstationCapacityID: 8, PlannedQty: 90}}}); err == nil {
		t.Fatal("SaveWorkOrderOperationSplits should reject empty id")
	}
	if _, err := svc.SaveWorkOrderOperationSplits(ctx, SaveWorkOrderOperationSplitsCommand{ID: 88, Items: []ProductionPlanOperationSplit{{PlannedQty: 90}}}); err == nil {
		t.Fatal("SaveWorkOrderOperationSplits should require workstation capacity")
	}
	if _, err := svc.SaveWorkOrderOperationSplits(ctx, SaveWorkOrderOperationSplitsCommand{ID: 88, Items: []ProductionPlanOperationSplit{{WorkstationCapacityID: 8}}}); err == nil {
		t.Fatal("SaveWorkOrderOperationSplits should require planned quantity")
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

func TestServiceOwnsWorkOrderInventoryControlWithStockDocumentPurpose(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{
			ID:            88,
			WorkOrderNo:   "WO-PR497-001",
			RunningItemID: 99,
			ProductName:   "桂花乌龙",
			SpecG:         227,
			PlannedG:      45400,
			Status:        "running",
		}},
		jobCards: []JobCardRow{{
			ID:          91,
			WorkOrderID: 88,
			WorkOrderNo: "WO-PR497-001",
			Operation:   "烘焙",
			Workstation: "布勒 18kg",
			Status:      "running",
		}},
		reservationRows: []WIPReservationRow{{
			ID:                 11,
			WorkOrderID:        88,
			WorkOrderNo:        "WO-PR497-001",
			RunningItemID:      99,
			MaterialID:         10,
			MaterialName:       "孟连水洗",
			RequiredG:          60000,
			ReservedG:          60000,
			RemainingReservedG: 45000,
			Status:             "reserved",
		}},
		stockEntryRows: []StockEntryRow{{
			ID:            7,
			EntryNo:       "SE-0000000007",
			EntryType:     "material_issue_to_wip",
			Purpose:       "material_transfer_for_manufacture",
			WorkOrderID:   88,
			RunningItemID: 99,
			Status:        "submitted",
		}},
		ledgerRows: []WorkOrderLedgerEntryRow{{
			ID:            21,
			StockEntryID:  7,
			EntryNo:       "SE-0000000007",
			Purpose:       "material_transfer_for_manufacture",
			ItemType:      "material",
			ItemID:        10,
			ItemName:      "孟连水洗",
			Warehouse:     "wip",
			QtyChangeG:    60000,
			QtyAfterG:     60000,
			SourceDocType: "stock_entry",
			SourceDocID:   7,
		}},
		productionLogs: ProductionLogsResult{Rows: []ProductionLogRow{{
			ID:             31,
			BatchID:        "BATCH-WO-88",
			InputG:         60000,
			FinishedTotalG: 45400,
		}}},
		batchCosts: []BatchCostRow{{RunningItemID: 99, BatchID: "BATCH-WO-88", MaterialCost: 36.25, OperationCost: 12.5, TotalCost: 48.75}},
	}
	svc := NewService(repo)
	ctx := context.Background()

	entry, err := svc.CreateStockEntry(ctx, StockEntryCommand{
		Purpose:     " material_transfer_for_manufacture ",
		WorkOrderID: 88,
		Operator:    " 仓管 ",
		Items: []StockEntryItemCommand{{
			MaterialID: 10,
			ItemName:   "孟连水洗",
			QtyG:       60000,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if entry.Purpose != "material_transfer_for_manufacture" || repo.stockEntry.EntryType != "material_issue_to_wip" || repo.stockEntry.Purpose != "material_transfer_for_manufacture" {
		t.Fatalf("CreateStockEntry purpose mapping entry=%+v command=%+v", entry, repo.stockEntry)
	}
	if repo.stockEntry.SourceType != "work_order" || repo.stockEntry.SourceID != 88 || repo.stockEntry.Items[0].FromWarehouse != "raw_materials" || repo.stockEntry.Items[0].ToWarehouse != "wip" {
		t.Fatalf("CreateStockEntry work-order defaults = %+v", repo.stockEntry)
	}

	rows, err := svc.ListStockEntries(ctx, StockEntryQuery{Purpose: " material_transfer_for_manufacture ", WorkOrderID: 88})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Purpose != "material_transfer_for_manufacture" || repo.stockEntryQuery.EntryType != "material_issue_to_wip" || repo.stockEntryQuery.Purpose != "material_transfer_for_manufacture" {
		t.Fatalf("ListStockEntries purpose rows=%+v query=%+v", rows, repo.stockEntryQuery)
	}

	detail, err := svc.GetWorkOrderDetail(ctx, 88)
	if err != nil {
		t.Fatal(err)
	}
	if detail.WorkOrder.ID != 88 || detail.WorkOrder.RunningItemID != 99 || len(detail.Materials) != 1 || len(detail.JobCards) != 1 || len(detail.StockDocuments) != 1 || len(detail.StockEntries) != 1 || len(detail.LedgerEntries) != 1 || len(detail.ProductionLogs.Rows) != 1 || detail.CostSummary.TotalCost != 48.75 {
		t.Fatalf("GetWorkOrderDetail() = %+v", detail)
	}
	if repo.workOrderQuery.ID != 88 || repo.jobCardQuery.WorkOrderID != 88 || repo.reservationQuery.WorkOrderNo != "WO-PR497-001" || repo.stockEntryQuery.WorkOrderID != 88 || repo.ledgerQuery.WorkOrderID != 88 || repo.ledgerQuery.RunningItemID != 99 || repo.productionLogsQuery.RunningItemID != 99 || repo.batchCostQuery.RunningItemID != 99 {
		t.Fatalf("detail queries workOrder=%+v jobCard=%+v reservations=%+v stock=%+v ledger=%+v logs=%+v costs=%+v", repo.workOrderQuery, repo.jobCardQuery, repo.reservationQuery, repo.stockEntryQuery, repo.ledgerQuery, repo.productionLogsQuery, repo.batchCostQuery)
	}

	cancelled, err := svc.CancelWorkOrder(ctx, WorkOrderCancelCommand{ID: 88, Operator: " 主管 ", Note: " 临时取消 "})
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != "cancelled" || repo.cancelWorkOrder.ID != 88 || repo.cancelWorkOrder.Operator != "主管" || repo.cancelWorkOrder.Note != "临时取消" {
		t.Fatalf("CancelWorkOrder result=%+v command=%+v", cancelled, repo.cancelWorkOrder)
	}
}

func TestWorkOrderExecutionHubReadModelAndTraceTimeline(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{
			ID:                   88,
			WorkOrderNo:          "WO-HUB-001",
			RunningItemID:        99,
			ProductionPlanID:     41,
			ProductID:            5,
			ProductName:          "桂花乌龙",
			SpecG:                227,
			PlannedG:             45400,
			PlannedOutputG:       45400,
			PlannedUnits:         200,
			Status:               "running",
			BatchID:              "BATCH-WO-88",
			BomVersionID:         17,
			ProcessSnapshotJSON:  `{"route_name":"标准包装路线","operations":[{"seq":1,"operation":"包装"},{"seq":2,"operation":"贴标"}]}`,
			OperationSummaryJSON: `[{"sequence_no":1,"operation":"包装","workstation":"包装工位A","status":"completed","planned_minutes":40},{"sequence_no":2,"operation":"贴标","workstation":"包装工位B","status":"pending","planned_minutes":20}]`,
			AssignedTo:           "生产主管",
			WorkCenter:           "包装线",
			Priority:             7,
			CreatedAt:            "2026-06-17 09:00",
		}},
		jobCards: []JobCardRow{{
			ID:             91,
			WorkOrderID:    88,
			WorkOrderNo:    "WO-HUB-001",
			Operation:      "包装",
			Workstation:    "包装工位A",
			WorkCenter:     "包装线",
			Status:         "completed",
			AssignedTo:     "阿强",
			CompletedAt:    "2026-06-17 10:00",
			PlannedMinutes: 40,
		}, {
			ID:              92,
			WorkOrderID:     88,
			WorkOrderNo:     "WO-HUB-001",
			Operation:       "贴标",
			Workstation:     "",
			WorkCenter:      "",
			Status:          "pending",
			AssignedTo:      "",
			PlannedMinutes:  20,
			ExceptionReason: "前序工序未确认",
		}},
		reservationRows: []WIPReservationRow{{
			ID:                 11,
			WorkOrderID:        88,
			WorkOrderNo:        "WO-HUB-001",
			RunningItemID:      99,
			MaterialID:         10,
			MaterialName:       "包装袋",
			RequiredG:          5000,
			ReservedG:          3200,
			RemainingReservedG: 3200,
			AvailableG:         1000,
			Status:             "reserved",
			UpdatedAt:          "2026-06-17 09:20",
		}},
		stockEntryRows: []StockEntryRow{{
			ID:            7,
			EntryNo:       "SE-0000000007",
			EntryType:     "material_issue_to_wip",
			Purpose:       "material_transfer_for_manufacture",
			WorkOrderID:   88,
			JobCardID:     91,
			RunningItemID: 99,
			Status:        "submitted",
			CreatedAt:     "2026-06-17 09:15",
		}},
		ledgerRows: []WorkOrderLedgerEntryRow{{
			ID:            21,
			StockEntryID:  7,
			EntryNo:       "SE-0000000007",
			Purpose:       "material_transfer_for_manufacture",
			ItemName:      "包装袋",
			Warehouse:     "wip",
			QtyChangeG:    3200,
			QtyAfterG:     3200,
			SourceDocType: "stock_entry",
			SourceDocID:   7,
			CreatedAt:     "2026-06-17 09:15",
		}},
		qualityRows: []QualityInspectionRow{{
			ID:          3,
			Scope:       "work_order",
			ReferenceNo: "WO-HUB-001",
			ItemName:    "桂花乌龙",
			Result:      "hold",
			Note:        "待复核",
			CreatedAt:   "2026-06-17 09:45",
		}},
		productionLogs: ProductionLogsResult{Rows: []ProductionLogRow{{
			ID:                31,
			BatchID:           "BATCH-WO-88",
			ProductName:       "桂花乌龙",
			FinishedTotalG:    45400,
			FinishedBatchCode: "FP-88",
			FinishedAt:        "2026-06-17 11:00",
		}}},
		batchCosts: []BatchCostRow{{RunningItemID: 99, BatchID: "BATCH-WO-88", MaterialCost: 36.25, OperationCost: 12.5, TotalCost: 48.75, CreatedAt: "2026-06-17 11:05"}},
	}
	svc := NewService(repo)

	detail, err := svc.GetWorkOrderDetail(context.Background(), 88)
	if err != nil {
		t.Fatal(err)
	}
	hub := detail.ExecutionHub
	if hub.Header.WorkOrderNo != "WO-HUB-001" || hub.Header.ProductName != "桂花乌龙" || hub.Header.BomVersionID != 17 {
		t.Fatalf("execution hub header = %+v", hub.Header)
	}
	if hub.Readiness.CanStart || hub.Readiness.CanComplete || hub.Readiness.NextHandler != "仓库/物料" || hub.Readiness.Severity != "blocked" {
		t.Fatalf("execution readiness = %+v", hub.Readiness)
	}
	if !blockingReasonCodesContain(hub.Readiness.BlockingReasons, "wip_shortage") || !blockingReasonCodesContain(hub.Readiness.BlockingReasons, "quality_freeze") || !blockingReasonCodesContain(hub.Readiness.BlockingReasons, "workstation_unassigned") || !blockingReasonCodesContain(hub.Readiness.BlockingReasons, "prior_operation_incomplete") {
		t.Fatalf("blocking reasons = %+v", hub.Readiness.BlockingReasons)
	}
	if hub.Readiness.SuggestedAction != "open_wip_issue" || len(hub.Readiness.RelatedLinks) == 0 || hub.Readiness.RelatedLinks[0].View != "stockOperations" {
		t.Fatalf("readiness links/action = %+v", hub.Readiness)
	}
	if hub.WIPStatus.RequiredG != 5000 || hub.WIPStatus.AvailableG != 1000 || hub.WIPStatus.ShortageG != 1800 {
		t.Fatalf("WIP status = %+v", hub.WIPStatus)
	}
	if hub.QualityStatus.Status != "blocked" || hub.QualityStatus.Result != "hold" {
		t.Fatalf("quality status = %+v", hub.QualityStatus)
	}
	if len(hub.OperationProgress) != 2 || hub.OperationProgress[1].Status != "pending" {
		t.Fatalf("operation progress = %+v", hub.OperationProgress)
	}
	for _, typ := range []string{"operation", "inventory", "quality", "cost", "log"} {
		if !timelineHasType(hub.TraceTimeline, typ) {
			t.Fatalf("trace timeline missing %s: %+v", typ, hub.TraceTimeline)
		}
	}
	for _, key := range []string{
		"productionIssue",
		"productionSupplement",
		"productionReturn",
		"productionConsume",
		"finishedReceipt",
		"openJobCard",
		"openQuality",
	} {
		if contextActionKeysContain(hub.ContextActions, key) {
			continue
		}
		t.Fatalf("context actions missing %s: %+v", key, hub.ContextActions)
	}
	if repo.qualityQuery.Scope != "work_order" {
		t.Fatalf("quality query = %+v", repo.qualityQuery)
	}
}

func TestReleasedWorkOrderUsesFrozenSnapshotWIPCoverageWithoutReservation(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{
			ID: 88, WorkOrderNo: "WO-PR559-001", Status: "released",
			ProductID: 9, ProductName: "如目达摩", PlannedG: 6356,
		}},
		jobCards: []JobCardRow{{
			ID: 91, WorkOrderID: 88, WorkOrderNo: "WO-PR559-001",
			Operation: "烘焙", Workstation: "15kg烘焙机", Status: "pending",
		}},
		wipCoverage: ProductionWIPStatus{
			DataComplete: true,
			Status:       "blocked",
			RequiredG:    6356,
			AvailableG:   1200,
			ShortageG:    5156,
			Materials: []WIPReservationRow{{
				WorkOrderID: 88, WorkOrderNo: "WO-PR559-001",
				MaterialID: 10, MaterialName: "如目达摩生豆",
				InventoryUnit: "g", QuantityBasis: "weight",
				RequiredQty: 6356, AvailableQty: 1200, ShortageQty: 5156,
				RequiredG: 6356, AvailableG: 1200, ShortageG: 5156,
			}},
		},
	}
	svc := NewService(repo)

	detail, err := svc.GetWorkOrderDetail(context.Background(), 88)
	if err != nil {
		t.Fatal(err)
	}
	if detail.ExecutionHub.Readiness.CanStart {
		t.Fatalf("released work order with WIP shortage must not be startable: %+v", detail.ExecutionHub.Readiness)
	}
	if detail.ExecutionHub.WIPStatus.ShortageG != 5156 || len(detail.Materials) != 1 {
		t.Fatalf("coverage = %+v materials=%+v", detail.ExecutionHub.WIPStatus, detail.Materials)
	}
	preview, err := svc.PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 88, Action: "issue"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Document.WorkOrderNo != "WO-PR559-001" || preview.Document.RunningItemID != 0 {
		t.Fatalf("preview context = %+v", preview.Document)
	}
	if len(preview.Document.Items) != 1 {
		t.Fatalf("preview items = %+v", preview.Document.Items)
	}
	item := preview.Document.Items[0]
	if item.InventoryUnit != "g" || item.QuantityBasis != "weight" || item.RequiredQty != 6356 || item.RemainingQty != 5156 || item.DefaultQty != 5156 || item.QtyG != 5156 || item.QtyUnits != 0 {
		t.Fatalf("preview item = %+v", item)
	}
}

func TestWorkOrderStockDocumentPreviewUsesFrozenWarehouseOwnerAndBatch(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders:        []WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR629-001", Status: "released"}},
		wipCoverage:       ProductionWIPStatus{DataComplete: true, Status: "ok"},
		usesFrozenSources: true,
		frozenSourceItems: []StockEntryItemCommand{{
			MaterialID: 10, ItemName: "客户生豆", InventoryUnit: "kg", QuantityBasis: "weight",
			FromWarehouse: "customer_74_raw", ToWarehouse: "wip", OwnerCustomerID: 74,
			BatchCode: "MB-C74-001", QtyG: 6000, DefaultQty: 6,
		}},
	}
	preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 88, Action: "issue"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Document.Items) != 1 {
		t.Fatalf("items = %+v", preview.Document.Items)
	}
	item := preview.Document.Items[0]
	if item.FromWarehouse != "customer_74_raw" || item.ToWarehouse != "wip" || item.OwnerCustomerID != 74 || item.BatchCode != "MB-C74-001" {
		t.Fatalf("frozen item changed = %+v", item)
	}
}

func TestWorkOrderStockDocumentPreviewRestoresExistingDraft(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR559-001", Status: "released"}},
		wipCoverage: ProductionWIPStatus{
			DataComplete: true, Status: "blocked", RequiredG: 7751, ShortageG: 7751,
			Materials: []WIPReservationRow{{
				WorkOrderID: 88, WorkOrderNo: "WO-PR559-001",
				MaterialID: 10, MaterialName: "如目达摩生豆",
				InventoryUnit: "g", QuantityBasis: "weight",
				RequiredQty: 7751, AvailableQty: 0, ShortageQty: 7751,
				RequiredG: 7751, AvailableG: 0, ShortageG: 7751,
			}},
		},
		stockDraft: &StockEntryCommand{
			ID: 71, EntryNo: "SE-0000000071", Status: "draft",
			Purpose: "material_transfer_for_manufacture", WorkOrderID: 88,
			Items: []StockEntryItemCommand{{MaterialID: 10, ItemName: "如目达摩生豆", InventoryUnit: "g", QtyG: 8000}},
		},
	}
	preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 88, Action: "issue"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Document.ID != 71 || preview.Document.EntryNo != "SE-0000000071" || preview.Document.WorkOrderNo != "WO-PR559-001" || len(preview.Document.Items) != 1 {
		t.Fatalf("restored draft = %+v", preview.Document)
	}
	item := preview.Document.Items[0]
	if item.QtyG != 8000 || item.RequiredQty != 7751 || item.RemainingQty != 7751 || item.DefaultQty != 8000 {
		t.Fatalf("refreshed draft item = %+v, want preserved 8000g draft and 7751g suggestion", item)
	}
	if len(preview.Warnings) != 1 ||
		!strings.Contains(preview.Warnings[0], "当前建议领用7751g") ||
		!strings.Contains(preview.Warnings[0], "草稿保留8000g") ||
		!strings.Contains(preview.Warnings[0], "超出部分提交后作为可用 WIP 库存保留") {
		t.Fatalf("preview warnings = %+v", preview.Warnings)
	}
	if repo.stockDraft.Items[0].QtyG != 8000 {
		t.Fatalf("preview must not persistently mutate stored draft, got %+v", repo.stockDraft.Items[0])
	}
}

func TestWorkOrderStockDocumentPreviewOpensOnlyTheRequestedDraft(t *testing.T) {
	baseRepo := func() *fakeFlowRepo {
		return &fakeFlowRepo{
			workOrders: []WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR561-001", Status: "released"}},
			wipCoverage: ProductionWIPStatus{
				DataComplete: true, Status: "blocked",
				Materials: []WIPReservationRow{{
					WorkOrderID: 88, MaterialID: 10, MaterialName: "哥伦比亚",
					InventoryUnit: "g", QuantityBasis: "weight", RequiredQty: 1974, ShortageQty: 1974,
					RequiredG: 1974, ShortageG: 1974,
				}},
			},
			stockDraft: &StockEntryCommand{
				ID: 72, EntryNo: "SE-0000000072", Status: "draft",
				Purpose: "material_transfer_for_manufacture", WorkOrderID: 88,
				Items: []StockEntryItemCommand{{
					MaterialID: 10, ItemName: "哥伦比亚", InventoryUnit: "g", QtyG: 60000,
				}},
			},
		}
	}

	repo := baseRepo()
	preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{
		ID: 88, Action: "issue", StockDocumentID: 72,
	})
	if err != nil {
		t.Fatal(err)
	}
	if repo.stockDraftWorkOrder != 88 || repo.stockDraftAction != "issue" || repo.stockDraftID != 72 {
		t.Fatalf("draft lookup = work_order:%d action:%s id:%d", repo.stockDraftWorkOrder, repo.stockDraftAction, repo.stockDraftID)
	}
	if preview.Document.ID != 72 {
		t.Fatalf("preview document id=%d, want requested draft 72", preview.Document.ID)
	}

	repo = baseRepo()
	_, err = NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{
		ID: 88, Action: "issue", StockDocumentID: 71,
	})
	if err == nil || !strings.Contains(err.Error(), "指定库存草稿不存在") {
		t.Fatalf("mismatched draft error=%v", err)
	}
	if repo.stockDraftID != 71 {
		t.Fatalf("requested draft id=%d, want 71", repo.stockDraftID)
	}
}

func TestWorkOrderStockDocumentPreviewUsesTypedOutputAndFrozenTargetWarehouse(t *testing.T) {
	tests := []struct {
		name          string
		workOrder     WorkOrderRow
		wantItemType  string
		wantMaterial  int64
		wantProduct   int64
		wantQtyG      int64
		wantQtyUnits  int64
		wantUnit      string
		wantWarehouse string
	}{
		{
			name: "material output",
			workOrder: WorkOrderRow{
				ID: 88, WorkOrderNo: "WO-MATERIAL-001", Status: "running", RunningItemID: 99,
				OutputType: "material", OutputMaterialID: 10, OutputName: "烘焙熟豆",
				OutputQty: 12.7, OutputUnit: "kg", TargetWarehouse: "wip",
			},
			wantItemType: "material", wantMaterial: 10, wantQtyG: 12_700,
			wantUnit: "kg", wantWarehouse: "wip",
		},
		{
			name: "product output with non-default warehouse",
			workOrder: WorkOrderRow{
				ID: 89, WorkOrderNo: "WO-PRODUCT-001", Status: "running", RunningItemID: 100,
				OutputType: "product", OutputProductID: 20, OutputName: "门店咖啡豆",
				ProductID: 20, ProductName: "门店咖啡豆", SpecG: 227,
				PlannedUnits: 100, TargetWarehouse: "finished_shop",
			},
			wantItemType: "finished_product", wantProduct: 20, wantQtyUnits: 100,
			wantWarehouse: "finished_shop",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeFlowRepo{workOrders: []WorkOrderRow{tt.workOrder}}
			preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: tt.workOrder.ID, Action: "finish"})
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Document.Items) != 1 {
				t.Fatalf("preview items=%+v, want one typed output", preview.Document.Items)
			}
			item := preview.Document.Items[0]
			if item.ItemType != tt.wantItemType || item.MaterialID != tt.wantMaterial || item.ProductID != tt.wantProduct ||
				item.QtyG != tt.wantQtyG || item.QtyUnits != tt.wantQtyUnits || item.InventoryUnit != tt.wantUnit ||
				item.ToWarehouse != tt.wantWarehouse {
				t.Fatalf("typed finish preview item=%+v", item)
			}
		})
	}
}

func TestWorkOrderStockDocumentPreviewPreservesDraftItemsWithoutCurrentWIPShortage(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR560-001", Status: "released"}},
		wipCoverage: ProductionWIPStatus{
			DataComplete: true, Status: "ready", RequiredG: 7751, AvailableG: 7751,
			Materials: []WIPReservationRow{{
				WorkOrderID: 88, WorkOrderNo: "WO-PR560-001",
				MaterialID: 10, MaterialName: "如目达摩生豆",
				InventoryUnit: "g", QuantityBasis: "weight",
				RequiredQty: 7751, AvailableQty: 7751, ShortageQty: 0,
				RequiredG: 7751, AvailableG: 7751, ShortageG: 0,
			}},
		},
		stockDraft: &StockEntryCommand{
			ID: 72, EntryNo: "SE-0000000072", Status: "draft",
			Purpose: "material_transfer_for_manufacture", WorkOrderID: 88,
			Items: []StockEntryItemCommand{{MaterialID: 10, ItemName: "如目达摩生豆", InventoryUnit: "g", QtyG: 7751}},
		},
	}
	preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 88, Action: "issue"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Document.Items) != 1 {
		t.Fatalf("bulk draft must remain visible when current WIP shortage is zero: %+v", preview.Document.Items)
	}
	item := preview.Document.Items[0]
	if item.QtyG != 7751 || item.RemainingQty != 0 || item.DefaultQty != 7751 {
		t.Fatalf("preserved zero-shortage draft item = %+v", item)
	}
	if len(preview.Warnings) != 1 ||
		!strings.Contains(preview.Warnings[0], "当前工单 WIP 已满足") ||
		!strings.Contains(preview.Warnings[0], "草稿7751g仍保留") ||
		!strings.Contains(preview.Warnings[0], "作为可用 WIP 库存保留") {
		t.Fatalf("preview warnings = %+v", preview.Warnings)
	}
	if len(repo.stockDraft.Items) != 1 || repo.stockDraft.Items[0].QtyG != 7751 {
		t.Fatalf("preview must not persistently mutate stored draft, got %+v", repo.stockDraft.Items)
	}
}

func TestWorkOrderStockDocumentPreviewKeepsZeroSuggestionMaterialAvailableForBulkIssue(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR561-001", Status: "released"}},
		wipCoverage: ProductionWIPStatus{
			DataComplete: true, Status: "ready", RequiredG: 1974, AvailableG: 1974,
			Materials: []WIPReservationRow{{
				WorkOrderID: 88, WorkOrderNo: "WO-PR561-001",
				MaterialID: 10, MaterialName: "哥伦比亚",
				InventoryUnit: "g", QuantityBasis: "weight",
				RequiredQty: 1974, AvailableQty: 1974, ShortageQty: 0,
				RequiredG: 1974, AvailableG: 1974, ShortageG: 0,
			}},
		},
	}
	preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 88, Action: "issue"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Document.Items) != 1 {
		t.Fatalf("zero-suggestion material must remain editable for a standard bulk issue: %+v", preview.Document.Items)
	}
	item := preview.Document.Items[0]
	if item.MaterialID != 10 || item.QtyG != 0 || item.DefaultQty != 0 || item.RemainingQty != 0 {
		t.Fatalf("zero-suggestion bulk issue item = %+v", item)
	}
}

func TestWorkOrderStockDocumentPreviewUsesRememberedStandardBatchAboveSuggestion(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		shortageQty float64
		shortageG   int64
	}{
		{name: "shortage remains", shortageQty: 1974, shortageG: 1974},
		{name: "wip already covered", shortageQty: 0, shortageG: 0},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeFlowRepo{
				workOrders: []WorkOrderRow{{ID: 83, WorkOrderNo: "WO-PP-0000000083-0000000051", Status: "released"}},
				wipCoverage: ProductionWIPStatus{
					DataComplete: true,
					Status:       "blocked",
					Materials: []WIPReservationRow{{
						WorkOrderID: 83, WorkOrderNo: "WO-PP-0000000083-0000000051",
						MaterialID: 10, MaterialName: "哥伦比亚",
						InventoryUnit: "g", QuantityBasis: "weight",
						RequiredQty: 1974, ShortageQty: testCase.shortageQty, RememberedQty: 60000,
						RequiredG: 1974, ShortageG: testCase.shortageG,
					}},
				},
			}
			preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 83, Action: "issue"})
			if err != nil {
				t.Fatal(err)
			}
			if len(preview.Document.Items) != 1 {
				t.Fatalf("items = %+v", preview.Document.Items)
			}
			item := preview.Document.Items[0]
			if item.QtyG != 60000 || item.DefaultQty != 60000 || item.RememberedQty != 60000 {
				t.Fatalf("remembered standard batch was not restored: %+v", item)
			}
			if item.RemainingQty != testCase.shortageQty {
				t.Fatalf("remaining_qty = %v, want suggestion %v", item.RemainingQty, testCase.shortageQty)
			}
		})
	}
}

func TestWorkOrderConsumptionPreviewCapsBulkWIPAtFrozenMaterialRequirement(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{
			ID: 83, WorkOrderNo: "WO-PP-0000000083-0000000051", Status: "released",
		}},
		wipCoverage: ProductionWIPStatus{
			DataComplete: true, Status: "ready",
			Materials: []WIPReservationRow{
				{
					WorkOrderID: 83, WorkOrderNo: "WO-PP-0000000083-0000000051",
					MaterialID: 10, MaterialName: "哥伦比亚",
					InventoryUnit: "g", QuantityBasis: "weight",
					RequiredQty: 1974, AvailableQty: 60000, RequiredG: 1974, WIPG: 60000,
				},
				{
					WorkOrderID: 83, WorkOrderNo: "WO-PP-0000000083-0000000051",
					MaterialID: 11, MaterialName: "耶加雪菲G2",
					InventoryUnit: "g", QuantityBasis: "weight",
					RequiredQty: 1974, AvailableQty: 60000, RequiredG: 1974, WIPG: 60000,
				},
				{
					WorkOrderID: 83, WorkOrderNo: "WO-PP-0000000083-0000000051",
					MaterialID: 12, MaterialName: "黄波旁水洗",
					InventoryUnit: "g", QuantityBasis: "weight",
					RequiredQty: 3948, AvailableQty: 60000, RequiredG: 3948, WIPG: 60000,
				},
			},
		},
		ledgerRows: []WorkOrderLedgerEntryRow{
			{StockEntryID: 71, Purpose: "material_transfer_for_manufacture", ItemType: "material", ItemID: 10, Warehouse: "wip", QtyChangeG: 60000},
			{StockEntryID: 71, Purpose: "material_transfer_for_manufacture", ItemType: "material", ItemID: 11, Warehouse: "wip", QtyChangeG: 60000},
			{StockEntryID: 71, Purpose: "material_transfer_for_manufacture", ItemType: "material", ItemID: 12, Warehouse: "wip", QtyChangeG: 60000},
		},
	}

	preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 83, Action: "consume"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Document.Items) != 3 {
		t.Fatalf("consume items = %+v", preview.Document.Items)
	}
	want := map[int64]int64{10: 1974, 11: 1974, 12: 3948}
	for _, item := range preview.Document.Items {
		if item.QtyG != want[item.MaterialID] {
			t.Fatalf("%s consume qty = %dg, want %dg; 60Kg bulk remainder must stay in WIP", item.ItemName, item.QtyG, want[item.MaterialID])
		}
	}

	returnPreview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 83, Action: "return"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range returnPreview.Document.Items {
		if item.QtyG != 60000 {
			t.Fatalf("%s return qty = %dg, want all 60000g returnable WIP", item.ItemName, item.QtyG)
		}
	}
}

func TestWorkOrderConsumptionPreviewSubtractsSubmittedConsumption(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{ID: 83, WorkOrderNo: "WO-PP-0000000083-0000000051", Status: "running"}},
		wipCoverage: ProductionWIPStatus{
			DataComplete: true, Status: "ready",
			Materials: []WIPReservationRow{{
				WorkOrderID: 83, WorkOrderNo: "WO-PP-0000000083-0000000051",
				MaterialID: 10, MaterialName: "哥伦比亚",
				InventoryUnit: "g", QuantityBasis: "weight",
				RequiredQty: 1974, AvailableQty: 59500, RequiredG: 1974, WIPG: 59500,
			}},
		},
		ledgerRows: []WorkOrderLedgerEntryRow{
			{StockEntryID: 71, Purpose: "material_transfer_for_manufacture", ItemType: "material", ItemID: 10, Warehouse: "wip", QtyChangeG: 60000},
			{StockEntryID: 72, Purpose: "material_consumption_for_manufacture", ItemType: "material", ItemID: 10, Warehouse: "wip", QtyChangeG: -500},
		},
	}

	preview, err := NewService(repo).PreviewWorkOrderStockDocument(context.Background(), StockDocumentPreviewCommand{ID: 83, Action: "consume"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Document.Items) != 1 || preview.Document.Items[0].QtyG != 1474 {
		t.Fatalf("consume preview = %+v, want frozen 1974g minus submitted 500g", preview.Document.Items)
	}
}

func TestRefreshWorkOrderStockDocumentDraftPreservesCanonicalWeightGrams(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		unit      string
		shortageG int64
	}{
		{name: "kg", unit: "kg", shortageG: 2007},
		{name: "lb", unit: "lb", shortageG: 61},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			draft := &StockEntryCommand{
				Items: []StockEntryItemCommand{{
					MaterialID: 10, ItemName: "测试物料",
					InventoryUnit: testCase.unit, QtyG: testCase.shortageG + 1000,
				}},
			}
			expectedRemainingQuantity := inventoryQuantityFromGrams(testCase.shortageG, testCase.unit)
			expectedDraftQuantity := inventoryQuantityFromGrams(testCase.shortageG+1000, testCase.unit)
			warnings := refreshWorkOrderStockDocumentDraft(draft, []WIPReservationRow{{
				MaterialID: 10, MaterialName: "测试物料",
				InventoryUnit: testCase.unit, QuantityBasis: "weight",
				RequiredQty: expectedRemainingQuantity, ShortageQty: expectedRemainingQuantity,
				RequiredG: testCase.shortageG, ShortageG: testCase.shortageG,
			}}, "issue")
			if len(warnings) != 1 {
				t.Fatalf("warnings = %+v", warnings)
			}
			if len(draft.Items) != 1 {
				t.Fatalf("draft items = %+v", draft.Items)
			}
			item := draft.Items[0]
			if item.QtyG != testCase.shortageG+1000 {
				t.Fatalf("qty_g = %d, want preserved canonical draft %d", item.QtyG, testCase.shortageG+1000)
			}
			if item.DefaultQty != expectedDraftQuantity || item.RemainingQty != expectedRemainingQuantity {
				t.Fatalf(
					"default/remaining = %v/%v, want preserved draft %v and suggestion %v",
					item.DefaultQty,
					item.RemainingQty,
					expectedDraftQuantity,
					expectedRemainingQuantity,
				)
			}
		})
	}
}

func TestProductionWorkstationOverviewAnswersProductionAndStationQuestions(t *testing.T) {
	repo := &fakeFlowRepo{
		workOrders: []WorkOrderRow{{
			ID:              88,
			WorkOrderNo:     "WO-OVERVIEW-001",
			RunningItemID:   99,
			ProductName:     "桂花乌龙",
			SpecG:           227,
			PlannedG:        45400,
			PlannedUnits:    200,
			PlannedLooseG:   0,
			Status:          "running",
			OrderNos:        "SO-20260613-001",
			AssignedTo:      "生产主管",
			Priority:        7,
			WorkCenter:      "包装线",
			PlannedStartAt:  "2026-06-13 09:00",
			PlannedEndAt:    "2026-06-13 11:00",
			SchedulingNote:  "优先发货",
			MaterialSummary: "挂耳包材",
		}, {
			ID:             89,
			WorkOrderNo:    "WO-OVERVIEW-002",
			ProductName:    "日晒瑰夏",
			SpecG:          454,
			PlannedG:       90800,
			PlannedUnits:   200,
			Status:         "released",
			OrderNos:       "SO-20260613-002",
			Priority:       4,
			WorkCenter:     "烘焙线",
			PlannedStartAt: "2026-06-13 10:00",
		}},
		jobCards: []JobCardRow{{
			ID:                91,
			WorkOrderID:       88,
			WorkOrderNo:       "WO-OVERVIEW-001",
			ProductName:       "桂花乌龙",
			SpecG:             227,
			OrderNos:          "SO-20260613-001",
			SequenceNo:        20,
			Operation:         "包装",
			Workstation:       "包装工位A",
			WorkCenter:        "包装线",
			Status:            "running",
			AssignedTo:        "阿强",
			Priority:          9,
			PlannedMinutes:    40,
			PlannedStartAt:    "2026-06-13 09:00",
			PlannedEndAt:      "2026-06-13 09:40",
			SchedulingNote:    "先做急单",
			PlannedOutputG:    45400,
			PlannedBatchCount: 2,
		}, {
			ID:             92,
			WorkOrderID:    88,
			WorkOrderNo:    "WO-OVERVIEW-001",
			ProductName:    "桂花乌龙",
			SpecG:          227,
			OrderNos:       "SO-20260613-001",
			SequenceNo:     30,
			Operation:      "贴标",
			Workstation:    "包装工位A",
			WorkCenter:     "包装线",
			Status:         "pending",
			AssignedTo:     "阿强",
			Priority:       6,
			PlannedMinutes: 20,
			PlannedStartAt: "2026-06-13 09:45",
		}, {
			ID:              93,
			WorkOrderID:     89,
			WorkOrderNo:     "WO-OVERVIEW-002",
			ProductName:     "日晒瑰夏",
			SpecG:           454,
			OrderNos:        "SO-20260613-002",
			SequenceNo:      10,
			Operation:       "烘焙",
			Workstation:     "布勒 18kg",
			WorkCenter:      "烘焙线",
			Status:          "paused",
			AssignedTo:      "现场主管",
			Priority:        8,
			PlannedMinutes:  55,
			ExceptionReason: "缺少生豆领料",
		}},
	}
	svc := NewService(repo)

	overview, err := svc.ProductionWorkstationOverview(context.Background(), ProductionWorkstationOverviewQuery{Limit: 999})
	if err != nil {
		t.Fatal(err)
	}
	if repo.workOrderQuery.Limit != 500 || repo.jobCardQuery.Limit != 500 {
		t.Fatalf("overview should clamp source queries to 500, workOrder=%+v jobCard=%+v", repo.workOrderQuery, repo.jobCardQuery)
	}
	if overview.TotalTasks != 3 {
		t.Fatalf("TotalTasks=%d tasks=%+v", overview.TotalTasks, overview.Tasks)
	}
	if overview.TodaySummary.PlannedTasks != 3 || overview.TodaySummary.PendingTasks != 1 || overview.TodaySummary.RunningTasks != 1 || overview.TodaySummary.BlockedTasks != 1 {
		t.Fatalf("today summary = %+v", overview.TodaySummary)
	}
	if overview.NavBadges["productionOverview"].Pending != 1 || overview.NavBadges["productionOverview"].Blocked != 1 || overview.NavBadges["productionOverview"].Running != 1 {
		t.Fatalf("production overview nav badge = %+v", overview.NavBadges["productionOverview"])
	}
	if overview.NavBadges["workstationView"].Pending != 1 || overview.NavBadges["workstationView"].Blocked != 1 || overview.NavBadges["workstationView"].Running != 1 {
		t.Fatalf("workstation nav badge = %+v", overview.NavBadges["workstationView"])
	}
	if overview.NavBadges["produceRunning"].Running != 1 {
		t.Fatalf("produce running nav badge = %+v", overview.NavBadges["produceRunning"])
	}
	if !summaryHas(overview.StatusSummary, "执行中", 1) || !summaryHas(overview.StatusSummary, "待处理", 1) || !summaryHas(overview.StatusSummary, "异常", 1) {
		t.Fatalf("status summary = %+v", overview.StatusSummary)
	}
	if !summaryHas(overview.BlockedSummary, "缺少生豆领料", 1) {
		t.Fatalf("blocked summary = %+v", overview.BlockedSummary)
	}
	if !summaryHas(overview.PrioritySummary, "P9", 1) {
		t.Fatalf("priority summary = %+v", overview.PrioritySummary)
	}

	packLoad := findWorkstationLoad(overview.WorkstationLoad, "包装线")
	if packLoad.Workstation != "包装线" || packLoad.RunningTasks != 1 || packLoad.PendingTasks != 1 || packLoad.LoadMinutes != 60 {
		t.Fatalf("包装线 load = %+v", packLoad)
	}
	if packLoad.QueueCount != 2 || packLoad.BlockedCount != 0 || packLoad.EstimatedMinutes != 60 || packLoad.LoadStatus != "normal" {
		t.Fatalf("包装线 enhanced load = %+v", packLoad)
	}
	if packLoad.CurrentTask != "包装 / 桂花乌龙" || packLoad.NextTask != "贴标 / 桂花乌龙" {
		t.Fatalf("包装线 current/next = %+v", packLoad)
	}
	roastLoad := findWorkstationLoad(overview.WorkstationLoad, "烘焙线")
	if roastLoad.BlockedTasks != 1 || roastLoad.BlockingReason != "缺少生豆领料" {
		t.Fatalf("烘焙线 load = %+v", roastLoad)
	}
	if roastLoad.BlockedCount != 1 || roastLoad.LoadStatus != "blocked" {
		t.Fatalf("烘焙线 enhanced load = %+v", roastLoad)
	}

	running := findProductionTask(overview.Tasks, 91)
	if running.RunningItemID != 99 || running.StatusLabel != "执行中" || running.Readiness != "running" || running.ReadinessLabel != "执行中" || running.NextHandler != "阿强" {
		t.Fatalf("running task = %+v", running)
	}
	if running.ReadinessDetail.CanStart || !running.ReadinessDetail.CanComplete || running.ReadinessDetail.SuggestedAction != "complete_job_card" || running.ReadinessDetail.Severity != "info" {
		t.Fatalf("running task readiness detail = %+v", running.ReadinessDetail)
	}
	for _, action := range []string{"pause", "complete", "report_exception", "material_call"} {
		if !stringSliceContains(running.AvailableActions, action) {
			t.Fatalf("running task actions missing %s: %+v", action, running.AvailableActions)
		}
	}
	if stringSliceContains(running.AvailableActions, "partial_finish") {
		t.Fatalf("running task actions must not expose partial_finish: %+v", running.AvailableActions)
	}
	blocked := findProductionTask(overview.Tasks, 93)
	if !blocked.IsBlocked || blocked.Readiness != "blocked" || blocked.ReadinessLabel != "不能做" || blocked.BlockingReason != "缺少生豆领料" || blocked.NextHandler != "现场主管" {
		t.Fatalf("blocked task = %+v", blocked)
	}
	if blocked.ReadinessDetail.CanStart || blocked.ReadinessDetail.CanComplete || blocked.ReadinessDetail.Severity != "blocked" || len(blocked.ReadinessDetail.BlockingReasons) == 0 || blocked.ReadinessDetail.SuggestedAction != "open_wip_issue" {
		t.Fatalf("blocked task readiness detail = %+v", blocked.ReadinessDetail)
	}
	if !relatedLinksContainView(blocked.ReadinessDetail.RelatedLinks, "stockOperations") {
		t.Fatalf("blocked task links = %+v", blocked.ReadinessDetail.RelatedLinks)
	}
	for _, action := range []string{"resume", "complete"} {
		if !stringSliceContains(blocked.AvailableActions, action) {
			t.Fatalf("blocked task actions missing %s: %+v", action, blocked.AvailableActions)
		}
	}
	for _, forbidden := range []string{"start", "pause", "report_exception", "material_call"} {
		if stringSliceContains(blocked.AvailableActions, forbidden) {
			t.Fatalf("paused task actions must not include %s: %+v", forbidden, blocked.AvailableActions)
		}
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

func TestServiceCompleteWorkOrderLeavesEmptyWarehouseForRepositoryResolution(t *testing.T) {
	repo := &fakeFlowRepo{}
	svc := NewService(repo)

	if _, err := svc.CompleteWorkOrder(context.Background(), WorkOrderCompleteCommand{
		ID:            88,
		FinishedUnits: 1,
		Warehouse:     "   ",
		Operator:      "主管",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.completeWorkOrder.Warehouse != "" {
		t.Fatalf("warehouse=%q, want repository to resolve omitted warehouse from frozen work order", repo.completeWorkOrder.Warehouse)
	}
}

func summaryHas(rows []ProductionSummaryCount, label string, count int) bool {
	for _, row := range rows {
		if row.Label == label && row.Count == count {
			return true
		}
	}
	return false
}

func findWorkstationLoad(rows []ProductionWorkstationLoad, workstation string) ProductionWorkstationLoad {
	for _, row := range rows {
		if row.Workstation == workstation {
			return row
		}
	}
	return ProductionWorkstationLoad{}
}

func findProductionTask(rows []ProductionTask, jobCardID int64) ProductionTask {
	for _, row := range rows {
		if row.JobCardID == jobCardID {
			return row
		}
	}
	return ProductionTask{}
}

func stringSliceContains(rows []string, want string) bool {
	for _, row := range rows {
		if row == want {
			return true
		}
	}
	return false
}

func blockingReasonCodesContain(rows []ProductionBlockingReason, want string) bool {
	for _, row := range rows {
		if row.Code == want {
			return true
		}
	}
	return false
}

func relatedLinksContainView(rows []ProductionRelatedLink, view string) bool {
	for _, row := range rows {
		if row.View == view {
			return true
		}
	}
	return false
}

func timelineHasType(rows []ProductionTraceTimelineEntry, typ string) bool {
	for _, row := range rows {
		if row.Type == typ {
			return true
		}
	}
	return false
}

func contextActionKeysContain(rows []ProductionContextAction, key string) bool {
	for _, row := range rows {
		if row.Key == key {
			return true
		}
	}
	return false
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
