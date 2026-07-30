package manufacturing

import (
	"context"
	"strings"
	"testing"
)

type fakeRepo struct {
	savedOperation           SaveManufacturingOperationCommand
	savedProcess             SaveProcessTemplateCommand
	savedRoute               SaveProcessRouteCommand
	savedIndustry            SaveIndustryTemplateCommand
	savedWorkstation         SaveManufacturingWorkstationCommand
	savedWorkstationCapacity SaveWorkstationCapacityCommand
	workstations             []ManufacturingWorkstation
	workstationCapacities    []ManufacturingWorkstationCapacity
	publishedID              int64
}

func (r *fakeRepo) ListManufacturingOperations(ctx context.Context) ([]ManufacturingOperation, error) {
	return nil, nil
}
func (r *fakeRepo) SaveManufacturingOperation(ctx context.Context, cmd SaveManufacturingOperationCommand) (ManufacturingOperation, error) {
	r.savedOperation = cmd
	return ManufacturingOperation{ID: 1, Name: cmd.Name, Code: cmd.Code, Status: cmd.Status, DefaultMinutes: cmd.DefaultMinutes, StandardOperationCost: cmd.StandardOperationCost}, nil
}
func (r *fakeRepo) DeactivateManufacturingOperation(ctx context.Context, cmd TemplateStatusCommand) error {
	return nil
}
func (r *fakeRepo) ListManufacturingWorkstations(ctx context.Context) ([]ManufacturingWorkstation, error) {
	return r.workstations, nil
}
func (r *fakeRepo) SaveManufacturingWorkstation(ctx context.Context, cmd SaveManufacturingWorkstationCommand) (ManufacturingWorkstation, error) {
	r.savedWorkstation = cmd
	return ManufacturingWorkstation{ID: 1, Name: cmd.Name, Code: cmd.Code, Status: cmd.Status, DefaultMinutes: cmd.DefaultMinutes, MachineHourlyCost: cmd.MachineHourlyCost, LaborHourlyCost: cmd.LaborHourlyCost, OverheadHourlyCost: cmd.OverheadHourlyCost, HourlyRate: cmd.HourlyRate, ApplicableOperationIDs: cmd.ApplicableOperationIDs}, nil
}
func (r *fakeRepo) DeactivateManufacturingWorkstation(ctx context.Context, cmd TemplateStatusCommand) error {
	return nil
}
func (r *fakeRepo) ListManufacturingWorkstationCapacities(ctx context.Context, query WorkstationCapacityQuery) ([]ManufacturingWorkstationCapacity, error) {
	rows := make([]ManufacturingWorkstationCapacity, 0)
	for _, row := range r.workstationCapacities {
		if query.WorkstationID > 0 && row.WorkstationID != query.WorkstationID {
			continue
		}
		if query.Status != "" && row.Status != query.Status {
			continue
		}
		rows = append(rows, row)
	}
	return rows, nil
}
func (r *fakeRepo) SaveManufacturingWorkstationCapacity(ctx context.Context, cmd SaveWorkstationCapacityCommand) (ManufacturingWorkstationCapacity, error) {
	r.savedWorkstationCapacity = cmd
	return ManufacturingWorkstationCapacity{
		ID:                 9,
		WorkstationID:      cmd.WorkstationID,
		Code:               cmd.Code,
		Name:               cmd.Name,
		Status:             cmd.Status,
		BatchSizeQty:       cmd.BatchSizeQty,
		BatchSizeUnit:      cmd.BatchSizeUnit,
		StandardMinutes:    cmd.StandardMinutes,
		HourlyRate:         cmd.HourlyRate,
		CostMethod:         cmd.CostMethod,
		PieceRate:          cmd.PieceRate,
		ProductionCapacity: cmd.ProductionCapacity,
		SortOrder:          cmd.SortOrder,
		Note:               cmd.Note,
	}, nil
}
func (r *fakeRepo) DeactivateManufacturingWorkstationCapacity(ctx context.Context, cmd TemplateStatusCommand) error {
	return nil
}
func (r *fakeRepo) ListIndustryTemplates(ctx context.Context) ([]IndustryFieldTemplate, error) {
	return nil, nil
}
func (r *fakeRepo) SaveIndustryTemplate(ctx context.Context, cmd SaveIndustryTemplateCommand) (IndustryFieldTemplate, error) {
	r.savedIndustry = cmd
	return IndustryFieldTemplate{ID: 1, Name: cmd.Name, Fields: cmd.Fields}, nil
}
func (r *fakeRepo) DeactivateIndustryTemplate(ctx context.Context, cmd TemplateStatusCommand) error {
	return nil
}
func (r *fakeRepo) ListProcessTemplates(ctx context.Context, query ProcessTemplateQuery) ([]ProcessTemplate, error) {
	return nil, nil
}
func (r *fakeRepo) SaveProcessTemplate(ctx context.Context, cmd SaveProcessTemplateCommand) (ProcessTemplate, error) {
	r.savedProcess = cmd
	return ProcessTemplate{ID: 2, Name: cmd.Name, Operations: cmd.Operations}, nil
}
func (r *fakeRepo) PublishProcessTemplate(ctx context.Context, cmd TemplateStatusCommand) error {
	r.publishedID = cmd.ID
	return nil
}
func (r *fakeRepo) DeactivateProcessTemplate(ctx context.Context, cmd TemplateStatusCommand) error {
	return nil
}
func (r *fakeRepo) ListProcessRoutes(ctx context.Context, query ProcessRouteQuery) ([]ProcessRoute, error) {
	return nil, nil
}
func (r *fakeRepo) SaveProcessRoute(ctx context.Context, cmd SaveProcessRouteCommand) (ProcessRoute, error) {
	r.savedRoute = cmd
	return ProcessRoute{ID: 3, Name: cmd.Name, Operations: cmd.Operations}, nil
}
func (r *fakeRepo) PublishProcessRoute(ctx context.Context, cmd TemplateStatusCommand) error {
	r.publishedID = cmd.ID
	return nil
}
func (r *fakeRepo) DeactivateProcessRoute(ctx context.Context, cmd TemplateStatusCommand) error {
	return nil
}

