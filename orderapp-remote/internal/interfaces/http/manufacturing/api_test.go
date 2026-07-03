package manufacturing

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	manufacturingapp "orderapp/internal/application/manufacturing"

	"github.com/labstack/echo/v4"
)

type apiRepo struct {
	processSaved             manufacturingapp.SaveProcessTemplateCommand
	routeSaved               manufacturingapp.SaveProcessRouteCommand
	industrySaved            manufacturingapp.SaveIndustryTemplateCommand
	workstationSaved         manufacturingapp.SaveManufacturingWorkstationCommand
	workstationCapacitySaved manufacturingapp.SaveWorkstationCapacityCommand
	publishedID              int64
}

func (r *apiRepo) ListManufacturingOperations(ctx context.Context) ([]manufacturingapp.ManufacturingOperation, error) {
	return []manufacturingapp.ManufacturingOperation{{ID: 1, Name: "烘焙", Code: "roast", Status: "active", StandardOperationCost: 8.5}}, nil
}
func (r *apiRepo) SaveManufacturingOperation(ctx context.Context, cmd manufacturingapp.SaveManufacturingOperationCommand) (manufacturingapp.ManufacturingOperation, error) {
	return manufacturingapp.ManufacturingOperation{ID: 1, Name: cmd.Name, Code: cmd.Code, Status: cmd.Status, DefaultMinutes: cmd.DefaultMinutes, StandardOperationCost: cmd.StandardOperationCost}, nil
}
func (r *apiRepo) DeactivateManufacturingOperation(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	return nil
}
func (r *apiRepo) ListManufacturingWorkstations(ctx context.Context) ([]manufacturingapp.ManufacturingWorkstation, error) {
	return []manufacturingapp.ManufacturingWorkstation{{
		ID: 2, Name: "烘焙机", Code: "roaster", Status: "active", MachineHourlyCost: 40, LaborHourlyCost: 55, OverheadHourlyCost: 5, HourlyRate: 100,
		ApplicableOperationIDs: []int64{1},
		ApplicableOperations:   []manufacturingapp.ManufacturingOperation{{ID: 1, Name: "烘焙", Status: "active"}},
	}}, nil
}
func (r *apiRepo) SaveManufacturingWorkstation(ctx context.Context, cmd manufacturingapp.SaveManufacturingWorkstationCommand) (manufacturingapp.ManufacturingWorkstation, error) {
	r.workstationSaved = cmd
	return manufacturingapp.ManufacturingWorkstation{ID: 2, Name: cmd.Name, Code: cmd.Code, Status: cmd.Status, DefaultMinutes: cmd.DefaultMinutes, MachineHourlyCost: cmd.MachineHourlyCost, LaborHourlyCost: cmd.LaborHourlyCost, OverheadHourlyCost: cmd.OverheadHourlyCost, HourlyRate: cmd.HourlyRate, ApplicableOperationIDs: cmd.ApplicableOperationIDs}, nil
}
func (r *apiRepo) DeactivateManufacturingWorkstation(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	return nil
}
func (r *apiRepo) ListManufacturingWorkstationCapacities(ctx context.Context, query manufacturingapp.WorkstationCapacityQuery) ([]manufacturingapp.ManufacturingWorkstationCapacity, error) {
	return []manufacturingapp.ManufacturingWorkstationCapacity{{
		ID: 9, WorkstationID: query.WorkstationID, Code: "BUHLER-18KG", Name: "布勒 18kg", Status: "active",
		BatchSizeQty: 18, BatchSizeUnit: "kg", StandardMinutes: 15, HourlyRate: 300, ProductionCapacity: 1,
	}}, nil
}
func (r *apiRepo) SaveManufacturingWorkstationCapacity(ctx context.Context, cmd manufacturingapp.SaveWorkstationCapacityCommand) (manufacturingapp.ManufacturingWorkstationCapacity, error) {
	r.workstationCapacitySaved = cmd
	return manufacturingapp.ManufacturingWorkstationCapacity{
		ID: 9, WorkstationID: cmd.WorkstationID, Code: cmd.Code, Name: cmd.Name, Status: cmd.Status,
		BatchSizeQty: cmd.BatchSizeQty, BatchSizeUnit: cmd.BatchSizeUnit, StandardMinutes: cmd.StandardMinutes,
		HourlyRate: cmd.HourlyRate, ProductionCapacity: cmd.ProductionCapacity,
		ApplicableOperationIDs: cmd.ApplicableOperationIDs,
	}, nil
}
func (r *apiRepo) DeactivateManufacturingWorkstationCapacity(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	r.publishedID = cmd.ID
	return nil
}
func (r *apiRepo) ListIndustryTemplates(ctx context.Context) ([]manufacturingapp.IndustryFieldTemplate, error) {
	return []manufacturingapp.IndustryFieldTemplate{{ID: 1, Name: "咖啡参数", IndustryKey: "coffee", Status: "active"}}, nil
}
func (r *apiRepo) SaveIndustryTemplate(ctx context.Context, cmd manufacturingapp.SaveIndustryTemplateCommand) (manufacturingapp.IndustryFieldTemplate, error) {
	r.industrySaved = cmd
	return manufacturingapp.IndustryFieldTemplate{ID: 3, Name: cmd.Name, IndustryKey: cmd.IndustryKey, Status: cmd.Status, Fields: cmd.Fields}, nil
}
func (r *apiRepo) DeactivateIndustryTemplate(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	return nil
}
func (r *apiRepo) ListProcessTemplates(ctx context.Context, query manufacturingapp.ProcessTemplateQuery) ([]manufacturingapp.ProcessTemplate, error) {
	return []manufacturingapp.ProcessTemplate{{ID: 2, Name: "服装裁剪缝制", ProductID: query.ProductID, Status: "active"}}, nil
}
func (r *apiRepo) SaveProcessTemplate(ctx context.Context, cmd manufacturingapp.SaveProcessTemplateCommand) (manufacturingapp.ProcessTemplate, error) {
	r.processSaved = cmd
	return manufacturingapp.ProcessTemplate{ID: 4, Name: cmd.Name, ProductID: cmd.ProductID, Status: cmd.Status, Operations: cmd.Operations}, nil
}
func (r *apiRepo) PublishProcessTemplate(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	r.publishedID = cmd.ID
	return nil
}
func (r *apiRepo) DeactivateProcessTemplate(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	return nil
}
func (r *apiRepo) ListProcessRoutes(ctx context.Context, query manufacturingapp.ProcessRouteQuery) ([]manufacturingapp.ProcessRoute, error) {
	return []manufacturingapp.ProcessRoute{{ID: 8, Name: "烘焙路线", Status: "active"}}, nil
}
func (r *apiRepo) SaveProcessRoute(ctx context.Context, cmd manufacturingapp.SaveProcessRouteCommand) (manufacturingapp.ProcessRoute, error) {
	r.routeSaved = cmd
	return manufacturingapp.ProcessRoute{ID: 9, Name: cmd.Name, Status: cmd.Status, Operations: cmd.Operations}, nil
}
func (r *apiRepo) PublishProcessRoute(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	r.publishedID = cmd.ID
	return nil
}
func (r *apiRepo) DeactivateProcessRoute(ctx context.Context, cmd manufacturingapp.TemplateStatusCommand) error {
	return nil
}

