package productcreator

import (
	"context"
	"encoding/json"
	"fmt"
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
	if len(catalog.Modules) != 39 {
		t.Fatalf("module count=%d, want 7 legacy modules plus 5 BOM-centric modules for versions 2 through 5 and 4 modules each for V6 through V8", len(catalog.Modules))
	}
	var legacyModules, bomCentricModules, variableModules, specificationTemplateModules, processPreviewModules, v6Modules int
	var foundLegacyBOM, foundBOMCentricBOM, foundSpecificationTemplateBOM, foundV5ProcessRouteBOM bool
	var foundV6MaterialRows bool
	foundV6NoPurchase := true
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
		if module.WorkflowVersion == 4 {
			specificationTemplateModules++
			if module.Kind == app.ModuleBOM {
				foundTemplateField := false
				for _, field := range module.Fields {
					if field.Key == "spec_template_version_id" {
						foundTemplateField = !field.Required
					}
				}
				foundSpecificationTemplateBOM = foundTemplateField && module.Category == "动作" && module.PaletteVisible
			}
			if module.Kind == app.ModuleProduct {
				for _, field := range module.Fields {
					if field.Key == "spec_template_version_id" || field.Key == "variants" {
						t.Fatalf("V4 product node exposes BOM specification field %q", field.Key)
					}
				}
			}
		}
		if module.WorkflowVersion == 5 {
			processPreviewModules++
			if module.Kind == app.ModuleBOM {
				hasRouteInput := false
				for _, port := range module.Inputs {
					if port.ID == "route" && port.Label == "工艺路线" {
						hasRouteInput = true
					}
				}
				foundV5ProcessRouteBOM = hasRouteInput && module.Category == "动作" && module.PaletteVisible
			}
			if module.Kind == app.ModuleProduct {
				for _, field := range module.Fields {
					if field.Key == "spec_template_version_id" || field.Key == "variants" {
						t.Fatalf("V5 product node exposes BOM specification field %q", field.Key)
					}
				}
			}
		}
		if module.WorkflowVersion == 6 {
			v6Modules++
			if module.Kind == app.ModuleMaterial {
				foundV6MaterialRows = false
				for _, field := range module.Fields {
					if field.Key == "default_rows" && field.Type == "repeater" {
						foundV6MaterialRows = true
					}
					if field.Key == "rows" {
						t.Fatal("V6 material module still exposes the legacy rows field")
					}
				}
			}
			if module.Kind == app.ModulePurchase {
				foundV6NoPurchase = false
			}
		}
	}
	if legacyModules != 7 || bomCentricModules != 5 || variableModules != 5 || specificationTemplateModules != 5 || processPreviewModules != 5 || v6Modules != 4 || !foundV6MaterialRows || !foundV6NoPurchase || !foundLegacyBOM || !foundBOMCentricBOM || !foundSpecificationTemplateBOM || !foundV5ProcessRouteBOM {
		t.Fatalf("catalog versions legacy=%d bom-centric=%d variable=%d specification-template=%d process-preview=%d V6=%d V6 material rows=%t V6 hides purchase=%t legacy BOM=%t BOM-centric BOM=%t specification-template BOM=%t V5 process-route BOM=%t", legacyModules, bomCentricModules, variableModules, specificationTemplateModules, processPreviewModules, v6Modules, foundV6MaterialRows, foundV6NoPurchase, foundLegacyBOM, foundBOMCentricBOM, foundSpecificationTemplateBOM, foundV5ProcessRouteBOM)
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

func TestRunAPIKeepsExecutionSnapshotAndCurrentArchiveNamesSeparate(t *testing.T) {
	repo := &memoryRepository{run: app.Run{ID: 4, Status: "config_committed", CurrentObjects: []app.ResultObject{{Type: "material", ID: 166, Name: "快乐樱桃-生豆", CreationName: "误填半成品", SourceNodeIDs: []string{"raw"}}}, BusinessResults: map[string]any{"objects": map[string]any{"raw": []any{map[string]any{"type": "material", "id": 166, "name": "误填半成品"}}}}}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Creator: app.NewService(repo)})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/product-creator/runs/4", nil))
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var run app.Run
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if len(run.CurrentObjects) != 1 || run.CurrentObjects[0].Name != "快乐樱桃-生豆" || run.CurrentObjects[0].CreationName != "误填半成品" {
		t.Fatal(run.CurrentObjects)
	}
	if !strings.Contains(rec.Body.String(), `"business_results"`) {
		t.Fatal("execution snapshot disappeared")
	}
}

