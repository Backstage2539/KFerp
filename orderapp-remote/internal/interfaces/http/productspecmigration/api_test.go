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

func TestMigrationRoutesAreRemoved(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Migration: productspecmigrationapp.NewService(&fakeMigrationRepo{})})
	for _, target := range []string{
		"/api/products/42/bom-spec-migration",
		"/api/products/42/bom-spec-migration/prepare",
		"/api/products/42/bom-spec-migration/readiness",
		"/api/products/42/bom-spec-migration/cutover",
		"/api/product-bom-spec-migrations/42",
	} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d body=%s", target, rec.Code, rec.Body.String())
		}
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
