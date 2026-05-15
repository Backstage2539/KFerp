package production

import (
	"context"
	"net/http"
	"net/http/httptest"
	productionapp "orderapp/internal/application/production"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

type workOrderAPIRepo struct {
	rows []productionapp.WorkOrderRow
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
	return nil, nil
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
