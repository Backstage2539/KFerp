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
	processSaved  manufacturingapp.SaveProcessTemplateCommand
	industrySaved manufacturingapp.SaveIndustryTemplateCommand
	publishedID   int64
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
