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
	materialPlanRows  []productionapp.MaterialPlanRow
	qualityCommand    productionapp.QualityInspectionCommand
	qualityQuery      productionapp.QualityInspectionQuery
	reservationQuery  productionapp.WIPReservationQuery
	adjustCommand     productionapp.WIPReservationAdjustCommand
	releaseCommand    productionapp.WIPReservationReleaseCommand
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
func (r *fakeManufacturingGapRepo) Finish(ctx context.Context, cmd productionapp.FinishCommand) (productionapp.FinishResult, error) {
	r.finish = cmd
	return productionapp.FinishResult{RunningItemID: cmd.ID}, nil
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
	if r.materialPlanRows != nil {
		return productionapp.MaterialPlanResult{Rows: r.materialPlanRows}, nil
	}
	return productionapp.MaterialPlanResult{Rows: []productionapp.MaterialPlanRow{{
		MaterialID:             10,
		MaterialName:           "孟连水洗5T批次",
		Unit:                   "g",
		RequiredG:              60000,
		WIPG:                   20000,
		RawG:                   30000,
		ReservedG:              5000,
		AvailableG:             15000,
		WIPTransferSuggestionG: 30000,
		ShortageG:              15000,
		PurchaseSuggestionG:    15000,
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
func (r *fakeManufacturingGapRepo) ListWIPReservations(ctx context.Context, query productionapp.WIPReservationQuery) (productionapp.WIPReservationResult, error) {
	r.reservationQuery = query
	return productionapp.WIPReservationResult{Rows: []productionapp.WIPReservationRow{{
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
	}}, TotalRemainingG: 40000}, nil
}
func (r *fakeManufacturingGapRepo) AdjustWIPReservation(ctx context.Context, cmd productionapp.WIPReservationAdjustCommand) (productionapp.WIPReservationRow, error) {
	r.adjustCommand = cmd
	return productionapp.WIPReservationRow{ID: cmd.ReservationID, ReservedG: cmd.ReservedG, ConsumedG: 20000, RemainingReservedG: cmd.ReservedG - 20000}, nil
}
func (r *fakeManufacturingGapRepo) ReleaseWIPReservations(ctx context.Context, cmd productionapp.WIPReservationReleaseCommand) (productionapp.WIPReservationReleaseResult, error) {
	r.releaseCommand = cmd
	return productionapp.WIPReservationReleaseResult{ReleasedCount: 1, ReleasedG: 40000}, nil
}
func (r *fakeManufacturingGapRepo) AcceptanceSmoke(ctx context.Context) (productionapp.AcceptanceSmokeResult, error) {
	return productionapp.AcceptanceSmokeResult{Rows: []productionapp.AcceptanceSmokeRow{{
		Code:       "wip_stock",
		Title:      "WIP库存",
		Status:     "ok",
		Count:      1,
		Detail:     "已有 WIP 批次",
		View:       "warehouseInventory",
		ViewParams: map[string]string{"warehouse": "wip", "item_type": "material"},
	}}}, nil
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
	for _, want := range []string{`"material_name":"孟连水洗5T批次"`, `"required_g":60000`, `"reserved_g":5000`, `"available_g":15000`, `"wip_transfer_suggestion_g":30000`, `"shortage_g":15000`, `"purchase_suggestion_g":15000`} {
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

	finishBody = []byte(`{"id":8,"finished_units":2,"finished_loose_g":10,"warehouse":"finished_goods","consumed_input_g":700}`)
	req = httptest.NewRequest(http.MethodPost, "/api/produce/running/finish", bytes.NewReader(finishBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/running/finish edited input status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.finish.ID != 8 || repo.finish.Partial || repo.finish.ConsumedInputG != 700 {
		t.Fatalf("edited full finish command = %+v", repo.finish)
	}

	qualityBody := []byte(`{"scope":"raw_material","reference_type":"raw_material","reference_no":"MB-0000000007","item_name":"孟连水洗5T批次","result":"pass","metrics_json":"{\"水分\":\"10.5%\"}","factory_flavor_description":"茉莉花、柑橘","moisture":"10.8%","density":"780g/L","note":"入库杯测通过"}`)
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
	if createResp["ok"] != true ||
		repo.qualityCommand.ReferenceNo != "MB-0000000007" ||
		repo.qualityCommand.FactoryFlavorDescription != "茉莉花、柑橘" ||
		repo.qualityCommand.Moisture != "10.8%" ||
		repo.qualityCommand.Density != "780g/L" ||
		repo.qualityCommand.Operator != "测试员" {
		t.Fatalf("quality create response=%+v command=%+v", createResp, repo.qualityCommand)
	}

	for _, tc := range []struct {
		scope       string
		referenceNo string
		itemName    string
	}{
		{scope: "raw_material", referenceNo: "MB-0000000007", itemName: "孟连水洗5T批次"},
		{scope: "finished_batch", referenceNo: "FP-0000000042", itemName: "耶加雪菲 227g"},
	} {
		body := []byte(`{"scope":"` + tc.scope + `","reference_type":"` + tc.scope + `","reference_no":"` + tc.referenceNo + `","item_name":"` + tc.itemName + `","result":"hold","metrics_json":"{}","note":"抽屉选择"}`)
		req = httptest.NewRequest(http.MethodPost, "/api/produce/quality-inspections", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST /api/produce/quality-inspections %s status=%d body=%s", tc.scope, rec.Code, rec.Body.String())
		}
		if repo.qualityCommand.Scope != tc.scope || repo.qualityCommand.ReferenceNo != tc.referenceNo || repo.qualityCommand.ItemName != tc.itemName {
			t.Fatalf("quality command for %s = %+v", tc.scope, repo.qualityCommand)
		}
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

	req = httptest.NewRequest(http.MethodGet, "/api/produce/wip-reservations?status=reserved&work_order_no=WO-0000000020", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/wip-reservations status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{`"work_order_no":"WO-0000000020"`, `"remaining_reserved_g":40000`, `"total_remaining_g":40000`} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("reservation list response missing %s: %s", want, rec.Body.String())
		}
	}
	if repo.reservationQuery.Status != "reserved" || repo.reservationQuery.WorkOrderNo != "WO-0000000020" {
		t.Fatalf("reservation query = %+v", repo.reservationQuery)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/produce/wip-reservations/adjust", bytes.NewReader([]byte(`{"reservation_id":9,"reserved_g":50000,"note":"修正"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/wip-reservations/adjust status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.adjustCommand.ReservationID != 9 || repo.adjustCommand.ReservedG != 50000 || repo.adjustCommand.Operator != "测试员" {
		t.Fatalf("adjust command = %+v", repo.adjustCommand)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/produce/wip-reservations/release", bytes.NewReader([]byte(`{"running_item_id":20,"work_order_no":"WO-0000000020","note":"释放"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/produce/wip-reservations/release status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.releaseCommand.RunningItemID != 20 || repo.releaseCommand.Operator != "测试员" {
		t.Fatalf("release command = %+v", repo.releaseCommand)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/produce/acceptance-smoke", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/acceptance-smoke status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"wip_stock"`) || !strings.Contains(rec.Body.String(), `"view":"warehouseInventory"`) || !strings.Contains(rec.Body.String(), `"view_params":{"item_type":"material","warehouse":"wip"}`) {
		t.Fatalf("acceptance smoke response missing checklist row: %s", rec.Body.String())
	}
}

func TestManufacturingGapAPIDripFinishedProductComponentShowsUpstreamShortage(t *testing.T) {
	repo := &fakeManufacturingGapRepo{}
	repo.materialPlanRows = []productionapp.MaterialPlanRow{{
		MaterialID:        2,
		MaterialName:      "蓝山熟豆",
		Unit:              "g",
		RequiredG:         150,
		AvailableG:        40,
		ShortageG:         110,
		ComponentType:     "finished_product",
		UpstreamProductID: 2,
		UpstreamShortageG: 110,
	}}
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

	req := httptest.NewRequest(http.MethodGet, "/api/produce/material-plan?selected=1-10&input_1_10=150", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/produce/material-plan status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"component_type":"finished_product"`,
		`"upstream_product_id":2`,
		`"upstream_shortage_g":110`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("drip upstream material plan response missing %s: %s", want, rec.Body.String())
		}
	}
}
