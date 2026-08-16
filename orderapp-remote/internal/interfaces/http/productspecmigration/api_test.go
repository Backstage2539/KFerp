package productspecmigration

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/labstack/echo/v4"
)

type fakeMigrationRepo struct {
	row     productspecmigrationapp.ProductMigration
	err     error
	options []productspecmigrationapp.ProductSpecOption
}

func (r *fakeMigrationRepo) Get(context.Context, int64) (productspecmigrationapp.ProductMigration, error) {
	return r.row, r.err
}
func (r *fakeMigrationRepo) Prepare(context.Context, productspecmigrationapp.PrepareCommand) (productspecmigrationapp.ProductMigration, error) {
	return r.row, r.err
}
func (r *fakeMigrationRepo) Assess(context.Context, productspecmigrationapp.AssessCommand) (productspecmigrationapp.ProductMigration, error) {
	return r.row, r.err
}
func (r *fakeMigrationRepo) Cutover(context.Context, productspecmigrationapp.CutoverCommand) (productspecmigrationapp.ProductMigration, error) {
	return r.row, r.err
}
func (r *fakeMigrationRepo) ResolveIdentity(_ context.Context, cmd productspecmigrationapp.ResolveIdentityCommand) (productspecmigrationapp.BusinessIdentity, error) {
	return productspecmigrationapp.BusinessIdentity{ProductID: cmd.ProductID, BomSpecID: cmd.BomSpecID}, r.err
}
func (r *fakeMigrationRepo) ListOptions(context.Context, int64) ([]productspecmigrationapp.ProductSpecOption, error) {
	return r.options, r.err
}

func TestMigrationRoutesExposePerProductState(t *testing.T) {
	e := echo.New()
	repo := &fakeMigrationRepo{row: productspecmigrationapp.ProductMigration{
		ProductID: 42,
		State:     productspecmigrationapp.StatePreparing,
	}}
	RegisterRoutes(e, Dependencies{Migration: productspecmigrationapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/products/42/bom-spec-migration", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"product_id":42`) || !strings.Contains(rec.Body.String(), `"state":"preparing"`) {
		t.Fatalf("GET body = %s", rec.Body.String())
	}
}

func TestCutoverRouteReturnsConflictWithMachineReadableBlockers(t *testing.T) {
	e := echo.New()
	readiness := productspecmigrationapp.Readiness{Blockers: []productspecmigrationapp.Blocker{{Code: "unfinished_orders", Count: 2}}}
	repo := &fakeMigrationRepo{err: &productspecmigrationapp.CutoverBlockedError{Readiness: readiness}}
	RegisterRoutes(e, Dependencies{Migration: productspecmigrationapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/products/42/bom-spec-migration/cutover", nil)
	req.Header.Set("X-Actor", "migration-admin")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("cutover status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"unfinished_orders"`) {
		t.Fatalf("cutover body = %s", rec.Body.String())
	}
}

func TestCutoverRouteExposesTemplateProvenanceAndMainInputBlockers(t *testing.T) {
	e := echo.New()
	readiness := productspecmigrationapp.Readiness{
		InvalidSpecTemplateProvenanceCount: 1,
		InactiveMainInputMaterialCount:     1,
		Blockers: []productspecmigrationapp.Blocker{
			{Code: "missing_published_spec_template_provenance", Count: 1, Message: "当前默认已发布 BOM 版本不是从曾发布的规格模板复制，不能切换"},
			{Code: "inactive_main_input_material", Count: 1, Message: "当前默认已发布 BOM 版本未配置有效的主投入物料"},
		},
	}
	repo := &fakeMigrationRepo{err: &productspecmigrationapp.CutoverBlockedError{Readiness: readiness}}
	RegisterRoutes(e, Dependencies{Migration: productspecmigrationapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/products/42/bom-spec-migration/cutover", nil)
	req.Header.Set("X-Actor", "migration-admin")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("cutover status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, fragment := range []string{
		`"invalid_spec_template_provenance_count":1`,
		`"inactive_main_input_material_count":1`,
		`"code":"missing_published_spec_template_provenance"`,
		`"code":"inactive_main_input_material"`,
	} {
		if !strings.Contains(rec.Body.String(), fragment) {
			t.Fatalf("cutover body = %s, want %s", rec.Body.String(), fragment)
		}
	}
}

func TestMigrationRouteRejectsInvalidProductID(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Migration: productspecmigrationapp.NewService(&fakeMigrationRepo{})})

	req := httptest.NewRequest(http.MethodPost, "/api/products/not-a-number/bom-spec-migration/prepare", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProductBOMSpecOptionsExposeCanonicalWriteIdentity(t *testing.T) {
	e := echo.New()
	repo := &fakeMigrationRepo{options: []productspecmigrationapp.ProductSpecOption{{
		ParentProductID: 42, BomSpecID: 501, BomVariantID: 601,
		WriteProductID: 42, WriteBomSpecID: 501, MigrationState: productspecmigrationapp.StateCutover,
	}}}
	RegisterRoutes(e, Dependencies{Migration: productspecmigrationapp.NewService(repo)})
	req := httptest.NewRequest(http.MethodGet, "/api/products/42/bom-spec-options", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"write_bom_spec_id":501`) {
		t.Fatalf("options status=%d body=%s", rec.Code, rec.Body.String())
	}
}
