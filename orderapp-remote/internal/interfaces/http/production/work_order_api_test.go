package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	stockapp "orderapp/internal/application/stock"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type workOrderAPIRepo struct {
	rows           []productionapp.WorkOrderRow
	jobCards       []productionapp.JobCardRow
	workOrderQuery productionapp.WorkOrderQuery
	jobCardQuery   productionapp.JobCardQuery
	jobCardActual  productionapp.JobCardActualsCommand
	jobCardAction  productionapp.JobCardActionCommand

	createPlan           productionapp.CreateProductionPlanCommand
	savePlanSplits       productionapp.SaveProductionPlanOperationSplitsCommand
	previewPlanSplits    productionapp.PreviewProductionPlanOperationSplitsCommand
	saveWorkOrderSplits  productionapp.SaveWorkOrderOperationSplitsCommand
	planSplits           []productionapp.ProductionPlanOperationSplit
	workOrderSplitRows   []productionapp.JobCardRow
	cancelPlan           productionapp.CancelProductionPlanCommand
	submitPlan           productionapp.SubmitProductionPlanCommand
	submitPlans          []productionapp.SubmitProductionPlanCommand
	startWorkOrder       productionapp.WorkOrderStartCommand
	completeWorkOrder    productionapp.WorkOrderCompleteCommand
	cancelWorkOrder      productionapp.WorkOrderCancelCommand
	productionPlanQuery  productionapp.ProductionPlanQuery
	productionPlan       productionapp.ProductionPlanDetail
	submittedPlan        productionapp.ProductionPlanSubmitResult
	submitPlanByID       map[int64]productionapp.ProductionPlanSubmitResult
	submitPlanErrByID    map[int64]error
	workOrderStarted     productionapp.WorkOrderStartResult
	workOrderCompleted   productionapp.WorkOrderCompleteResult
	workOrderCancelled   productionapp.WorkOrderRow
	scheduleAssignment   productionapp.ScheduleAssignmentCommand
	capacityCalendar     productionapp.CapacityCalendarCommand
	scheduleQuery        productionapp.ScheduleBoardQuery
	mrpQuery             productionapp.MRPSuggestionQuery
	traceQuery           productionapp.ProductionTraceAnalyticsQuery
	stockEntry           productionapp.StockEntryCommand
	stockDocumentDraft   *productionapp.StockEntryCommand
	stockDocumentDraftID int64
	stockEntryQuery      productionapp.StockEntryQuery
	stockEntryRows       []productionapp.StockEntryRow
	stockEntryID         int64
	reservationQuery     productionapp.WIPReservationQuery
	reservationRows      []productionapp.WIPReservationRow
	productionLogsQuery  productionapp.ProductionLogsQuery
	productionLogs       productionapp.ProductionLogsResult
	batchCostQuery       productionapp.BatchCostQuery
	batchCosts           []productionapp.BatchCostRow
	ledgerQuery          productionapp.WorkOrderLedgerQuery
	ledgerRows           []productionapp.WorkOrderLedgerEntryRow
	qualityQuery         productionapp.QualityInspectionQuery
	qualityRows          []productionapp.QualityInspectionRow
}

type stockDocumentAPIRepo struct {
	stockapp.Repository
	detail      stockapp.StockDocumentDetail
	createCount int
	updateCount int
	submitCount int
	cancelCount int
}

func (r *stockDocumentAPIRepo) CreateStockDocumentDraft(_ context.Context, cmd stockapp.StockDocumentCommand) (stockapp.StockDocumentDetail, error) {
	r.createCount++
	r.detail = stockapp.StockDocumentDetail{
		StockDocumentRow: stockapp.StockDocumentRow{
			ID: 31, EntryNo: "SE-0000000031", EntryType: cmd.EntryType, Purpose: cmd.Purpose,
			IsReturn: cmd.IsReturn, Status: "draft", WorkOrderID: cmd.WorkOrderID, Operator: cmd.Operator,
		},
	}
	return r.detail, nil
}

func (r *stockDocumentAPIRepo) UpdateStockDocumentDraft(_ context.Context, id int64, cmd stockapp.StockDocumentCommand) (stockapp.StockDocumentDetail, error) {
	r.updateCount++
	r.detail.ID = id
	r.detail.Purpose = cmd.Purpose
	r.detail.EntryType = cmd.EntryType
	r.detail.IsReturn = cmd.IsReturn
	r.detail.Note = cmd.Note
	return r.detail, nil
}

func (r *stockDocumentAPIRepo) SubmitStockDocument(_ context.Context, id int64, actor string) (stockapp.StockDocumentDetail, error) {
	if r.detail.Status != "submitted" {
		r.submitCount++
	}
	r.detail.ID = id
	r.detail.Status = "submitted"
	r.detail.Operator = actor
	return r.detail, nil
}

func (r *stockDocumentAPIRepo) CancelStockDocument(_ context.Context, id int64, actor string) (stockapp.StockDocumentDetail, error) {
	if r.detail.Status != "cancelled" {
		r.cancelCount++
	}
	r.detail.ID = id
	r.detail.Status = "cancelled"
	r.detail.Operator = actor
	return r.detail, nil
}

func (r *stockDocumentAPIRepo) ListStockDocuments(_ context.Context, _ stockapp.StockDocumentQuery) (stockapp.StockDocumentResult, error) {
	return stockapp.StockDocumentResult{Rows: []stockapp.StockDocumentRow{r.detail.StockDocumentRow}, Total: 1, Limit: 100}, nil
}

func (r *stockDocumentAPIRepo) GetStockDocument(_ context.Context, id int64) (stockapp.StockDocumentDetail, error) {
	r.detail.ID = id
	return r.detail, nil
}

func (r *stockDocumentAPIRepo) CreateAndSubmitStockDocument(ctx context.Context, cmd stockapp.StockDocumentCommand) (stockapp.StockDocumentDetail, error) {
	detail, err := r.CreateStockDocumentDraft(ctx, cmd)
	if err != nil {
		return stockapp.StockDocumentDetail{}, err
	}
	return r.SubmitStockDocument(ctx, detail.ID, cmd.Operator)
}