func TestSaveProcessTemplateValidatesAndNormalizesOperations(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if _, err := svc.SaveProcessTemplate(context.Background(), SaveProcessTemplateCommand{
		Name:      "标准制造",
		ProductID: 7,
		Operations: []ProcessTemplateOperation{{
			Operation:            "裁剪",
			ParameterSchemaJSON:  `{"fields":["cloth_width"]}`,
			QualityChecklistJSON: `["尺寸"]`,
			RecordsLoss:          true,
		}},
	}); err != nil {
		t.Fatalf("SaveProcessTemplate: %v", err)
	}
	if repo.savedProcess.Status != "draft" {
		t.Fatalf("status = %q, want draft", repo.savedProcess.Status)
	}
	if repo.savedProcess.KeyParamsJSON != "{}" {
		t.Fatalf("key params = %s, want {}", repo.savedProcess.KeyParamsJSON)
	}
	if repo.savedProcess.Operations[0].Seq != 1 || !repo.savedProcess.Operations[0].RecordsLoss {
		t.Fatalf("operation not normalized: %+v", repo.savedProcess.Operations[0])
	}
}

func TestSaveProcessTemplateRejectsInvalidJSON(t *testing.T) {
	svc := NewService(&fakeRepo{})
	_, err := svc.SaveProcessTemplate(context.Background(), SaveProcessTemplateCommand{
		Name:          "坏模板",
		ProductID:     7,
		KeyParamsJSON: "[]",
		Operations:    []ProcessTemplateOperation{{Operation: "包装"}},
	})
	if err == nil || !strings.Contains(err.Error(), "key_params_json") {
		t.Fatalf("expected key_params_json error, got %v", err)
	}
}

func TestSaveIndustryTemplateValidatesFields(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if _, err := svc.SaveIndustryTemplate(context.Background(), SaveIndustryTemplateCommand{
		Name:        "服装参数",
		IndustryKey: "apparel",
		Fields: []IndustryFieldDefinition{{
			FieldKey:    "cloth_loss_rate",
			Label:       "布料损耗率",
			FieldType:   "ratio",
			OptionsJSON: "",
		}},
	}); err != nil {
		t.Fatalf("SaveIndustryTemplate: %v", err)
	}
	field := repo.savedIndustry.Fields[0]
	if field.SortOrder != 1 || field.OptionsJSON != "[]" {
		t.Fatalf("field not normalized: %+v", field)
	}
}

