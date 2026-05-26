package manufacturing

import (
	"context"
	"strings"
	"testing"
)

type fakeRepo struct {
	savedProcess  SaveProcessTemplateCommand
	savedIndustry SaveIndustryTemplateCommand
	publishedID   int64
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