func (r *workOrderAPIRepo) CreateBatch(ctx context.Context, cmd productionapp.CreateBatchCommand) (productionapp.CreateBatchResult, error) {
	return productionapp.CreateBatchResult{}, nil
}
func (r *workOrderAPIRepo) ListBatches(ctx context.Context, cmd productionapp.ListBatchesCommand) ([]productionapp.BatchListItem, error) {
	return nil, nil
}
func (r *workOrderAPIRepo) Detail(ctx context.Context, batchID string) (productionapp.BatchDetail, error) {
	return productionapp.BatchDetail{}, nil
}
func (r *workOrderAPIRepo) PreviewDeduct(ctx context.Context, batchID string) (productionapp.DeductPreview, error) {
	return productionapp.DeductPreview{}, nil
}
func (r *workOrderAPIRepo) ConfirmDeduct(ctx context.Context, batchID, operator string) (productionapp.DeductConfirmResult, error) {
	return productionapp.DeductConfirmResult{}, nil
}
func (r *workOrderAPIRepo) ListRunning(ctx context.Context) ([]productionapp.RunningItem, error) {
	return nil, nil
}
func (r *workOrderAPIRepo) ListStartNeeds(ctx context.Context, cmd productionapp.StartCommand) ([]productionapp.StartNeed, error) {
	return nil, nil
}
func (r *workOrderAPIRepo) Start(ctx context.Context, cmd productionapp.StartExecutionCommand) (productionapp.StartResult, error) {
	return productionapp.StartResult{}, nil
}
func (r *workOrderAPIRepo) Finish(ctx context.Context, cmd productionapp.FinishCommand) (productionapp.FinishResult, error) {
	return productionapp.FinishResult{RunningItemID: cmd.ID}, nil
}
func (r *workOrderAPIRepo) Cancel(ctx context.Context, cmd productionapp.CancelCommand) error {
	return nil
}
func (r *workOrderAPIRepo) CreateProductionPlan(ctx context.Context, cmd productionapp.CreateProductionPlanCommand) (productionapp.ProductionPlanDetail, error) {
	r.createPlan = cmd
	if r.productionPlan.ID == 0 {
		r.productionPlan = productionapp.ProductionPlanDetail{
			ID:     41,
			PlanNo: "PP-0000000041",
			Status: "draft",
			Items:  []productionapp.ProductionPlanItem{{ID: 51, ProductID: 1, ProductName: "计划拼配", SpecG: 227, PlannedG: 600, GapG: 454, OrderNos: "SO-PLAN-1"}},
		}
	}
	return r.productionPlan, nil
}
func (r *workOrderAPIRepo) ListProductionPlans(ctx context.Context, query productionapp.ProductionPlanQuery) ([]productionapp.ProductionPlanRow, error) {
	r.productionPlanQuery = query
	return []productionapp.ProductionPlanRow{{ID: 41, PlanNo: "PP-0000000041", Status: "draft", ItemCount: 1}}, nil
}
func (r *workOrderAPIRepo) GetProductionPlan(ctx context.Context, id int64) (productionapp.ProductionPlanDetail, error) {
	if r.productionPlan.ID == 0 {
		r.productionPlan = productionapp.ProductionPlanDetail{ID: id, PlanNo: "PP-0000000041", Status: "draft"}
	}
	return r.productionPlan, nil
}
func (r *workOrderAPIRepo) SaveProductionPlanOperationSplits(ctx context.Context, cmd productionapp.SaveProductionPlanOperationSplitsCommand) ([]productionapp.ProductionPlanOperationSplit, error) {
	r.savePlanSplits = cmd
	if len(r.planSplits) == 0 {
		r.planSplits = cmd.Items
	}
	return r.planSplits, nil
}
func (r *workOrderAPIRepo) PreviewProductionPlanOperationSplits(ctx context.Context, cmd productionapp.PreviewProductionPlanOperationSplitsCommand) (productionapp.ProductionPlanOperationSplitPreview, error) {
	r.previewPlanSplits = cmd
	return productionapp.ProductionPlanOperationSplitPreview{
		CoverageSummary: productionapp.ProductionPlanOperationSplitCoverageSummary{
			RequiredG: 20000,
			ArrangedG: 12000,
			DiffG:     -8000,
			Status:    "short",
		},
		OperationCoverage: []productionapp.ProductionPlanOperationSplitCoverageRow{{
			ProductionPlanItemID: 51,
			ProductName:          "烘焙计划",
			OperationSeq:         10,
			Operation:            "烘焙",
			RequiredG:            20000,
			ArrangedG:            12000,
			DiffG:                -8000,
			Status:               "short",
		}},
		MaterialSummary: []productionapp.ProductionPlanOperationSplitMaterialPreview{{
			Name:        "孟连水洗A",
			Unit:        "g",
			RequiredQty: 10000,
			ArrangedQty: 6000,
			DiffQty:     -4000,
			Status:      "short",
		}},
	}, nil
}
func (r *workOrderAPIRepo) SaveWorkOrderOperationSplits(ctx context.Context, cmd productionapp.SaveWorkOrderOperationSplitsCommand) (productionapp.WorkOrderOperationSplitsResult, error) {
	r.saveWorkOrderSplits = cmd
	if len(r.workOrderSplitRows) == 0 && len(cmd.Items) > 0 {
		r.workOrderSplitRows = []productionapp.JobCardRow{{
			ID:                      91,
			WorkOrderID:             cmd.ID,
			SequenceNo:              1,
			Operation:               "烘焙",
			WorkstationCapacityID:   cmd.Items[0].WorkstationCapacityID,
			WorkstationCapacityName: "布勒 18kg",
			PlannedBatchCount:       4,
			Status:                  "pending",
		}}
	}
	return productionapp.WorkOrderOperationSplitsResult{
		WorkOrder: productionapp.WorkOrderRow{ID: cmd.ID, WorkOrderNo: "WO-PR493-001", Status: "released"},
		JobCards:  r.workOrderSplitRows,
	}, nil
}
func (r *workOrderAPIRepo) SubmitProductionPlan(ctx context.Context, cmd productionapp.SubmitProductionPlanCommand) (productionapp.ProductionPlanSubmitResult, error) {
	r.submitPlan = cmd
	r.submitPlans = append(r.submitPlans, cmd)
	if err := r.submitPlanErrByID[cmd.ID]; err != nil {
		return productionapp.ProductionPlanSubmitResult{}, err
	}
	if res, ok := r.submitPlanByID[cmd.ID]; ok {
		return res, nil
	}
	if r.submittedPlan.Plan.ID == 0 {
		r.submittedPlan = productionapp.ProductionPlanSubmitResult{
			Plan:       productionapp.ProductionPlanDetail{ID: cmd.ID, PlanNo: "PP-0000000041", Status: "submitted"},
			WorkOrders: []productionapp.WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "released", RunningItemID: 0}},
			JobCards:   []productionapp.JobCardRow{{ID: 91, WorkOrderID: 88, SequenceNo: 1, Operation: "烘焙", Workstation: "烘焙机", Status: "pending"}},
		}
	}
	return r.submittedPlan, nil
}
func (r *workOrderAPIRepo) CancelProductionPlan(ctx context.Context, cmd productionapp.CancelProductionPlanCommand) (productionapp.ProductionPlanDetail, error) {
	r.cancelPlan = cmd
	return productionapp.ProductionPlanDetail{
		ID:          cmd.ID,
		PlanNo:      "PP-0000000041",
		Status:      "cancelled",
		CancelledAt: "2026-07-26 12:00",
	}, nil
}
func (r *workOrderAPIRepo) StartWorkOrder(ctx context.Context, cmd productionapp.WorkOrderStartCommand) (productionapp.WorkOrderStartResult, error) {
	r.startWorkOrder = cmd
	if r.workOrderStarted.WorkOrder.ID == 0 {
		r.workOrderStarted = productionapp.WorkOrderStartResult{
			BatchID:       "BATCH-WO-88",
			RunningItemID: 99,
			WorkOrder:     productionapp.WorkOrderRow{ID: cmd.ID, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "running", RunningItemID: 99},
		}
	}
	return r.workOrderStarted, nil
}
func (r *workOrderAPIRepo) CompleteWorkOrder(ctx context.Context, cmd productionapp.WorkOrderCompleteCommand) (productionapp.WorkOrderCompleteResult, error) {
	r.completeWorkOrder = cmd
	if r.workOrderCompleted.WorkOrder.ID == 0 {
		r.workOrderCompleted = productionapp.WorkOrderCompleteResult{
			WorkOrder:    productionapp.WorkOrderRow{ID: cmd.ID, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "completed", RunningItemID: 99, ActualCost: 48.75},
			StockEntries: []productionapp.StockEntryRow{{ID: 7, EntryNo: "SE-0000000007", EntryType: "finished_receipt", WorkOrderID: cmd.ID, RunningItemID: 99, Status: "submitted"}},
			Cost:         productionapp.BatchCostRow{RunningItemID: 99, MaterialCost: 36.25, OperationCost: 12.5, TotalCost: 48.75},
		}
	}
	return r.workOrderCompleted, nil
}
func (r *workOrderAPIRepo) CancelWorkOrder(ctx context.Context, cmd productionapp.WorkOrderCancelCommand) (productionapp.WorkOrderRow, error) {
	r.cancelWorkOrder = cmd
	if r.workOrderCancelled.ID == 0 {
		r.workOrderCancelled = productionapp.WorkOrderRow{ID: cmd.ID, WorkOrderNo: "WO-PR497-001", Status: "cancelled"}
	}
	return r.workOrderCancelled, nil
}
func (r *workOrderAPIRepo) CreateStockEntry(ctx context.Context, cmd productionapp.StockEntryCommand) (productionapp.StockEntryDetail, error) {
	r.stockEntry = cmd
	return productionapp.StockEntryDetail{
		ID:            7,
		EntryNo:       "SE-0000000007",
		EntryType:     cmd.EntryType,
		Purpose:       cmd.Purpose,
		Status:        "submitted",
		WorkOrderID:   cmd.WorkOrderID,
		JobCardID:     cmd.JobCardID,
		RunningItemID: cmd.RunningItemID,
		Operator:      cmd.Operator,
		Items: []productionapp.StockEntryItemRow{{
			ID:            1,
			MaterialID:    cmd.Items[0].MaterialID,
			ItemType:      cmd.Items[0].ItemType,
			FromWarehouse: cmd.Items[0].FromWarehouse,
			ToWarehouse:   cmd.Items[0].ToWarehouse,
			QtyG:          cmd.Items[0].QtyG,
		}},
	}, nil
}
func (r *workOrderAPIRepo) ListStockEntries(ctx context.Context, query productionapp.StockEntryQuery) ([]productionapp.StockEntryRow, error) {
	r.stockEntryQuery = query
	if len(r.stockEntryRows) > 0 {
		return r.stockEntryRows, nil
	}
	return []productionapp.StockEntryRow{{ID: 7, EntryNo: "SE-0000000007", EntryType: query.EntryType, WorkOrderID: query.WorkOrderID, Status: "submitted"}}, nil
}
func (r *workOrderAPIRepo) GetStockEntry(ctx context.Context, id int64) (productionapp.StockEntryDetail, error) {
	r.stockEntryID = id
	return productionapp.StockEntryDetail{ID: id, EntryNo: "SE-0000000007", EntryType: "material_issue_to_wip", Purpose: "material_transfer_for_manufacture", Status: "submitted", Items: []productionapp.StockEntryItemRow{{ID: 1, MaterialID: 10, ItemType: "material", QtyG: 60000}}}, nil
}
func (r *workOrderAPIRepo) GetWorkOrderStockDocumentDraft(_ context.Context, _ int64, _ string, stockDocumentID int64) (*productionapp.StockEntryCommand, error) {
	r.stockDocumentDraftID = stockDocumentID
	if stockDocumentID > 0 && (r.stockDocumentDraft == nil || r.stockDocumentDraft.ID != stockDocumentID) {
		return nil, nil
	}
	return r.stockDocumentDraft, nil
}
func (r *workOrderAPIRepo) TransitionJobCard(ctx context.Context, cmd productionapp.JobCardActionCommand) (productionapp.JobCardActionResult, error) {
	r.jobCardAction = cmd
	status := "running"
	if cmd.Action == "pause" {
		status = "paused"
	}
	if cmd.Action == "complete" {
		status = "completed"
	}
	return productionapp.JobCardActionResult{
		JobCard:   productionapp.JobCardRow{ID: cmd.ID, WorkOrderID: 88, Status: status, ActualInputQty: cmd.ActualInputQty, ActualOutputQty: cmd.ActualOutputQty, ActualLossQty: cmd.ActualLossQty, ActualLossRate: cmd.ActualLossRate, Operator: cmd.Operator},
		WorkOrder: productionapp.WorkOrderRow{ID: 88, Status: "running"},
	}, nil
}
func (r *workOrderAPIRepo) ListMachines(ctx context.Context, activeOnly bool) ([]productionapp.RoastMachine, error) {
	return nil, nil
}
func (r *workOrderAPIRepo) SaveMachine(ctx context.Context, cmd productionapp.RoastMachineCommand) error {
	return nil
}
func (r *workOrderAPIRepo) PlanSummary(ctx context.Context, query productionapp.PlanSummaryQuery) (productionapp.PlanSummaryData, error) {
	return productionapp.PlanSummaryData{}, nil
}
func (r *workOrderAPIRepo) ListProductionLogs(ctx context.Context, query productionapp.ProductionLogsQuery) (productionapp.ProductionLogsResult, error) {
	r.productionLogsQuery = query
	return r.productionLogs, nil
}
func (r *workOrderAPIRepo) ListWorkOrders(ctx context.Context, query productionapp.WorkOrderQuery) ([]productionapp.WorkOrderRow, error) {
	r.workOrderQuery = query
	return r.rows, nil
}
func (r *workOrderAPIRepo) ListJobCards(ctx context.Context, query productionapp.JobCardQuery) ([]productionapp.JobCardRow, error) {
	r.jobCardQuery = query
	return r.jobCards, nil
}
func (r *workOrderAPIRepo) UpdateJobCardActuals(ctx context.Context, cmd productionapp.JobCardActualsCommand) error {
	r.jobCardActual = cmd
	return nil
}
func (r *workOrderAPIRepo) ListBatchCosts(ctx context.Context, query productionapp.BatchCostQuery) ([]productionapp.BatchCostRow, error) {
	r.batchCostQuery = query
	return r.batchCosts, nil
}
func (r *workOrderAPIRepo) ListWorkOrderLedgerEntries(ctx context.Context, query productionapp.WorkOrderLedgerQuery) ([]productionapp.WorkOrderLedgerEntryRow, error) {
	r.ledgerQuery = query
	return r.ledgerRows, nil
}
func (r *workOrderAPIRepo) MaterialPlan(ctx context.Context, query productionapp.MaterialPlanQuery) (productionapp.MaterialPlanResult, error) {
	return productionapp.MaterialPlanResult{}, nil
}
func (r *workOrderAPIRepo) MRPSuggestions(ctx context.Context, query productionapp.MRPSuggestionQuery) (productionapp.MRPSuggestionResult, error) {
	r.mrpQuery = query
	return productionapp.MRPSuggestionResult{
		Rows: []productionapp.MRPSuggestionRow{{
			MaterialID:             10,
			MaterialName:           "孟连水洗5T批次",
			Unit:                   "g",
			RequiredG:              60000,
			WIPG:                   10000,
			RawG:                   30000,
			ReservedG:              5000,
			AvailableG:             5000,
			WIPTransferSuggestionG: 30000,
			ShortageG:              25000,
			PurchaseSuggestionG:    25000,
			WorkOrderCount:         2,
			SourceWorkOrders:       "WO-PR482-001,WO-PR482-002",
			SuggestionType:         "purchase_suggestion",
		}},
		PurchaseSuggestionG: 25000,
		TransferSuggestionG: 30000,
	}, nil
}
func (r *workOrderAPIRepo) ProductionTraceAnalytics(ctx context.Context, query productionapp.ProductionTraceAnalyticsQuery) (productionapp.ProductionTraceAnalyticsResult, error) {
	r.traceQuery = query
	return productionapp.ProductionTraceAnalyticsResult{
		TraceLinks: []productionapp.ProductionTraceLinkRow{{
			WorkOrderID:   88,
			WorkOrderNo:   "WO-PR484-001",
			RunningItemID: 99,
			BatchID:       "BATCH-WO-88",
			JobCardID:     91,
			Operation:     "烘焙",
			StockEntryID:  7,
			EntryNo:       "SE-0000000007",
			EntryType:     "finished_receipt",
			MaterialID:    10,
			MaterialName:  "孟连水洗5T批次",
			QtyG:          45400,
		}},
		CostVariance: []productionapp.ProductionCostVarianceRow{{
			WorkOrderID:  88,
			WorkOrderNo:  "WO-PR484-001",
			ProductName:  "工厂量单商品",
			PlannedCost:  100,
			ActualCost:   118,
			Variance:     18,
			VarianceRate: 0.18,
		}},
		AbnormalLosses: []productionapp.ProductionAbnormalLossRow{{
			JobCardID:      91,
			WorkOrderID:    88,
			WorkOrderNo:    "WO-PR484-001",
			Operation:      "烘焙",
			ActualLossQty:  1200,
			ActualLossRate: 0.12,
			Severity:       "warning",
		}},
		TotalVariance:     18,
		AbnormalLossCount: 1,
	}, nil
}
func (r *workOrderAPIRepo) CreateQualityInspection(ctx context.Context, cmd productionapp.QualityInspectionCommand) (productionapp.QualityInspectionRow, error) {
	return productionapp.QualityInspectionRow{}, nil
}
func (r *workOrderAPIRepo) ListQualityInspections(ctx context.Context, query productionapp.QualityInspectionQuery) ([]productionapp.QualityInspectionRow, error) {
	r.qualityQuery = query
	return r.qualityRows, nil
}
func (r *workOrderAPIRepo) ListWIPReservations(ctx context.Context, query productionapp.WIPReservationQuery) (productionapp.WIPReservationResult, error) {
	r.reservationQuery = query
	return productionapp.WIPReservationResult{Rows: r.reservationRows}, nil
}
func (r *workOrderAPIRepo) AdjustWIPReservation(ctx context.Context, cmd productionapp.WIPReservationAdjustCommand) (productionapp.WIPReservationRow, error) {
	return productionapp.WIPReservationRow{}, nil
}
func (r *workOrderAPIRepo) ReleaseWIPReservations(ctx context.Context, cmd productionapp.WIPReservationReleaseCommand) (productionapp.WIPReservationReleaseResult, error) {
	return productionapp.WIPReservationReleaseResult{}, nil
}
func (r *workOrderAPIRepo) AcceptanceSmoke(ctx context.Context) (productionapp.AcceptanceSmokeResult, error) {
	return productionapp.AcceptanceSmokeResult{}, nil
}

