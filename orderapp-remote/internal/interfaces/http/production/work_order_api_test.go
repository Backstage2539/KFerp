package production

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type workOrderAPIRepo struct {
	rows          []productionapp.WorkOrderRow
	jobCards      []productionapp.JobCardRow
	jobCardActual productionapp.JobCardActualsCommand
	jobCardAction productionapp.JobCardActionCommand

	createPlan          productionapp.CreateProductionPlanCommand
	submitPlan          productionapp.SubmitProductionPlanCommand
	submitPlans         []productionapp.SubmitProductionPlanCommand
	startWorkOrder      productionapp.WorkOrderStartCommand
	completeWorkOrder   productionapp.WorkOrderCompleteCommand
	productionPlanQuery productionapp.ProductionPlanQuery
	productionPlan      productionapp.ProductionPlanDetail
	submittedPlan       productionapp.ProductionPlanSubmitResult
	submitPlanByID      map[int64]productionapp.ProductionPlanSubmitResult
	submitPlanErrByID   map[int64]error
	workOrderStarted    productionapp.WorkOrderStartResult
	workOrderCompleted  productionapp.WorkOrderCompleteResult
	stockEntry          productionapp.StockEntryCommand
	stockEntryQuery     productionapp.StockEntryQuery
	stockEntryID        int64
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
func (r *workOrderAPIRepo) CreateStockEntry(ctx context.Context, cmd productionapp.StockEntryCommand) (productionapp.StockEntryDetail, error) {
	r.stockEntry = cmd
	return productionapp.StockEntryDetail{
		ID:            7,
		EntryNo:       "SE-0000000007",
		EntryType:     cmd.EntryType,
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
	return []productionapp.StockEntryRow{{ID: 7, EntryNo: "SE-0000000007", EntryType: query.EntryType, WorkOrderID: query.WorkOrderID, Status: "submitted"}}, nil
}
func (r *workOrderAPIRepo) GetStockEntry(ctx context.Context, id int64) (productionapp.StockEntryDetail, error) {
	r.stockEntryID = id
	return productionapp.StockEntryDetail{ID: id, EntryNo: "SE-0000000007", EntryType: "material_issue_to_wip", Status: "submitted", Items: []productionapp.StockEntryItemRow{{ID: 1, MaterialID: 10, ItemType: "material", QtyG: 60000}}}, nil
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
	return productionapp.ProductionLogsResult{}, nil
}
func (r *workOrderAPIRepo) ListWorkOrders(ctx context.Context, query productionapp.WorkOrderQuery) ([]productionapp.WorkOrderRow, error) {
	return r.rows, nil
}
func (r *workOrderAPIRepo) ListJobCards(ctx context.Context, query productionapp.JobCardQuery) ([]productionapp.JobCardRow, error) {
	return r.jobCards, nil
}
func (r *workOrderAPIRepo) UpdateJobCardActuals(ctx context.Context, cmd productionapp.JobCardActualsCommand) error {
	r.jobCardActual = cmd
	return nil
}
func (r *workOrderAPIRepo) ListBatchCosts(ctx context.Context, query productionapp.BatchCostQuery) ([]productionapp.BatchCostRow, error) {
	return nil, nil
}
func (r *workOrderAPIRepo) MaterialPlan(ctx context.Context, query productionapp.MaterialPlanQuery) (productionapp.MaterialPlanResult, error) {
	return productionapp.MaterialPlanResult{}, nil
}
func (r *workOrderAPIRepo) CreateQualityInspection(ctx context.Context, cmd productionapp.QualityInspectionCommand) (productionapp.QualityInspectionRow, error) {
	return productionapp.QualityInspectionRow{}, nil
}
func (r *workOrderAPIRepo) ListQualityInspections(ctx context.Context, query productionapp.QualityInspectionQuery) ([]productionapp.QualityInspectionRow, error) {
	return nil, nil
}
func (r *workOrderAPIRepo) ListWIPReservations(ctx context.Context, query productionapp.WIPReservationQuery) (productionapp.WIPReservationResult, error) {
	return productionapp.WIPReservationResult{}, nil
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
		ID:              9,
		WorkOrderID:     20,
		Operation:       "cutting",
		PlannedInputQty: 1000,
		ActualInputQty:  1000,
		ActualOutputQty: 815,
		ActualLossQty:   185,
		ActualLossRate:  0.185,
		ExceptionReason: "裁剪边角料",
		MetricsJSON:     `{"fabric":"cotton"}`,
	}}}))

	req := httptest.NewRequest(http.MethodGet, "/api/produce/job-cards", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{`"planned_input_qty":1000`, `"actual_input_qty":1000`, `"actual_output_qty":815`, `"actual_loss_qty":185`, `"actual_loss_rate":0.185`, `"exception_reason":"裁剪边角料"`, `"metrics_json":"{\"fabric\":\"cotton\"}"`} {
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
