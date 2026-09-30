package productcreator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	app "orderapp/internal/application/productcreator"

	"github.com/labstack/echo/v4"
)

type memoryRepository struct {
	template app.Template
	versions []app.TemplateVersion
	run      app.Run
}

func (r *memoryRepository) ListTemplates(context.Context) ([]app.Template, error) {
	if r.template.ID == 0 {
		return []app.Template{}, nil
	}
	return []app.Template{r.template}, nil
}
func (r *memoryRepository) GetTemplate(_ context.Context, id int64) (app.Template, error) {
	if r.template.ID != id {
		return app.Template{}, app.ErrNotFound
	}
	return r.template, nil
}
func (r *memoryRepository) SaveTemplate(_ context.Context, input app.TemplateSave) (app.Template, error) {
	if input.ID > 0 && input.ExpectedRev != r.template.Revision {
		return app.Template{}, app.ErrConflict
	}
	if input.ID == 0 {
		input.ID = 1
		input.ExpectedRev = 0
		r.template.Revision = 1
	} else {
		r.template.Revision++
	}
	r.template.ID, r.template.Name, r.template.Description, r.template.Draft = input.ID, input.Name, input.Description, input.Workflow
	if r.template.Status == "" {
		r.template.Status = "draft"
	}
	return r.template, nil
}
func (r *memoryRepository) CopyTemplate(context.Context, int64, string, string) (app.Template, error) {
	return app.Template{}, nil
}
func (r *memoryRepository) PublishTemplate(_ context.Context, id, rev int64, actor string) (app.TemplateVersion, error) {
	if r.template.ID != id {
		return app.TemplateVersion{}, app.ErrNotFound
	}
	if r.template.Revision != rev {
		return app.TemplateVersion{}, app.ErrConflict
	}
	if issues := app.ValidateWorkflow(r.template.Draft); len(issues) > 0 {
		return app.TemplateVersion{}, app.InvalidWorkflowError{Issues: issues}
	}
	version := app.TemplateVersion{ID: int64(len(r.versions) + 1), TemplateID: id, Version: int64(len(r.versions) + 1), Name: r.template.Name, Description: r.template.Description, Workflow: r.template.Draft, PublishedBy: actor}
	r.versions = append(r.versions, version)
	r.template.Status, r.template.PublishedVersion, r.template.Revision = "published", version.Version, r.template.Revision+1
	return version, nil
}
func (r *memoryRepository) ListVersions(context.Context, int64) ([]app.TemplateVersion, error) {
	return r.versions, nil
}
func (r *memoryRepository) DisableTemplate(context.Context, int64, int64, string) error { return nil }
func (r *memoryRepository) StartRun(_ context.Context, id int64, actor string) (app.Run, error) {
	if r.template.ID != id || r.template.PublishedVersion == 0 {
		return app.Run{}, app.ErrNotPublished
	}
	r.run = app.Run{ID: 1, TemplateID: id, Version: r.template.PublishedVersion, Revision: 1, Status: "draft", Workflow: r.template.Draft, Inputs: map[string]map[string]any{}}
	return r.run, nil
}
func (r *memoryRepository) ListTemplateRuns(context.Context, int64, int) ([]app.RunSummary, error) {
	return nil, nil
}
func (r *memoryRepository) GetRun(context.Context, int64) (app.Run, error) { return r.run, nil }
func (r *memoryRepository) SaveRunInputs(_ context.Context, id, revision int64, inputs map[string]map[string]any, _ string) (app.Run, error) {
	if id != r.run.ID || revision != r.run.Revision {
		return app.Run{}, app.ErrConflict
	}
	r.run.Inputs, r.run.Revision = inputs, r.run.Revision+1
	return r.run, nil
}
func (r *memoryRepository) SaveRunDraft(_ context.Context, id, revision int64, inputs map[string]map[string]any, variables map[string]string, _ string) (app.Run, error) {
	if id != r.run.ID || revision != r.run.Revision {
		return app.Run{}, app.ErrConflict
	}
	r.run.Inputs, r.run.VariableValues, r.run.Revision = inputs, variables, r.run.Revision+1
	return r.run, nil
}
func (r *memoryRepository) SaveRunPreview(_ context.Context, id, revision int64, preview app.RunPreview, _ string) (app.Run, error) {
	if id != r.run.ID || revision != r.run.Revision {
		return app.Run{}, app.ErrConflict
	}
	r.run.Preview = &preview
	return r.run, nil
}