func TestIndustryCalculatorPreviewUsesGenericManufacturingFormula(t *testing.T) {
	svc := NewService(&fakeRepo{})
	cases := []struct {
		name        string
		industryKey string
		lossRate    float64
		wantInputG  int64
		wantLossG   int64
	}{
		{name: "coffee", industryKey: "coffee", lossRate: 0.18, wantInputG: 55366, wantLossG: 9966},
		{name: "packaging", industryKey: "packaging_box", lossRate: 0.03, wantInputG: 46805, wantLossG: 1405},
		{name: "garment", industryKey: "garment", lossRate: 0.08, wantInputG: 49348, wantLossG: 3948},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := svc.PreviewIndustryCalculator(context.Background(), IndustryCalculatorPreviewCommand{
				IndustryKey: tc.industryKey,
				Inputs:      map[string]float64{"demand_output_g": 45400},
				Config: map[string]float64{
					"loss_rate":                 tc.lossRate,
					"material_unit_cost_per_kg": 42.5,
					"operation_minutes":         60,
					"hourly_rate":               90,
				},
			})
			if err != nil {
				t.Fatalf("PreviewIndustryCalculator() error = %v", err)
			}
			if got.PlannedInputG != tc.wantInputG || got.ExpectedLossG != tc.wantLossG || got.IndustryKey != tc.industryKey {
				t.Fatalf("preview = %+v", got)
			}
			if got.MaterialCost <= 0 || got.OperationCost != 90 || got.TotalCost <= got.MaterialCost || len(got.Lines) < 3 {
				t.Fatalf("cost/lines not calculated: %+v", got)
			}
		})
	}
}

func TestIndustryCalculatorPreviewRejectsInvalidInput(t *testing.T) {
	svc := NewService(&fakeRepo{})
	if _, err := svc.PreviewIndustryCalculator(context.Background(), IndustryCalculatorPreviewCommand{Inputs: map[string]float64{"demand_output_g": 0}}); err == nil {
		t.Fatal("PreviewIndustryCalculator() error = nil, want demand validation")
	}
	if _, err := svc.PreviewIndustryCalculator(context.Background(), IndustryCalculatorPreviewCommand{Inputs: map[string]float64{"demand_output_g": 1000}, Config: map[string]float64{"loss_rate": 1}}); err == nil {
		t.Fatal("PreviewIndustryCalculator() error = nil, want loss rate validation")
	}
}

func TestPublishProcessTemplateRequiresID(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if err := svc.PublishProcessTemplate(context.Background(), TemplateStatusCommand{}); err == nil {
		t.Fatal("PublishProcessTemplate should require id")
	}
	if err := svc.PublishProcessTemplate(context.Background(), TemplateStatusCommand{ID: 9}); err != nil {
		t.Fatalf("PublishProcessTemplate: %v", err)
	}
	if repo.publishedID != 9 {
		t.Fatalf("publishedID = %d, want 9", repo.publishedID)
	}
}

func TestSaveManufacturingWorkstationDerivesHourlyRateFromCostComponents(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	got, err := svc.SaveManufacturingWorkstation(context.Background(), SaveManufacturingWorkstationCommand{
		Name:               "Loring S15",
		MachineHourlyCost:  42.5,
		LaborHourlyCost:    60,
		OverheadHourlyCost: 7.5,
		HourlyRate:         999,
		Actor:              "tester",
	})
	if err != nil {
		t.Fatalf("SaveManufacturingWorkstation: %v", err)
	}
	if repo.savedWorkstation.HourlyRate != 110 || got.HourlyRate != 110 {
		t.Fatalf("hourly rate = saved %.2f returned %.2f, want component sum 110", repo.savedWorkstation.HourlyRate, got.HourlyRate)
	}
	if got.MachineHourlyCost != 42.5 || got.LaborHourlyCost != 60 || got.OverheadHourlyCost != 7.5 {
		t.Fatalf("cost components not preserved: %+v", got)
	}
}