func TestWorkOrderAPIIncludesRoastAdvice(t *testing.T) {
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(&workOrderAPIRepo{rows: []productionapp.WorkOrderRow{{
		WorkOrderNo:         "WO-0000000020",
		BatchID:             "A20260427-071539-6b",
		ProductName:         "橘皮乌龙",
		SpecG:               454,
		PlannedG:            12000,
		RoastLevel:          "浅烘",
		YieldRate:           0.82,
		SuggestedInputG:     12000,
		SuggestedMachine:    "样机",
		SuggestedBatchCount: 2,
		SuggestedBatchG:     6000,
		SuggestedBatchPlan:  "6kg x 2",
		PlannedUnits:        21,
		PlannedLooseG:       306,
		MaterialSummary:     "孟连水洗5T批次 100%",
		OrderNos:            "SO-1",
	}}}))

	req := httptest.NewRequest(http.MethodGet, "/api/produce/work-orders", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"work_order_no":"WO-0000000020"`,
		`"roast_level":"浅烘"`,
		`"yield_rate":0.82`,
		`"suggested_machine":"样机"`,
		`"suggested_batch_plan":"6kg x 2"`,
		`"material_summary":"孟连水洗5T批次 100%"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
}

func TestProductionWorkstationOverviewAPIAndStationActions(t *testing.T) {
	repo := &workOrderAPIRepo{
		rows: []productionapp.WorkOrderRow{{
			ID:            88,
			WorkOrderNo:   "WO-OVERVIEW-001",
			RunningItemID: 99,
			ProductName:   "桂花乌龙",
			SpecG:         227,
			PlannedG:      45400,
			PlannedUnits:  200,
			Status:        "running",
			OrderNos:      "SO-20260613-001",
			Priority:      8,
			WorkCenter:    "包装线",
			AssignedTo:    "生产主管",
		}},
		jobCards: []productionapp.JobCardRow{{
			ID:             91,
			WorkOrderID:    88,
			WorkOrderNo:    "WO-OVERVIEW-001",
			ProductName:    "桂花乌龙",
			SpecG:          227,
			OrderNos:       "SO-20260613-001",
			Operation:      "包装",
			Workstation:    "包装工位A",
			WorkCenter:     "包装线",
			Status:         "running",
			AssignedTo:     "阿强",
			Priority:       9,
			PlannedMinutes: 40,
			PlannedStartAt: "2026-06-13 09:00",
			PlannedOutputG: 45400,
		}, {
			ID:              92,
			WorkOrderID:     88,
			WorkOrderNo:     "WO-OVERVIEW-001",
			ProductName:     "桂花乌龙",
			SpecG:           227,
			Operation:       "贴标",
			Workstation:     "包装工位A",
			WorkCenter:      "包装线",
			Status:          "paused",
			AssignedTo:      "现场主管",
			Priority:        7,
			ExceptionReason: "包材未到位",
		}},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{Production: productionapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/production/workstation-overview?limit=999", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET workstation overview status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.workOrderQuery.Limit != 500 || repo.jobCardQuery.Limit != 500 {
		t.Fatalf("overview source queries not clamped: workOrder=%+v jobCard=%+v", repo.workOrderQuery, repo.jobCardQuery)
	}
	for _, want := range []string{
		`"total_tasks":2`,
		`"today_summary"`,
		`"nav_badges"`,
		`"productionOverview":{"pending":0,"blocked":1,"running":1}`,
		`"readiness":"running"`,
		`"readiness_label":"执行中"`,
		`"status_summary"`,
		`"workstation_load"`,
		`"current_task":"包装 / 桂花乌龙"`,
		`"next_handler":"现场主管"`,
		`"blocking_reason":"包材未到位"`,
		`"available_actions":["pause","complete","report_exception","material_call"]`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("overview response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/production/workstation/tasks/92/exception", strings.NewReader(`{"exception_reason":"温度异常"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.jobCardAction.ID != 92 || repo.jobCardAction.Action != "pause" || repo.jobCardAction.ExceptionReason != "温度异常" {
		t.Fatalf("POST exception status=%d body=%s action=%+v", rec.Code, rec.Body.String(), repo.jobCardAction)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/production/workstation/tasks/91/material-call", strings.NewReader(`{"note":"挂耳滤袋不足"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.jobCardAction.ID != 91 || repo.jobCardAction.Action != "pause" || repo.jobCardAction.ExceptionReason != "呼叫补料: 挂耳滤袋不足" {
		t.Fatalf("POST material-call status=%d body=%s action=%+v", rec.Code, rec.Body.String(), repo.jobCardAction)
	}
}

func TestManufacturingPhase2StockEntryAndExecutionAPIs(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "测试员")
			c.Set("actor", "测试员")
			return next(c)
		}
	})
	svc := productionapp.NewService(repo)
	registerStockEntryAPI(e, svc)
	registerWorkOrderAPI(e, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/stock-entries", strings.NewReader(`{
		"entry_type":"material_issue_to_wip",
		"work_order_id":88,
		"job_card_id":91,
		"note":"二期领料",
		"items":[{"material_id":10,"item_type":"material","item_name":"卡蒂姆水洗","from_warehouse":"raw_materials","to_warehouse":"wip","qty_g":60000}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/stock-entries status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.stockEntry.EntryType != "material_issue_to_wip" || repo.stockEntry.WorkOrderID != 88 || repo.stockEntry.JobCardID != 91 || len(repo.stockEntry.Items) != 1 {
		t.Fatalf("stock entry command = %+v", repo.stockEntry)
	}
	if !strings.Contains(rec.Body.String(), `"entry_no":"SE-0000000007"`) || !strings.Contains(rec.Body.String(), `"status":"submitted"`) {
		t.Fatalf("stock entry response = %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock-entries?entry_type=material_issue_to_wip&work_order_id=88", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.stockEntryQuery.EntryType != "material_issue_to_wip" || repo.stockEntryQuery.WorkOrderID != 88 || !strings.Contains(rec.Body.String(), `"rows"`) {
		t.Fatalf("GET /api/stock-entries status=%d body=%s query=%+v", rec.Code, rec.Body.String(), repo.stockEntryQuery)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock-entries/7", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.stockEntryID != 7 || !strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("GET /api/stock-entries/7 status=%d body=%s id=%d", rec.Code, rec.Body.String(), repo.stockEntryID)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/job-cards/91/start", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.jobCardAction.Action != "start" || repo.jobCardAction.ID != 91 {
		t.Fatalf("POST job card start status=%d body=%s action=%+v", rec.Code, rec.Body.String(), repo.jobCardAction)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/job-cards/91/pause", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.jobCardAction.Action != "pause" {
		t.Fatalf("POST job card pause status=%d body=%s action=%+v", rec.Code, rec.Body.String(), repo.jobCardAction)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/job-cards/91/resume", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.jobCardAction.Action != "resume" {
		t.Fatalf("POST job card resume status=%d body=%s action=%+v", rec.Code, rec.Body.String(), repo.jobCardAction)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/job-cards/91/complete", strings.NewReader(`{"actual_input_qty":600,"actual_output_qty":540,"exception_reason":"正常损耗"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.jobCardAction.Action != "complete" || repo.jobCardAction.ActualLossQty != 60 {
		t.Fatalf("POST job card complete status=%d body=%s action=%+v", rec.Code, rec.Body.String(), repo.jobCardAction)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/work-orders/88/complete", strings.NewReader(`{"finished_units":2,"finished_loose_g":10,"consumed_input_g":600,"warehouse":"finished_goods","note":"完工入库"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.completeWorkOrder.ID != 88 || repo.completeWorkOrder.FinishedUnits != 2 || repo.completeWorkOrder.Warehouse != "finished_goods" {
		t.Fatalf("POST work order complete status=%d body=%s command=%+v", rec.Code, rec.Body.String(), repo.completeWorkOrder)
	}
	if !strings.Contains(rec.Body.String(), `"stock_entries"`) || !strings.Contains(rec.Body.String(), `"total_cost":48.75`) {
		t.Fatalf("work order complete response missing stock/cost: %s", rec.Body.String())
	}
}

func TestStockDocumentPurposeAliasesUseERPNextFlowLanguage(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "仓管")
			c.Set("actor", "仓管")
			return next(c)
		}
	})
	registerStockEntryAPI(e, productionapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/stock-documents", strings.NewReader(`{
		"purpose":"material_transfer_for_manufacture",
		"work_order_id":88,
		"note":"按工单领料",
		"items":[{"material_id":10,"item_name":"孟连水洗","qty_g":60000}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/stock-documents status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.stockEntry.EntryType != "material_issue_to_wip" || repo.stockEntry.Purpose != "material_transfer_for_manufacture" || repo.stockEntry.SourceType != "work_order" || repo.stockEntry.SourceID != 88 {
		t.Fatalf("stock document command = %+v", repo.stockEntry)
	}
	for _, want := range []string{`"purpose":"material_transfer_for_manufacture"`, `"entry_type":"material_issue_to_wip"`, `"from_warehouse":"raw_materials"`, `"to_warehouse":"wip"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("stock document response missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock-documents?purpose=material_transfer_for_manufacture&work_order_id=88", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.stockEntryQuery.Purpose != "material_transfer_for_manufacture" || repo.stockEntryQuery.EntryType != "material_issue_to_wip" || repo.stockEntryQuery.WorkOrderID != 88 {
		t.Fatalf("GET /api/stock-documents status=%d body=%s query=%+v", rec.Code, rec.Body.String(), repo.stockEntryQuery)
	}
	if !strings.Contains(rec.Body.String(), `"purpose":"material_transfer_for_manufacture"`) {
		t.Fatalf("stock documents list should expose purpose: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/stock-documents/7", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.stockEntryID != 7 || !strings.Contains(rec.Body.String(), `"purpose":"material_transfer_for_manufacture"`) {
		t.Fatalf("GET /api/stock-documents/7 status=%d body=%s id=%d", rec.Code, rec.Body.String(), repo.stockEntryID)
	}
}

func TestUnifiedStockDocumentHTTPLifecycleKeepsDraftUnpostedAndIsIdempotent(t *testing.T) {
	productionRepo := &workOrderAPIRepo{}
	stockRepo := &stockDocumentAPIRepo{}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "仓管")
			c.Set("actor", "仓管")
			return next(c)
		}
	})
	registerStockEntryAPI(e, productionapp.NewService(productionRepo), stockapp.NewService(stockRepo))
	for _, retiredBody := range []string{
		`{"purpose":"material_receipt","items":[{"material_id":10,"item_type":"material","qty_g":1000}]}`,
		`{"entry_type":"material_receipt","items":[{"material_id":10,"item_type":"material","qty_g":1000}]}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/stock-documents", strings.NewReader(retiredBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "采购入库") {
			t.Fatalf("retired material receipt status=%d body=%s", rec.Code, rec.Body.String())
		}
	}
	if stockRepo.createCount != 0 {
		t.Fatalf("retired material receipt created %d drafts", stockRepo.createCount)
	}

	body := `{
		"purpose":"material_transfer_for_manufacture",
		"work_order_id":88,
		"note":"按工单领料",
		"items":[{"material_id":10,"item_type":"material","qty_g":1500}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/stock-documents", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || stockRepo.createCount != 1 || stockRepo.submitCount != 0 || !strings.Contains(rec.Body.String(), `"status":"draft"`) {
		t.Fatalf("create draft status=%d body=%s counts=%d/%d", rec.Code, rec.Body.String(), stockRepo.createCount, stockRepo.submitCount)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/stock-documents/31", strings.NewReader(strings.Replace(body, "按工单领料", "复核后领料", 1)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || stockRepo.updateCount != 1 || !strings.Contains(rec.Body.String(), "复核后领料") {
		t.Fatalf("update draft status=%d body=%s count=%d", rec.Code, rec.Body.String(), stockRepo.updateCount)
	}

	for i := 0; i < 2; i++ {
		req = httptest.NewRequest(http.MethodPost, "/api/stock-documents/31/submit", nil)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"submitted"`) {
			t.Fatalf("submit %d status=%d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	if stockRepo.submitCount != 1 {
		t.Fatalf("submit count=%d, want one posting", stockRepo.submitCount)
	}

	for i := 0; i < 2; i++ {
		req = httptest.NewRequest(http.MethodPost, "/api/stock-documents/31/cancel", nil)
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"status":"cancelled"`) {
			t.Fatalf("cancel %d status=%d body=%s", i+1, rec.Code, rec.Body.String())
		}
	}
	if stockRepo.cancelCount != 1 {
		t.Fatalf("cancel count=%d, want one reverse posting", stockRepo.cancelCount)
	}
}

func TestWorkOrderStockDocumentPreviewUsesPhysicalWIPGapAndReturnableBalance(t *testing.T) {
	repo := &workOrderAPIRepo{
		rows: []productionapp.WorkOrderRow{{
			ID: 88, WorkOrderNo: "WO-0000000088", Status: "running", RunningItemID: 99,
			ProductID: 9, ProductName: "测试熟豆", SpecG: 1000, PlannedUnits: 5,
		}},
		reservationRows: []productionapp.WIPReservationRow{{
			ID: 1, WorkOrderID: 88, WorkOrderNo: "WO-0000000088", RunningItemID: 99,
			MaterialID: 10, MaterialName: "测试生豆", RequiredG: 6000, ReservedG: 6000,
			ConsumedG: 1000, ReturnedG: 500, RemainingReservedG: 4500,
		}},
		ledgerRows: []productionapp.WorkOrderLedgerEntryRow{{
			ID: 1, StockEntryID: 7, EntryNo: "SE-0000000007", EntryType: "material_issue_to_wip",
			Purpose: "material_transfer_for_manufacture", ItemType: "material", ItemID: 10,
			ItemName: "测试生豆", Warehouse: "wip", QtyChangeG: 3000,
		}},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "仓管")
			return next(c)
		}
	})
	registerWorkOrderAPI(e, productionapp.NewService(repo))

	assertPreview := func(action string, wants ...string) {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, "/api/produce/work-orders/88/stock-document-preview", strings.NewReader(fmt.Sprintf(`{"action":%q}`, action)))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("preview %s status=%d body=%s", action, rec.Code, rec.Body.String())
		}
		for _, want := range wants {
			if !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("preview %s missing %s: %s", action, want, rec.Body.String())
			}
		}
	}
	assertPreview("issue", `"qty_g":1500`, `"from_warehouse":"raw_materials"`, `"to_warehouse":"wip"`)
	assertPreview("return", `"qty_g":3000`, `"is_return":true`, `"from_warehouse":"wip"`, `"to_warehouse":"raw_materials"`)
}

func TestWorkOrderStockDocumentPreviewAPIPreservesBulkDraftAndReportsSuggestion(t *testing.T) {
	repo := &workOrderAPIRepo{
		rows: []productionapp.WorkOrderRow{{
			ID: 88, WorkOrderNo: "WO-PR560-001", Status: "released",
		}},
		reservationRows: []productionapp.WIPReservationRow{{
			ID: 1, WorkOrderID: 88, WorkOrderNo: "WO-PR560-001",
			MaterialID: 10, MaterialName: "如目达摩生豆",
			InventoryUnit: "g", QuantityBasis: "weight",
			RequiredQty: 7751, ShortageQty: 7751,
			RequiredG: 7751, RemainingReservedG: 7751,
		}},
		stockDocumentDraft: &productionapp.StockEntryCommand{
			ID: 71, EntryNo: "SE-0000000071", Status: "draft",
			Purpose: "material_transfer_for_manufacture", WorkOrderID: 88,
			Items: []productionapp.StockEntryItemCommand{{
				MaterialID: 10, ItemName: "如目达摩生豆", InventoryUnit: "g", QtyG: 8000,
			}},
		},
	}
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/produce/work-orders/88/stock-document-preview", strings.NewReader(`{"action":"issue","stock_document_id":71}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	var preview productionapp.StockDocumentPreview
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Document.Items) != 1 ||
		preview.Document.Items[0].QtyG != 8000 ||
		preview.Document.Items[0].RemainingQty != 7751 ||
		preview.Document.Items[0].DefaultQty != 8000 {
		t.Fatalf("preview document = %+v", preview.Document)
	}
	if len(preview.Warnings) != 1 ||
		!strings.Contains(preview.Warnings[0], "当前建议领用7751g") ||
		!strings.Contains(preview.Warnings[0], "草稿保留8000g") ||
		!strings.Contains(preview.Warnings[0], "超出部分提交后作为可用 WIP 库存保留") {
		t.Fatalf("preview warnings = %+v", preview.Warnings)
	}
	if repo.stockDocumentDraft.Items[0].QtyG != 8000 {
		t.Fatalf("preview must not persistently mutate draft: %+v", repo.stockDocumentDraft.Items[0])
	}
	if repo.stockDocumentDraftID != 71 {
		t.Fatalf("requested stock document id=%d, want 71", repo.stockDocumentDraftID)
	}
}

func TestWorkOrderStockDocumentPreviewAcceptsDraftIDAlias(t *testing.T) {
	repo := &workOrderAPIRepo{
		rows: []productionapp.WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR561-001", Status: "released"}},
		reservationRows: []productionapp.WIPReservationRow{{
			WorkOrderID: 88, MaterialID: 10, MaterialName: "哥伦比亚",
			InventoryUnit: "g", QuantityBasis: "weight", RequiredQty: 1974, ShortageQty: 1974,
			RequiredG: 1974, ShortageG: 1974,
		}},
		stockDocumentDraft: &productionapp.StockEntryCommand{
			ID: 72, EntryNo: "SE-0000000072", Status: "draft",
			Purpose: "material_transfer_for_manufacture", WorkOrderID: 88,
			Items: []productionapp.StockEntryItemCommand{{
				MaterialID: 10, ItemName: "哥伦比亚", InventoryUnit: "g", QtyG: 60000,
			}},
		},
	}
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(repo))
	req := httptest.NewRequest(http.MethodPost, "/api/produce/work-orders/88/stock-document-preview", strings.NewReader(`{"action":"issue","draft_id":72}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.stockDocumentDraftID != 72 {
		t.Fatalf("requested draft alias id=%d, want 72", repo.stockDocumentDraftID)
	}
}

func TestWorkOrderStockDocumentPreviewRejectsAnotherDraftID(t *testing.T) {
	repo := &workOrderAPIRepo{
		rows: []productionapp.WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PR561-001", Status: "released"}},
		reservationRows: []productionapp.WIPReservationRow{{
			WorkOrderID: 88, MaterialID: 10, MaterialName: "哥伦比亚",
			InventoryUnit: "g", QuantityBasis: "weight", RequiredQty: 1974, ShortageQty: 1974,
			RequiredG: 1974, ShortageG: 1974,
		}},
		stockDocumentDraft: &productionapp.StockEntryCommand{
			ID: 72, EntryNo: "SE-0000000072", Status: "draft",
			Purpose: "material_transfer_for_manufacture", WorkOrderID: 88,
		},
	}
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(repo))
	req := httptest.NewRequest(http.MethodPost, "/api/produce/work-orders/88/stock-document-preview", strings.NewReader(`{"action":"issue","stock_document_id":71}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "指定库存草稿不存在") {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.stockDocumentDraftID != 71 {
		t.Fatalf("requested draft id=%d, want 71", repo.stockDocumentDraftID)
	}
}

func TestWorkOrderProducePathOwnsInventoryActionsAndDetail(t *testing.T) {
	repo := &workOrderAPIRepo{
		rows: []productionapp.WorkOrderRow{{
			ID:            88,
			WorkOrderNo:   "WO-PR497-001",
			RunningItemID: 99,
			ProductName:   "桂花乌龙",
			Status:        "running",
		}},
		jobCards:        []productionapp.JobCardRow{{ID: 91, WorkOrderID: 88, WorkOrderNo: "WO-PR497-001", Operation: "烘焙", Status: "running"}},
		reservationRows: []productionapp.WIPReservationRow{{ID: 11, WorkOrderID: 88, WorkOrderNo: "WO-PR497-001", RunningItemID: 99, MaterialID: 10, MaterialName: "孟连水洗", ReservedG: 60000, RemainingReservedG: 45000}},
		stockEntryRows:  []productionapp.StockEntryRow{{ID: 7, EntryNo: "SE-0000000007", EntryType: "material_issue_to_wip", Purpose: "material_transfer_for_manufacture", WorkOrderID: 88, RunningItemID: 99, Status: "submitted"}},
		ledgerRows:      []productionapp.WorkOrderLedgerEntryRow{{ID: 21, StockEntryID: 7, EntryNo: "SE-0000000007", Purpose: "material_transfer_for_manufacture", ItemType: "material", ItemID: 10, Warehouse: "wip", QtyChangeG: 60000}},
		qualityRows:     []productionapp.QualityInspectionRow{{ID: 3, Scope: "work_order", ReferenceNo: "WO-PR497-001", Result: "hold", Note: "待复核"}},
		productionLogs:  productionapp.ProductionLogsResult{Rows: []productionapp.ProductionLogRow{{ID: 31, BatchID: "BATCH-WO-88", InputG: 60000, FinishedTotalG: 45400}}},
		batchCosts:      []productionapp.BatchCostRow{{RunningItemID: 99, BatchID: "BATCH-WO-88", TotalCost: 48.75}},
	}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("employee_id", int64(1))
			c.Set("operator_employee", "主管")
			c.Set("actor", "主管")
			return next(c)
		}
	})
	svc := productionapp.NewService(repo)
	registerStockEntryAPI(e, svc)
	registerWorkOrderAPI(e, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/produce/work-orders/88/start", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.startWorkOrder.ID != 88 || !strings.Contains(rec.Body.String(), `"running_item_id":99`) {
		t.Fatalf("POST /api/produce/work-orders/88/start status=%d body=%s command=%+v", rec.Code, rec.Body.String(), repo.startWorkOrder)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/produce/work-orders/88/issue-materials", strings.NewReader(`{
		"note":"工单领料",
		"items":[{"material_id":10,"item_name":"孟连水洗","qty_g":60000}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.stockEntry.EntryType != "material_issue_to_wip" || repo.stockEntry.Purpose != "material_transfer_for_manufacture" || repo.stockEntry.WorkOrderID != 88 {
		t.Fatalf("POST issue-materials status=%d body=%s command=%+v", rec.Code, rec.Body.String(), repo.stockEntry)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/produce/work-orders/88/complete", strings.NewReader(`{"finished_units":2,"finished_loose_g":10,"consumed_input_g":600,"note":"完工入库"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.completeWorkOrder.ID != 88 || repo.completeWorkOrder.FinishedUnits != 2 || repo.completeWorkOrder.Warehouse != "" {
		t.Fatalf("POST /api/produce/work-orders/88/complete status=%d body=%s command=%+v", rec.Code, rec.Body.String(), repo.completeWorkOrder)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/produce/work-orders/88/cancel", strings.NewReader(`{"note":"计划取消"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.cancelWorkOrder.ID != 88 || repo.cancelWorkOrder.Note != "计划取消" || !strings.Contains(rec.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("POST /api/produce/work-orders/88/cancel status=%d body=%s command=%+v", rec.Code, rec.Body.String(), repo.cancelWorkOrder)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/produce/work-orders/88", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/work-orders/88 status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"work_order"`,
		`"materials"`,
		`"job_cards"`,
		`"stock_documents"`,
		`"stock_entries"`,
		`"ledger_entries"`,
		`"production_logs"`,
		`"cost_summary"`,
		`"execution_hub"`,
		`"readiness"`,
		`"can_start"`,
		`"can_complete"`,
		`"blocking_reasons"`,
		`"next_handler"`,
		`"suggested_action"`,
		`"related_links"`,
		`"operation_progress"`,
		`"wip_status"`,
		`"quality_status"`,
		`"trace_timeline"`,
		`"context_actions"`,
		`"purpose":"material_transfer_for_manufacture"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("work order detail response missing %s: %s", want, body)
		}
	}
	if repo.workOrderQuery.ID != 88 || repo.jobCardQuery.WorkOrderID != 88 || repo.reservationQuery.WorkOrderNo != "WO-PR497-001" || repo.stockEntryQuery.WorkOrderID != 88 || repo.ledgerQuery.WorkOrderID != 88 || repo.ledgerQuery.RunningItemID != 99 || repo.productionLogsQuery.RunningItemID != 99 || repo.batchCostQuery.RunningItemID != 99 {
		t.Fatalf("detail queries workOrder=%+v jobCard=%+v reservations=%+v stock=%+v ledger=%+v logs=%+v costs=%+v", repo.workOrderQuery, repo.jobCardQuery, repo.reservationQuery, repo.stockEntryQuery, repo.ledgerQuery, repo.productionLogsQuery, repo.batchCostQuery)
	}
}

func TestProductionPlanOperationSplitAPIReadsAndSavesDraftCapacitySplits(t *testing.T) {
	repo := &workOrderAPIRepo{
		productionPlan: productionapp.ProductionPlanDetail{
			ID:     41,
			PlanNo: "PP-0000000041",
			Status: "draft",
			Items:  []productionapp.ProductionPlanItem{{ID: 51, ProductName: "烘焙计划", PlannedG: 98000}},
			OperationSplits: []productionapp.ProductionPlanOperationSplit{{
				ID:                      1,
				ProductionPlanID:        41,
				ProductionPlanItemID:    51,
				OperationSeq:            10,
				Operation:               "烘焙",
				WorkstationCapacityID:   8,
				WorkstationCapacityName: "布勒 18kg",
				CostMethod:              "piece",
				PieceRate:               0.5,
				PlannedBatchCount:       5,
				PlannedQtyG:             90000,
				PlannedMinutes:          75,
				PlannedOperationCost:    375,
			}},
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Production: productionapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/production-plans/41/operation-splits", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET operation splits status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"operation":"烘焙"`, `"workstation_capacity_name":"布勒 18kg"`, `"cost_method":"piece"`, `"piece_rate":0.5`, `"planned_batch_count":5`, `"planned_qty_g":90000`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("GET operation splits missing %s: %s", want, rec.Body.String())
		}
	}

	req = httptest.NewRequest(http.MethodPost, "/api/production-plans/41/operation-splits", strings.NewReader(`{"items":[
		{"production_plan_item_id":51,"operation_seq":10,"operation":"烘焙","workstation_capacity_id":8,"planned_qty":90,"cost_method":"piece","piece_rate":0.5},
		{"production_plan_item_id":51,"operation_seq":10,"operation":"烘焙","workstation_capacity_id":9,"planned_qty":8}
	]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST operation splits status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.savePlanSplits.ID != 41 || repo.savePlanSplits.Operator == "" || len(repo.savePlanSplits.Items) != 2 {
		t.Fatalf("save split command = %+v", repo.savePlanSplits)
	}
	if repo.savePlanSplits.Items[0].WorkstationCapacityID != 8 || repo.savePlanSplits.Items[1].PlannedQty != 8 {
		t.Fatalf("saved split items = %+v", repo.savePlanSplits.Items)
	}
	if repo.savePlanSplits.Items[0].CostMethod != "piece" || repo.savePlanSplits.Items[0].PieceRate != 0.5 {
		t.Fatalf("piece cost fields = %+v", repo.savePlanSplits.Items[0])
	}
}

func TestProductionPlanOperationSplitPreviewAPIReturnsDemandGapWithoutSaving(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Production: productionapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/production-plans/41/operation-splits/preview", strings.NewReader(`{"items":[
		{"production_plan_item_id":51,"operation_seq":10,"operation":"烘焙","workstation_capacity_id":8,"planned_qty":12}
	]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST operation split preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.previewPlanSplits.ID != 41 || len(repo.previewPlanSplits.Items) != 1 {
		t.Fatalf("preview split command = %+v", repo.previewPlanSplits)
	}
	if len(repo.savePlanSplits.Items) != 0 {
		t.Fatalf("preview endpoint must not save splits, save command = %+v", repo.savePlanSplits)
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"coverage_summary"`,
		`"required_g":20000`,
		`"arranged_g":12000`,
		`"diff_g":-8000`,
		`"status":"short"`,
		`"operation_coverage"`,
		`"material_summary"`,
		`"required_qty":10000`,
		`"arranged_qty":6000`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("preview response missing %s: %s", want, body)
		}
	}
}

func TestWorkOrderOperationSplitAPISavesReleasedWorkOrderCapacitySplits(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/work-orders/88/operation-splits", strings.NewReader(`{"items":[{
		"operation_seq":1,
		"operation_id":7,
		"operation":"烘焙",
		"workstation_capacity_id":5,
		"planned_qty":72
	}]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || repo.saveWorkOrderSplits.ID != 88 || len(repo.saveWorkOrderSplits.Items) != 1 {
		t.Fatalf("POST work order operation splits status=%d body=%s command=%+v", rec.Code, rec.Body.String(), repo.saveWorkOrderSplits)
	}
	if got := repo.saveWorkOrderSplits.Items[0]; got.WorkstationCapacityID != 5 || got.PlannedQty != 72 || got.ProductionPlanItemID != 0 {
		t.Fatalf("work order split item = %+v, want capacity 5 planned qty 72 without production plan item requirement", got)
	}
	if !strings.Contains(rec.Body.String(), `"job_cards"`) {
		t.Fatalf("response body = %s, want rebuilt job cards", rec.Body.String())
	}
}

func TestProductionPlanAPICreatesListsAndSubmitsFormalPlan(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	registerProductionPlanAPI(e, productionapp.NewService(repo))

	createBody := `{"from":"2026-06-11","to":"2026-06-12","selected":["1-227"],"source_type":"erp_order"}`
	req := httptest.NewRequest(http.MethodPost, "/api/production-plans", strings.NewReader(createBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/production-plans status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.createPlan.InputByKey == nil || len(repo.createPlan.InputByKey) != 0 || !repo.createPlan.Selected["1-227"] || repo.createPlan.SourceType != "erp_order" {
		t.Fatalf("create plan command = %+v", repo.createPlan)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"status":"draft"`) || !strings.Contains(body, `"plan_no":"PP-0000000041"`) {
		t.Fatalf("create plan response = %s, want draft plan", body)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/production-plans", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"plan_no":"PP-0000000041"`) {
		t.Fatalf("GET /api/production-plans status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/api/production-plans/41/submit", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/production-plans/41/submit status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"status":"submitted"`, `"status":"released"`, `"status":"pending"`, `"work_order_no":"WO-PP-0000000041-0000000051"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("submit response missing %s: %s", want, body)
		}
	}
	if repo.submitPlan.ID != 41 {
		t.Fatalf("submit command = %+v, want id 41", repo.submitPlan)
	}
}

func TestProductionPlanAPIListAcceptsStatusAndTimeFilters(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	registerProductionPlanAPI(e, productionapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/production-plans?status=submitted&time_field=submitted_at&from=2026-06-01&to=2026-06-11&limit=50", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/production-plans status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.productionPlanQuery.Status != "submitted" ||
		repo.productionPlanQuery.TimeField != "submitted_at" ||
		repo.productionPlanQuery.From != "2026-06-01" ||
		repo.productionPlanQuery.To != "2026-06-11" ||
		repo.productionPlanQuery.Limit != 50 {
		t.Fatalf("production plan query = %+v, want status and submitted_at date filters", repo.productionPlanQuery)
	}
}

func TestProductionPlanAPIDetailIncludesDocumentSummary(t *testing.T) {
	repo := &workOrderAPIRepo{
		productionPlan: productionapp.ProductionPlanDetail{
			ID:          41,
			PlanNo:      "PP-0000000041",
			SourceType:  "erp_order",
			Status:      "submitted",
			CreatedBy:   "计划员",
			CreatedAt:   "2026-06-12 10:00",
			SubmittedBy: "主管",
			SubmittedAt: "2026-06-12 10:10",
			Items: []productionapp.ProductionPlanItem{{
				ID:                  51,
				PlanID:              41,
				ProductName:         "包装盒",
				SpecG:               0,
				PlannedG:            3000,
				PlannedOutputG:      100,
				GapG:                100,
				OrderNos:            "SO-BOX-1",
				BomVersionID:        701,
				ProcessRouteID:      801,
				ProcessSnapshotJSON: `{"name":"包装盒路线","operations":[{"seq":1,"operation":"印刷","workstation":"印刷工位"}]}`,
			}},
			MaterialSummary: []productionapp.MaterialNeed{{
				Name: "纸板",
				Qty:  100,
				Unit: "张",
			}},
			RelatedWorkOrders: []productionapp.ProductionPlanRelatedWorkOrder{{
				ID:                   88,
				WorkOrderNo:          "WO-PP-0000000041-0000000051",
				ProductionPlanID:     41,
				ProductionPlanItemID: 51,
				ProductName:          "包装盒",
				PlannedG:             3000,
				PlannedOutputG:       100,
				Status:               "released",
				JobCardCount:         2,
			}},
			JobCardCount: 2,
		},
	}
	e := echo.New()
	registerProductionPlanAPI(e, productionapp.NewService(repo))

	req := httptest.NewRequest(http.MethodGet, "/api/production-plans/41", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/production-plans/41 status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`"plan_no":"PP-0000000041"`,
		`"material_summary":[`,
		`"name":"纸板"`,
		`"related_work_orders":[`,
		`"work_order_no":"WO-PP-0000000041-0000000051"`,
		`"job_card_count":2`,
		`"process_route_id":801`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("production plan detail missing %s: %s", want, body)
		}
	}
}

func TestProductionPlanAPIBatchSubmitReportsPartialResults(t *testing.T) {
	repo := &workOrderAPIRepo{
		submitPlanByID: map[int64]productionapp.ProductionPlanSubmitResult{
			41: {
				Plan:       productionapp.ProductionPlanDetail{ID: 41, PlanNo: "PP-0000000041", Status: "submitted"},
				WorkOrders: []productionapp.WorkOrderRow{{ID: 88, WorkOrderNo: "WO-PP-0000000041-0000000051", Status: "released"}},
				JobCards:   []productionapp.JobCardRow{{ID: 91, WorkOrderID: 88, Status: "pending"}},
			},
		},
		submitPlanErrByID: map[int64]error{
			42: fmt.Errorf("production plan must be draft to submit"),
		},
	}
	e := echo.New()
	registerProductionPlanAPI(e, productionapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/production-plans/submit", strings.NewReader(`{"ids":[41,42]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/production-plans/submit status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"work_order_count":1`, `"job_card_count":1`, `"plan_no":"PP-0000000041"`, `"id":42`, `"production plan must be draft to submit"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("batch submit response missing %s: %s", want, body)
		}
	}
	if len(repo.submitPlans) != 2 || repo.submitPlans[0].ID != 41 || repo.submitPlans[1].ID != 42 {
		t.Fatalf("batch submit commands = %+v, want 41 then 42", repo.submitPlans)
	}
}

func TestWorkOrderStartAPIStartsReleasedWorkOrder(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(repo))

	req := httptest.NewRequest(http.MethodPost, "/api/work-orders/88/start", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/work-orders/88/start status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"ok":true`, `"running_item_id":99`, `"status":"running"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("start work order response missing %s: %s", want, body)
		}
	}
	if repo.startWorkOrder.ID != 88 {
		t.Fatalf("start command = %+v, want work order id 88", repo.startWorkOrder)
	}
}

func TestWorkOrderAPIIncludesExpectedLossAndOperationSummary(t *testing.T) {
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(&workOrderAPIRepo{rows: []productionapp.WorkOrderRow{{
		WorkOrderNo:          "WO-0000000020",
		YieldRate:            0.82,
		ExpectedYieldRate:    0.82,
		ExpectedLossRate:     0.18,
		OperationSummaryJSON: `{"actual_loss_qty":185,"actual_loss_rate":0.185}`,
	}}}))

	req := httptest.NewRequest(http.MethodGet, "/api/produce/work-orders", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"expected_yield_rate":0.82`, `"expected_loss_rate":0.18`, `"operation_summary_json":"{\"actual_loss_qty\":185,\"actual_loss_rate\":0.185}"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
}

func TestJobCardAPIIncludesActualLossFields(t *testing.T) {
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(&workOrderAPIRepo{jobCards: []productionapp.JobCardRow{{
		ID:               9,
		WorkOrderID:      20,
		WorkOrderNo:      "WO-PR491-001",
		ProductID:        539,
		ProductName:      "PR491 商品",
		SpecG:            454,
		OrderNos:         "SO-PR491-001",
		PlannedG:         1000,
		PlannedOutputG:   908,
		BomVersionID:     723,
		MaterialSnapshot: `[{"material_name":"孟连水洗","unit":"g","ratio_pct":70},{"material_name":"包装袋","unit":"unit","qty_per_unit":1}]`,
		Operation:        "cutting",
		PlannedInputQty:  1000,
		ActualInputQty:   1000,
		ActualOutputQty:  815,
		ActualLossQty:    185,
		ActualLossRate:   0.185,
		ExceptionReason:  "裁剪边角料",
		MetricsJSON:      `{"fabric":"cotton"}`,
	}}}))

	req := httptest.NewRequest(http.MethodGet, "/api/produce/job-cards", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"planned_input_qty":1000`, `"actual_input_qty":1000`, `"actual_output_qty":815`, `"actual_loss_qty":185`, `"actual_loss_rate":0.185`, `"exception_reason":"裁剪边角料"`, `"metrics_json":"{\"fabric\":\"cotton\"}"`, `"work_order_no":"WO-PR491-001"`, `"product_name":"PR491 商品"`, `"bom_version_id":723`, `"material_snapshot":"[{\"material_name\":\"孟连水洗\"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("response missing %s: %s", want, body)
		}
	}
}

func TestJobCardAPIUpdatesActualLossFields(t *testing.T) {
	repo := &workOrderAPIRepo{}
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(repo))

	body := `{"actual_input_qty":1000,"actual_output_qty":815,"exception_reason":"裁剪边角料","metrics_json":{"fabric":"cotton"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/produce/job-cards/9/actuals", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.jobCardActual.ID != 9 {
		t.Fatalf("job card id = %d, want 9", repo.jobCardActual.ID)
	}
	if repo.jobCardActual.ActualLossQty != 185 || repo.jobCardActual.ActualLossRate != 0.185 {
		t.Fatalf("actual loss = %.3f %.4f, want 185 and 0.185", repo.jobCardActual.ActualLossQty, repo.jobCardActual.ActualLossRate)
	}
	if repo.jobCardActual.ExceptionReason != "裁剪边角料" {
		t.Fatalf("exception reason = %q", repo.jobCardActual.ExceptionReason)
	}
	if !strings.Contains(repo.jobCardActual.MetricsJSON, `"fabric":"cotton"`) {
		t.Fatalf("metrics json = %s", repo.jobCardActual.MetricsJSON)
	}
}

func TestJobCardMetricsAliasAcceptsStringMetricsAndReturnsRow(t *testing.T) {
	repo := &workOrderAPIRepo{jobCards: []productionapp.JobCardRow{{
		ID:                  9,
		WorkOrderID:         20,
		SequenceNo:          2,
		Operation:           "裁剪",
		RecordsLoss:         true,
		ParameterSchemaJSON: `{"fabric":"text"}`,
		ActualLossQty:       185,
		ActualLossRate:      0.185,
	}}}
	e := echo.New()
	registerWorkOrderAPI(e, productionapp.NewService(repo))

	body := `{"actual_input_qty":1000,"actual_output_qty":815,"exception_reason":"裁剪边角料","metrics_json":"{\"fabric\":\"cotton\"}"}`
	req := httptest.NewRequest(http.MethodPost, "/api/produce/job-cards/9/metrics", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if repo.jobCardActual.MetricsJSON != `{"fabric":"cotton"}` {
		t.Fatalf("metrics json = %s", repo.jobCardActual.MetricsJSON)
	}
	for _, want := range []string{`"sequence_no":2`, `"records_loss":true`, `"parameter_schema_json":"{\"fabric\":\"text\"}"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("response missing %s: %s", want, rec.Body.String())
		}
	}
}
