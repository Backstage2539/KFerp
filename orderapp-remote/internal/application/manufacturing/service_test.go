package manufacturing

import (
	"context"
	"strings"
	"testing"
)

type fakeRepo struct {
	savedProcess  SaveProcessTemplateCommand
	savedRoute    SaveProcessRouteCommand
	savedIndustry SaveIndustryTemplateCommand
	publishedID   int64
}

func (r *fakeRepo) ListManufacturingOperations(ctx context.Context) ([]ManufacturingOperation, error) {
	return nil, nil
}
func (r *fakeRepo) SaveManufacturingOperation(ctx context.Context, cmd SaveManufacturingOperationCommand) (ManufacturingOperation, error) {
	return ManufacturingOperation{ID: 1, Name: cmd.Name, Code: cmd.Code, Status: cmd.Status, DefaultMinutes: cmd.DefaultMinutes}, nil
}
func (r *fakeRepo) DeactivateManufacturingOperation(ctx context.Context, cmd TemplateStatusCommand) error {
	return nil
}
func (r *fakeRepo) ListManufacturingWorkstations(ctx context.Context) ([]ManufacturingWorkstation, error) {
	return nil, nil
}
func (r *fakeRepo) SaveManufacturingWorkstation(ctx context.Context, cmd SaveManufacturingWorkstationCommand) (ManufacturingWorkstation, error) {
	return ManufacturingWorkstation{ID: 1, Name: cmd.Name, Code: cmd.Code, Status: cmd.Status, DefaultMinutes: cmd.DefaultMinutes, HourlyRate: cmd.HourlyRate}, nil
}
func (r *fakeRepo) DeactivateManufacturingWorkstation(ctx context.Context, cmd TemplateStatusCommand) error {
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