func TestSaveManufacturingOperationKeepsStandardOperationCost(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	got, err := svc.SaveManufacturingOperation(context.Background(), SaveManufacturingOperationCommand{
		Name:                  "烘焙",
		StandardOperationCost: 8.5,
		Actor:                 "tester",
	})
	if err != nil {
		t.Fatalf("SaveManufacturingOperation: %v", err)
	}
	if repo.savedOperation.StandardOperationCost != 8.5 || got.StandardOperationCost != 8.5 {
		t.Fatalf("standard operation cost saved %.2f returned %.2f, want 8.5", repo.savedOperation.StandardOperationCost, got.StandardOperationCost)
	}
	if _, err := svc.SaveManufacturingOperation(context.Background(), SaveManufacturingOperationCommand{Name: "坏工序", StandardOperationCost: -1}); err == nil || !strings.Contains(err.Error(), "standard_operation_cost") {
		t.Fatalf("expected standard_operation_cost validation error, got %v", err)
	}
}

func TestSaveManufacturingWorkstationNormalizesApplicableOperationIDs(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	got, err := svc.SaveManufacturingWorkstation(context.Background(), SaveManufacturingWorkstationCommand{
		Name:                   "布勒烘焙机",
		ApplicableOperationIDs: []int64{2, 0, 2, -1, 1},
	})
	if err != nil {
		t.Fatalf("SaveManufacturingWorkstation: %v", err)
	}
	want := []int64{2, 1}
	if len(repo.savedWorkstation.ApplicableOperationIDs) != len(want) {
		t.Fatalf("saved applicable operation ids = %+v, want %+v", repo.savedWorkstation.ApplicableOperationIDs, want)
	}
	for i := range want {
		if repo.savedWorkstation.ApplicableOperationIDs[i] != want[i] {
			t.Fatalf("saved applicable operation ids = %+v, want %+v", repo.savedWorkstation.ApplicableOperationIDs, want)
		}
	}
	if len(got.ApplicableOperationIDs) != len(want) || got.ApplicableOperationIDs[0] != 2 || got.ApplicableOperationIDs[1] != 1 {
		t.Fatalf("returned applicable operation ids = %+v, want %+v", got.ApplicableOperationIDs, want)
	}
}

func TestSaveProcessRouteKeepsRouteLeanAndNormalizesOperations(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	if _, err := svc.SaveProcessRoute(context.Background(), SaveProcessRouteCommand{
		Name: "咖啡烘焙路线",
		Operations: []ProcessRouteOperation{{
			Operation:            "烘焙",
			Workstation:          "烘焙区",
			DefaultEquipment:     "Loring S15",
			DefaultMinutes:       15,
			RecordsLoss:          true,
			QualityChecklistJSON: `["颜色","香气"]`,
		}},
	}); err != nil {
		t.Fatalf("SaveProcessRoute: %v", err)
	}
	if repo.savedRoute.Status != "draft" {
		t.Fatalf("status = %q, want draft", repo.savedRoute.Status)
	}
	if repo.savedRoute.Operations[0].Seq != 1 || !repo.savedRoute.Operations[0].RecordsLoss {
		t.Fatalf("operation not normalized: %+v", repo.savedRoute.Operations[0])
	}
	if strings.Contains(repo.savedRoute.Note, "expected_loss_rate") {
		t.Fatalf("route should not carry product parameters: %+v", repo.savedRoute)
	}
}

func TestSaveWorkstationCapacityNormalizesReusablePreset(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	got, err := svc.SaveManufacturingWorkstationCapacity(context.Background(), SaveWorkstationCapacityCommand{
		WorkstationID:      2,
		Name:               "布勒 18kg",
		BatchSizeQty:       18,
		BatchSizeUnit:      "kg",
		StandardMinutes:    15,
		HourlyRate:         300,
		ProductionCapacity: 0,
		Actor:              "tester",
	})
	if err != nil {
		t.Fatalf("SaveManufacturingWorkstationCapacity: %v", err)
	}
	if got.Status != "active" || got.Code == "" || got.ProductionCapacity != 1 {
		t.Fatalf("capacity defaults not normalized: %+v", got)
	}
	if repo.savedWorkstationCapacity.WorkstationID != 2 || repo.savedWorkstationCapacity.BatchSizeUnit != "kg" || repo.savedWorkstationCapacity.StandardMinutes != 15 {
		t.Fatalf("saved capacity command = %+v", repo.savedWorkstationCapacity)
	}
	if repo.savedWorkstationCapacity.HourlyRate != 0 || got.HourlyRate != 0 {
		t.Fatalf("capacity should not store hourly rate, saved %.2f returned %.2f", repo.savedWorkstationCapacity.HourlyRate, got.HourlyRate)
	}
}

