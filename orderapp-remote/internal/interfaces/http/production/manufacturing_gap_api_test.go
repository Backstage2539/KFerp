package production

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"

	"github.com/labstack/echo/v4"
)

type fakeManufacturingGapRepo struct {
	finish            productionapp.FinishCommand
	materialPlanQuery productionapp.MaterialPlanQuery
	qualityCommand    productionapp.QualityInspectionCommand
	qualityQuery      productionapp.QualityInspectionQuery
}

func (r *fakeManufacturingGapRepo) CreateBatch(ctx context.Context, cmd productionapp.CreateBatchCommand) (productionapp.CreateBatchResult, error) {
	return productionapp.CreateBatchResult{}, nil
}
func (r *fakeManufacturingGapRepo) ListBatches(ctx context.Context, cmd productionapp.ListBatchesCommand) ([]productionapp.BatchListItem, error) {
	return nil, nil
}
func (r *fakeManufacturingGapRepo) Detail(ctx context.Context, batchID string) (productionapp.BatchDetail, error) {
	return productionapp.BatchDetail{}, nil
}
func (r *fakeManufacturingGapRepo) PreviewDeduct(ctx context.Context, batchID string) (productionapp.DeductPreview, error) {
	return productionapp.DeductPreview{}, nil
}
func (r *fakeManufacturingGapRepo) ConfirmDeduct(ctx context.Context, batchID, operator string) (productionapp.DeductConfirmResult, error) {
	return productionapp.DeductConfirmResult{}, nil
}
func (r *fakeManufacturingGapRepo) ListRunning(ctx context.Context) ([]productionapp.RunningItem, error) {
	return nil, nil
}
func (r *fakeManufacturingGapRepo) ListStartNeeds(ctx context.Context, cmd productionapp.StartCommand) ([]productionapp.StartNeed, error) {
	return nil, nil
}
func (r *fakeManufacturingGapRepo) Start(ctx context.Context, cmd productionapp.StartExecutionCommand) (productionapp.StartResult, error) {
	return productionapp.StartResult{}, nil
}
func (r *fakeManufacturingGapRepo) Finish(ctx context.Context, cmd productionapp.FinishCommand) error {
	r.finish = cmd
	return nil
}
func (r *fakeManufacturingGapRepo) Cancel(ctx context.Context, cmd productionapp.CancelCommand) error {
	return nil
}
func (r *fakeManufacturingGapRepo) ListMachines(ctx context.Context, activeOnly bool) ([]productionapp.RoastMachine, error) {
	return nil, nil
}
func (r *fakeManufacturingGapRepo) SaveMachine(ctx context.Context, cmd productionapp.RoastMachineCommand) error {
	return nil
}
func (r *fakeManufacturingGapRepo) PlanSummary(ctx context.Context, query productionapp.PlanSummaryQuery) (productionapp.PlanSummaryData, error) {
	return productionapp.PlanSummaryData{}, nil
}
func (r *fakeManufacturingGapRepo) ListProductionLogs(ctx context.Context, query productionapp.ProductionLogsQuery) (productionapp.ProductionLogsResult, error) {
	return productionapp.ProductionLogsResult{}, nil
}
func (r *fakeManufacturingGapRepo) ListWorkOrders(ctx context.Context, query productionapp.WorkOrderQuery) ([]productionapp.WorkOrderRow, error) {
	return nil, nil
}
func (r *fakeManufacturingGapRepo) ListJobCards(ctx context.Context, query productionapp.JobCardQuery) ([]productionapp.JobCardRow, error) {
	return nil, nil
}
func (r *fakeManufacturingGapRepo) ListBatchCosts(ctx context.Context, query productionapp.BatchCostQuery) ([]productionapp.BatchCostRow, error) {
	return nil, nil
}
func (r *fakeManufacturingGapRepo) MaterialPlan(ctx context.Context, query productionapp.MaterialPlanQuery) (productionapp.MaterialPlanResult, error) {
	r.materialPlanQuery = query
	return productionapp.MaterialPlanResult{Rows: []productionapp.MaterialPlanRow{{
		MaterialID:          10,
		MaterialName:        "孟连水洗5T批次",
		Unit:                "g",
		RequiredG:           60000,
		WIPG:                20000,
		RawG:                30000,
		ReservedG:           5000,
		ShortageG:           15000,
		PurchaseSuggestionG: 15000,
	}}}, nil
}
func (r *fakeManufacturingGapRepo) CreateQualityInspection(ctx context.Context, cmd productionapp.QualityInspectionCommand) (productionapp.QualityInspectionRow, error) {
	r.qualityCommand = cmd
	return productionapp.QualityInspectionRow{ID: 1, Scope: cmd.Scope, ReferenceNo: cmd.ReferenceNo, ItemName: cmd.ItemName, Result: cmd.Result, Operator: cmd.Operator}, nil
}
func (r *fakeManufacturingGapRepo) ListQualityInspections(ctx context.Context, query productionapp.QualityInspectionQuery) ([]productionapp.QualityInspectionRow, error) {
	r.qualityQuery = query
	return []productionapp.QualityInspectionRow{{ID: 1, Scope: "work_order", ReferenceNo: "WO-0000000020", ItemName: "测试拼配", Result: "pass"}}, nil
}

func TestManufacturingGapAPIs(t *testing.T) {
	repo := &fakeManufacturingGapRepo{}
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

	req := httptest.NewRequest(http.MethodGet, "/api/produce/material-plan?from=2026-04-01&to=2026-04-28&selected=1-227&input_1_227=60000", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/material-plan status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"material_name":"孟连水洗5T批次"`, `"required_g":60000`, `"reserved_g":5000`, `"shortage_g":15000`, `"purchase_suggestion_g":15000`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("material plan response missing %s: %s", want, rec.Body.String())
		}
	}
	if !repo.materialPlanQuery.Selected["1-227"] || repo.materialPlanQuery.InputByKey["1-227"] != 60000 {
		t.Fatalf("material plan query = %+v", repo.materialPlanQuery)
	}

	finishBody := []byte(`{"id":7,"finished_units":1,"finished_loose_g":0,"warehouse":"finished_goods","partial":true,"consumed_input_g":30000}`)
	req = httptest.NewRequest(http.MethodPost, "/api/produce/running/finish", bytes.NewReader(finishBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/running/finish status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !repo.finish.Partial || repo.finish.ConsumedInputG != 30000 {
		t.Fatalf("finish command = %+v", repo.finish)
	}

	qualityBody := []byte(`{"scope":"work_order","reference_type":"work_order","reference_no":"WO-0000000020","item_name":"测试拼配","result":"pass","metrics_json":"{\"色值\":\"正常\"}","note":"首锅通过"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/produce/quality-inspections", bytes.NewReader(qualityBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/quality-inspections status=%d body=%s", rec.Code, rec.Body.String())
	}
	var createResp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatal(err)
	}
	if createResp["ok"] != true || repo.qualityCommand.ReferenceNo != "WO-0000000020" || repo.qualityCommand.Operator != "测试员" {
		t.Fatalf("quality create response=%+v command=%+v", createResp, repo.qualityCommand)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/produce/quality-inspections?scope=work_order&result=pass", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/quality-inspections status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"scope":"work_order"`, `"reference_no":"WO-0000000020"`, `"result":"pass"`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("quality list response missing %s: %s", want, rec.Body.String())
		}
	}
	if repo.qualityQuery.Scope != "work_order" || repo.qualityQuery.Result != "pass" {
		t.Fatalf("quality query = %+v", repo.qualityQuery)
	}
}