func TestMaterialAcquisitionPublicationAPI(t *testing.T) {
	for _, version := range []int{6, 7} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			workflow := app.Workflow{Version: version, Nodes: []app.Node{
				{ID: "raw", Kind: app.ModuleMaterial, Config: map[string]any{"data_role": "input", "supply_mode": "purchase"}},
				{ID: "bom", Kind: app.ModuleBOM, Config: map[string]any{"output_type": "material", "route_id": 4}},
				{ID: "semi", Kind: app.ModuleMaterial, Config: map[string]any{"data_role": "output"}},
			}, Edges: []app.Edge{
				{ID: "in", Kind: app.EdgeData, Source: "raw", SourceHandle: "material", Target: "bom", TargetHandle: "components"},
				{ID: "out", Kind: app.EdgeData, Source: "bom", SourceHandle: "assembly", Target: "semi", TargetHandle: "from_bom"},
			}}
			repo := &memoryRepository{template: app.Template{ID: 1, Revision: 1, Draft: workflow}}
			e := echo.New()
			RegisterRoutes(e, Dependencies{Creator: app.NewService(repo)})
			publish := func() *httptest.ResponseRecorder {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/api/product-creator/templates/1/publish", strings.NewReader(`{"revision":1}`))
				req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
				e.ServeHTTP(rec, req)
				return rec
			}
			if version == 7 {
				repo.template.Draft.Nodes[2].Config["supply_mode"] = "manufacture"
				repo.template.Draft.Edges = workflow.Edges[:1]
				rec := publish()
				if rec.Code != 422 || !strings.Contains(rec.Body.String(), "output_bom_required") || len(repo.versions) != 0 {
					t.Fatalf("missing BOM accepted: %d %s", rec.Code, rec.Body.String())
				}
				repo.template.Draft.Edges = workflow.Edges
			}
			rec := publish()
			if rec.Code != http.StatusCreated || len(repo.versions) != 1 {
				t.Fatalf("valid manufactured output rejected: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestV8PreviewAPIDisablesUpstreamAndPermissionsFollowExecution(t *testing.T) {
	w := app.Workflow{Version: 8, Nodes: []app.Node{
		{ID: "raw", Kind: app.ModuleMaterial, Config: map[string]any{"data_role": "input", "supply_mode": "purchase"}},
		{ID: "roast", Kind: app.ModuleBOM, Config: map[string]any{"output_type": "material", "route_id": 1}},
		{ID: "semi", Kind: app.ModuleMaterial, Config: map[string]any{"data_role": "output", "supply_mode": "manufacture"}},
	}, Edges: []app.Edge{
		{ID: "a", Source: "raw", SourceHandle: "material", Target: "roast", TargetHandle: "components", Kind: app.EdgeData},
		{ID: "b", Source: "roast", SourceHandle: "assembly", Target: "semi", TargetHandle: "from_bom", Kind: app.EdgeData},
	}}
	repo := &memoryRepository{run: app.Run{ID: 1, Revision: 1, Status: "draft", Workflow: w, Inputs: map[string]map[string]any{"semi": {"action": "reuse", "material_id": 42}}}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Creator: app.NewService(repo)})
	req := httptest.NewRequest(http.MethodPost, "/api/product-creator/runs/1/preview", strings.NewReader(`{"revision":1}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	var run app.Run
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatal(err)
	}
	if !run.Preview.Valid {
		t.Fatal(run.Preview.Issues)
	}
	for _, s := range run.Preview.Steps {
		if s.NodeID != "semi" && s.Status != "skipped" {
			t.Fatal(s)
		}
	}
	got := strings.Join(configurationRunPermissions(repo.run), ",")
	if got != "materials.read" {
		t.Fatalf("reuse must not require disabled write permissions: %s", got)
	}
	repo.run.Workflow.Version = 7
	if strings.Join(configurationRunPermissions(repo.run), ",") == got {
		t.Fatal("old behavior must remain")
	}
}

func TestV8ExistingInputProductNeedsReadOnlyPermission(t *testing.T) {
	run := app.Run{Workflow: app.Workflow{Version: 8, Nodes: []app.Node{{ID: "source", Kind: app.ModuleProduct, Config: map[string]any{"data_role": "input"}}}}, Inputs: map[string]map[string]any{"source": {"action": "reuse", "product_id": 52}}}
	if got := strings.Join(configurationRunPermissions(run), ","); got != "products.read" {
		t.Fatalf("reference-only input requires no creation permission: %s", got)
	}
	run.Workflow.Version = 7
	if got := strings.Join(configurationRunPermissions(run), ","); got != "products.write" {
		t.Fatalf("legacy permission changed: %s", got)
	}
}