func TestSaveWorkstationCapacityIgnoresApplicableOperationIDs(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	got, err := svc.SaveManufacturingWorkstationCapacity(context.Background(), SaveWorkstationCapacityCommand{
		WorkstationID:          2,
		Name:                   "布勒 18kg",
		BatchSizeQty:           18,
		BatchSizeUnit:          "kg",
		StandardMinutes:        15,
		HourlyRate:             300,
		ApplicableOperationIDs: []int64{2, 0, 2, -1, 1},
		ProductionCapacity:     1,
	})
	if err != nil {
		t.Fatalf("SaveManufacturingWorkstationCapacity: %v", err)
	}
	if len(repo.savedWorkstationCapacity.ApplicableOperationIDs) != 0 || len(got.ApplicableOperationIDs) != 0 {
		t.Fatalf("capacity should not own applicable operation ids, saved=%+v returned=%+v", repo.savedWorkstationCapacity.ApplicableOperationIDs, got.ApplicableOperationIDs)
	}
}

func TestSaveWorkstationCapacitySupportsPieceCostForCountUnits(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	got, err := svc.SaveManufacturingWorkstationCapacity(context.Background(), SaveWorkstationCapacityCommand{
		WorkstationID:      2,
		Name:               "包装 100 件",
		BatchSizeQty:       100,
		BatchSizeUnit:      "件",
		StandardMinutes:    20,
		CostMethod:         "piece",
		PieceRate:          0.5,
		ProductionCapacity: 1,
	})
	if err != nil {
		t.Fatalf("SaveManufacturingWorkstationCapacity: %v", err)
	}
	if got.CostMethod != "piece" || got.PieceRate != 0.5 {
		t.Fatalf("piece capacity = %+v, want piece 0.5", got)
	}
	if repo.savedWorkstationCapacity.CostMethod != "piece" || repo.savedWorkstationCapacity.PieceRate != 0.5 {
		t.Fatalf("saved piece capacity = %+v", repo.savedWorkstationCapacity)
	}
	if repo.savedWorkstationCapacity.BatchSizeUnit != "件" {
		t.Fatalf("saved piece unit = %q, want canonical 件", repo.savedWorkstationCapacity.BatchSizeUnit)
	}

	_, err = svc.SaveManufacturingWorkstationCapacity(context.Background(), SaveWorkstationCapacityCommand{
		WorkstationID: 2,
		Name:          "兼容英文计件",
		BatchSizeQty:  100,
		BatchSizeUnit: "pcs",
		CostMethod:    "piece",
		PieceRate:     0.5,
	})
	if err != nil {
		t.Fatalf("SaveManufacturingWorkstationCapacity alias: %v", err)
	}
	if repo.savedWorkstationCapacity.BatchSizeUnit != "件" {
		t.Fatalf("aliased piece unit = %q, want canonical 件", repo.savedWorkstationCapacity.BatchSizeUnit)
	}
}

func TestSaveWorkstationCapacityRejectsPieceCostForWeightUnits(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	_, err := svc.SaveManufacturingWorkstationCapacity(context.Background(), SaveWorkstationCapacityCommand{
		WorkstationID: 2,
		Name:          "错误包装产能",
		BatchSizeQty:  22.7,
		BatchSizeUnit: "kg",
		CostMethod:    "piece",
		PieceRate:     0.5,
	})
	if err == nil || !strings.Contains(err.Error(), "标准产能单位必须使用“件”") {
		t.Fatalf("weight piece cost error = %v, want Chinese sales-spec piece rejection", err)
	}
	_, err = svc.SaveManufacturingWorkstationCapacity(context.Background(), SaveWorkstationCapacityCommand{
		WorkstationID: 2,
		Name:          "歧义包装产能",
		BatchSizeQty:  100,
		BatchSizeUnit: "袋",
		CostMethod:    "piece",
		PieceRate:     0.5,
	})
	if err == nil || !strings.Contains(err.Error(), "标准产能单位必须使用“件”") {
		t.Fatalf("package-layer piece cost error = %v, want Chinese sales-spec piece rejection", err)
	}
}