func TestProcessTemplateAPIListAndSave(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/process-templates?product_id=7", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"product_id":7`) {
		t.Fatalf("list response missing product_id filter echo: %s", rec.Body.String())
	}

	body := `{"name":"通用加工","product_id":7,"operations":[{"operation":"裁剪","records_loss":true}]}`
	req = httptest.NewRequest(http.MethodPost, "/api/process-templates", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.processSaved.ProductID != 7 || repo.processSaved.Operations[0].Operation != "裁剪" {
		t.Fatalf("saved command = %+v", repo.processSaved)
	}
}

func TestIndustryFieldTemplateAPISave(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	body := `{"name":"鲜果参数","industry_key":"fruit","fields":[{"field_key":"peel_loss_rate","label":"去皮损耗率","field_type":"ratio"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/industry-field-templates", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.industrySaved.IndustryKey != "fruit" || repo.industrySaved.Fields[0].FieldKey != "peel_loss_rate" {
		t.Fatalf("saved industry command = %+v", repo.industrySaved)
	}
}

func TestIndustryFieldTemplateAPISavesTextAndSelectFields(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	body := `{"name":"商品内容字段","fields":[{"field_key":"烘焙度","field_type":"select","options_json":"[\"浅烘\",\"中烘\"]"},{"field_key":"卖点","field_type":"text"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/industry-field-templates", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.industrySaved.IndustryKey != "general" {
		t.Fatalf("industry key = %q, want general", repo.industrySaved.IndustryKey)
	}
	if len(repo.industrySaved.Fields) != 2 || repo.industrySaved.Fields[0].FieldKey != "烘焙度" || repo.industrySaved.Fields[0].Label != "烘焙度" || repo.industrySaved.Fields[0].FieldType != "select" || repo.industrySaved.Fields[0].OptionsJSON != `["浅烘","中烘"]` || repo.industrySaved.Fields[1].FieldKey != "卖点" || repo.industrySaved.Fields[1].Label != "卖点" || repo.industrySaved.Fields[1].FieldType != "text" {
		t.Fatalf("saved industry text/select fields = %+v", repo.industrySaved.Fields)
	}
}

func TestIndustryCalculatorPreviewAPI(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	body := `{"industry_key":"garment","inputs":{"demand_output_g":45400},"config":{"loss_rate":0.08,"material_unit_cost_per_kg":42.5,"operation_minutes":60,"hourly_rate":90}}`
	req := httptest.NewRequest(http.MethodPost, "/api/industry-calculators/preview", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"industry_key":"garment"`,
		`"demand_output_g":45400`,
		`"planned_input_g":49348`,
		`"expected_loss_g":3948`,
		`"operation_cost":90`,
		`"lines"`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("calculator preview missing %s: %s", want, rec.Body.String())
		}
	}
}

func TestWorkstationAPISavesCostComponentsAndDerivedHourlyRate(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	body := `{"name":"Loring S15","machine_hourly_cost":42.5,"labor_hourly_cost":60,"overhead_hourly_cost":7.5,"hourly_rate":999,"applicable_operation_ids":[1,2]}`
	req := httptest.NewRequest(http.MethodPost, "/api/manufacturing-workstations", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save workstation status=%d body=%s", rec.Code, rec.Body.String())
	}
	for _, want := range []string{
		`"machine_hourly_cost":42.5`,
		`"labor_hourly_cost":60`,
		`"overhead_hourly_cost":7.5`,
		`"hourly_rate":110`,
	} {
		if !strings.Contains(rec.Body.String(), want) {
			t.Fatalf("workstation response missing %s: %s", want, rec.Body.String())
		}
	}
	if repo.workstationSaved.HourlyRate != 110 {
		t.Fatalf("saved hourly rate = %.2f, want derived 110", repo.workstationSaved.HourlyRate)
	}
	if len(repo.workstationSaved.ApplicableOperationIDs) != 2 || repo.workstationSaved.ApplicableOperationIDs[0] != 1 || repo.workstationSaved.ApplicableOperationIDs[1] != 2 {
		t.Fatalf("saved workstation applicable operations = %+v", repo.workstationSaved.ApplicableOperationIDs)
	}
}

func TestManufacturingOperationAPISavesStandardOperationCost(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	body := `{"name":"烘焙","standard_operation_cost":8.5}`
	req := httptest.NewRequest(http.MethodPost, "/api/manufacturing-operations", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save operation status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"standard_operation_cost":8.5`) {
		t.Fatalf("operation response missing standard operation cost: %s", rec.Body.String())
	}
}

func TestPublishProcessTemplateAPI(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/process-templates/9/publish", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.publishedID != 9 {
		t.Fatalf("publishedID=%d, want 9", repo.publishedID)
	}
}

func TestProcessRouteAPIListSaveAndPublish(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/process-routes", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"烘焙路线"`) {
		t.Fatalf("list route status=%d body=%s", rec.Code, rec.Body.String())
	}

	body := `{"name":"通用烘焙","operations":[{"operation":"烘焙","planned_operation_cost":18.5,"records_loss":true,"quality_checklist_json":"[\"颜色\"]"}]}`
	req = httptest.NewRequest(http.MethodPost, "/api/process-routes", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save route status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.routeSaved.Name != "通用烘焙" || !repo.routeSaved.Operations[0].RecordsLoss || repo.routeSaved.Operations[0].PlannedOperationCost != 0 {
		t.Fatalf("saved route command = %+v", repo.routeSaved)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/process-routes/9/publish", strings.NewReader(`{}`))
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.publishedID != 9 {
		t.Fatalf("publish route status=%d published=%d body=%s", rec.Code, repo.publishedID, rec.Body.String())
	}
}

func TestWorkstationCapacityAPIListSaveAndDeactivate(t *testing.T) {
	repo := &apiRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Manufacturing: manufacturingapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/manufacturing-workstation-capacities?workstation_id=2", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"name":"布勒 18kg"`) {
		t.Fatalf("list capacity status=%d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"applicable_operations"`) {
		t.Fatalf("capacity response should not own applicable operations: %s", rec.Body.String())
	}

	body := `{"workstation_id":2,"name":"布勒 15kg","batch_size_qty":15,"batch_size_unit":"kg","standard_minutes":13,"hourly_rate":280,"applicable_operation_ids":[1,2]}`
	req = httptest.NewRequest(http.MethodPost, "/api/manufacturing-workstation-capacities", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("save capacity status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.workstationCapacitySaved.WorkstationID != 2 || repo.workstationCapacitySaved.BatchSizeQty != 15 || repo.workstationCapacitySaved.StandardMinutes != 13 || repo.workstationCapacitySaved.HourlyRate != 0 {
		t.Fatalf("saved workstation capacity command = %+v", repo.workstationCapacitySaved)
	}
	if len(repo.workstationCapacitySaved.ApplicableOperationIDs) != 0 {
		t.Fatalf("capacity should ignore applicable operations = %+v", repo.workstationCapacitySaved.ApplicableOperationIDs)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/manufacturing-workstation-capacities/9/deactivate", strings.NewReader(`{}`))
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || repo.publishedID != 9 {
		t.Fatalf("deactivate capacity status=%d id=%d body=%s", rec.Code, repo.publishedID, rec.Body.String())
	}
}