func TestProductCreatorModuleCatalogAndTemplateLifecycleAPI(t *testing.T) {
	repo := &memoryRepository{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Creator: app.NewService(repo)})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/product-creator/modules", nil)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET module catalog status=%d body=%s", rec.Code, rec.Body.String())
	}
	var catalog struct {
		Modules []app.Module `json:"modules"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &catalog); err != nil {
		t.Fatal(err)
	}
	if len(catalog.Modules) != 17 {
		t.Fatalf("module count=%d, want 7 legacy modules plus 5 BOM-centric modules for versions 2 and 3", len(catalog.Modules))
	}
	var legacyModules, bomCentricModules, variableModules int
	var foundLegacyBOM, foundBOMCentricBOM bool
	for _, module := range catalog.Modules {
		if module.WorkflowVersion == 1 {
			legacyModules++
			if module.Kind == app.ModuleBOM {
				foundLegacyBOM = len(module.Inputs) == 3 && len(module.Outputs) == 3
			}
		}
		if module.WorkflowVersion == 2 {
			bomCentricModules++
			if module.Kind == app.ModuleBOM {
				foundBOMCentricBOM = len(module.Inputs) == 2 && len(module.Outputs) == 1 && module.Category == "动作" && module.PaletteVisible
			}
		}
		if module.WorkflowVersion == 3 {
			variableModules++
			if module.Kind == app.ModuleProduct || module.Kind == app.ModuleMaterial {
				for _, field := range module.Fields {
					if field.Key == "kind" || field.Key == "product_kind" {
						t.Fatalf("V3 module %q exposes removed category field %q", module.Kind, field.Key)
					}
				}
			}
		}
	}
	if legacyModules != 7 || bomCentricModules != 5 || variableModules != 5 || !foundLegacyBOM || !foundBOMCentricBOM {
		t.Fatalf("catalog versions legacy=%d bom-centric=%d variable=%d legacy BOM=%t BOM-centric BOM=%t", legacyModules, bomCentricModules, variableModules, foundLegacyBOM, foundBOMCentricBOM)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/product-creator/templates", strings.NewReader(`{"name":"空模板","workflow":{"nodes":[],"edges":[]}}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST template status=%d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/product-creator/templates/1/publish", strings.NewReader(`{"revision":1}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "empty_workflow") {
		t.Fatalf("publishing empty template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.versions) != 0 {
		t.Fatalf("invalid template created %d published versions", len(repo.versions))
	}
}

func TestCommitConfigurationRouteRequiresIdempotencyKeyAndReturnsExecutorUnavailable(t *testing.T) {
	repo := &memoryRepository{run: app.Run{ID: 9, Revision: 2, Status: "draft", Workflow: app.Workflow{Nodes: []app.Node{{ID: "p", Kind: app.ModuleProduct}}}, Inputs: map[string]map[string]any{"p": {"name": "测试商品", "action": "create", "owner": "factory"}}, Preview: &app.RunPreview{Valid: true}}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("basic_auth_admin", true)
			return next(c)
		}
	})
	RegisterRoutes(e, Dependencies{Creator: app.NewService(repo)})
	req := httptest.NewRequest(http.MethodPost, "/api/product-creator/runs/9/commit", strings.NewReader(`{"revision":2}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "idempotency") {
		t.Fatalf("missing idempotency key status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodPost, "/api/product-creator/runs/9/commit", strings.NewReader(`{"revision":2}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set("Idempotency-Key", "run-9-submit")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured executor status=%d body=%s", rec.Code, rec.Body.String())
	}
}