func TestSaveWorkstationCapacityDefaultsLegacyCostToTime(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	got, err := svc.SaveManufacturingWorkstationCapacity(context.Background(), SaveWorkstationCapacityCommand{
		WorkstationID: 2,
		Name:          "布勒 18kg",
		BatchSizeQty:  18,
		BatchSizeUnit: "kg",
		PieceRate:     9,
	})
	if err != nil {
		t.Fatalf("SaveManufacturingWorkstationCapacity: %v", err)
	}
	if got.CostMethod != "time" || got.PieceRate != 0 {
		t.Fatalf("legacy capacity = %+v, want time with zero piece rate", got)
	}
}

func TestSaveProcessRouteClearsWorkstationCapacityCostFields(t *testing.T) {
	repo := &fakeRepo{workstationCapacities: []ManufacturingWorkstationCapacity{{
		ID: 9, WorkstationID: 2, Workstation: "布勒烘焙机", Name: "布勒 18kg", Status: "active",
		BatchSizeQty: 5, BatchSizeUnit: "kg", StandardMinutes: 60, HourlyRate: 5,
	}}}
	svc := NewService(repo)
	if _, err := svc.SaveProcessRoute(context.Background(), SaveProcessRouteCommand{
		Name: "烘焙路线",
		Operations: []ProcessRouteOperation{{
			Operation:               "烘焙",
			WorkstationCapacityID:   9,
			WorkstationCapacityName: "布勒 18kg",
			WorkstationID:           2,
			Workstation:             "布勒烘焙机",
			BatchSizeQty:            5,
			BatchSizeUnit:           "kg",
			StandardMinutes:         60,
			HourlyRate:              5,
			PlannedBatchCount:       1,
			PlannedMinutes:          60,
			PlannedOperationCost:    1,
			RecordsLoss:             true,
		}},
	}); err != nil {
		t.Fatalf("SaveProcessRoute: %v", err)
	}
	op := repo.savedRoute.Operations[0]
	if op.WorkstationID != 0 || op.Workstation != "" || op.WorkstationCapacityID != 0 || op.WorkstationCapacityName != "" {
		t.Fatalf("route operation should clear workstation capacity fields: %+v", op)
	}
	if op.BatchSizeQty != 0 || op.BatchSizeUnit != "" || op.StandardMinutes != 0 || op.HourlyRate != 0 || op.CostMethod != "time" || op.PieceRate != 0 || op.PlannedOperationCost != 0 {
		t.Fatalf("route operation should clear capacity cost fields: %+v", op)
	}
}

func TestSaveProcessRouteSnapshotsPieceCostCapacity(t *testing.T) {
	repo := &fakeRepo{workstationCapacities: []ManufacturingWorkstationCapacity{{
		ID: 9, WorkstationID: 2, Workstation: "包装工位", Name: "包装 100 件", Status: "active",
		BatchSizeQty: 100, BatchSizeUnit: "件", StandardMinutes: 20, HourlyRate: 300,
		CostMethod: "piece", PieceRate: 0.5, ApplicableOperationIDs: []int64{7},
	}}}
	svc := NewService(repo)
	route, err := svc.SaveProcessRoute(context.Background(), SaveProcessRouteCommand{
		Name: "计件包装路线",
		Operations: []ProcessRouteOperation{{
			OperationID: 7, Operation: "包装", StandardCostCapacityID: 9,
		}},
	})
	if err != nil {
		t.Fatalf("SaveProcessRoute: %v", err)
	}
	op := route.Operations[0]
	if op.CostMethod != "piece" || op.PieceRate != 0.5 || op.PlannedOperationCost != 0.5 {
		t.Fatalf("piece route snapshot = %+v, want 0.5 per bag", op)
	}
	if !strings.Contains(op.StandardCostSummary, "0.5000元/销售规格件") {
		t.Fatalf("piece route summary = %q", op.StandardCostSummary)
	}
}

func TestSaveProcessRouteKeepsStandardCostCapacityAndSnapshotsCandidate(t *testing.T) {
	repo := &fakeRepo{workstationCapacities: []ManufacturingWorkstationCapacity{{
		ID: 9, WorkstationID: 2, Workstation: "布勒烘焙机", Name: "布勒 18kg", Status: "active",
		BatchSizeQty: 20, BatchSizeUnit: "kg", StandardMinutes: 60, HourlyRate: 100,
		ApplicableOperationIDs: []int64{7},
	}}}
	svc := NewService(repo)
	route, err := svc.SaveProcessRoute(context.Background(), SaveProcessRouteCommand{
		Name: "标准烘焙路线",
		Operations: []ProcessRouteOperation{{
			OperationID:            7,
			Operation:              "烘焙",
			StandardCostCapacityID: 9,
		}},
	})
	if err != nil {
		t.Fatalf("SaveProcessRoute: %v", err)
	}
	op := route.Operations[0]
	if op.StandardCostCapacityID != 9 || op.StandardCostCapacityName != "布勒 18kg" || op.StandardCostWorkstation != "布勒烘焙机" {
		t.Fatalf("standard cost capacity not preserved: %+v", op)
	}
	if op.BatchSizeQty != 20 || op.BatchSizeUnit != "kg" || op.StandardMinutes != 60 || op.HourlyRate != 100 || op.PlannedOperationCost != 5 {
		t.Fatalf("standard cost capacity snapshot = %+v, want 20kg/60min/100 hourly => 5/kg", op)
	}
	if repo.savedRoute.Operations[0].WorkstationCapacityID != 0 || repo.savedRoute.Operations[0].WorkstationID != 0 {
		t.Fatalf("standard cost default must not become actual route capacity fields: %+v", repo.savedRoute.Operations[0])
	}
}

func TestSaveProcessRouteRejectsStandardCostCapacityOutsideOperation(t *testing.T) {
	repo := &fakeRepo{workstationCapacities: []ManufacturingWorkstationCapacity{{
		ID: 9, WorkstationID: 2, Workstation: "布勒烘焙机", Name: "布勒 18kg", Status: "active",
		BatchSizeQty: 20, BatchSizeUnit: "kg", StandardMinutes: 60, HourlyRate: 100,
		ApplicableOperationIDs: []int64{8},
	}}}
	svc := NewService(repo)
	_, err := svc.SaveProcessRoute(context.Background(), SaveProcessRouteCommand{
		Name: "标准烘焙路线",
		Operations: []ProcessRouteOperation{{
			OperationID:            7,
			Operation:              "烘焙",
			StandardCostCapacityID: 9,
		}},
	})
	if err == nil || !strings.Contains(err.Error(), "standard_cost_capacity_id") {
		t.Fatalf("SaveProcessRoute error = %v, want standard_cost_capacity_id validation", err)
	}
}

func TestSaveProcessRouteIgnoresPlannedOperationCostInput(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)
	route, err := svc.SaveProcessRoute(context.Background(), SaveProcessRouteCommand{
		Name: "路线模板",
		Operations: []ProcessRouteOperation{{
			Operation:            "烘焙",
			PlannedOperationCost: -1,
		}},
	})
	if err != nil {
		t.Fatalf("SaveProcessRoute should ignore route planned cost input, got error: %v", err)
	}
	if route.Operations[0].PlannedOperationCost != 0 {
		t.Fatalf("route operation planned cost = %.2f, want cleared", route.Operations[0].PlannedOperationCost)
	}
}

func TestSaveProcessRouteIgnoresCapacityWorkstationMismatch(t *testing.T) {
	repo := &fakeRepo{workstationCapacities: []ManufacturingWorkstationCapacity{{
		ID: 9, WorkstationID: 2, Name: "布勒 18kg", Status: "active",
		BatchSizeQty: 18, BatchSizeUnit: "kg", StandardMinutes: 15, HourlyRate: 300,
	}}}
	svc := NewService(repo)
	route, err := svc.SaveProcessRoute(context.Background(), SaveProcessRouteCommand{
		Name: "错误路线",
		Operations: []ProcessRouteOperation{{
			Operation:             "烘焙",
			WorkstationID:         3,
			Workstation:           "智烘",
			WorkstationCapacityID: 9,
		}},
	})
	if err != nil {
		t.Fatalf("SaveProcessRoute should ignore route capacity fields, got error: %v", err)
	}
	if route.Operations[0].WorkstationID != 0 || route.Operations[0].WorkstationCapacityID != 0 {
		t.Fatalf("route operation should clear mismatched capacity fields: %+v", route.Operations[0])
	}
}
