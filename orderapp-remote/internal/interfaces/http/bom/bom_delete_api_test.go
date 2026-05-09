package bom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	bomapp "orderapp/internal/application/bom"

	"github.com/labstack/echo/v4"
)

type apiFakeRepo struct {
	deactivatedBomProductID int64
}

func (r *apiFakeRepo) List(context.Context) ([]bomapp.ListItem, error) { return nil, nil }
func (r *apiFakeRepo) Detail(context.Context, int64) (bomapp.Detail, error) {
	return bomapp.Detail{}, nil
}
func (r *apiFakeRepo) Products(context.Context) ([]bomapp.Option, error)  { return nil, nil }
func (r *apiFakeRepo) Materials(context.Context) ([]bomapp.Option, error) { return nil, nil }
func (r *apiFakeRepo) BagSpecMappings(context.Context) ([]bomapp.BagSpecMapping, error) {
	return nil, nil
}
func (r *apiFakeRepo) SyncProductYield(context.Context, int64) error { return nil }
func (r *apiFakeRepo) DeactivateBom(_ context.Context, productID int64) error {
	r.deactivatedBomProductID = productID
	return nil
}
func (r *apiFakeRepo) SaveItem(context.Context, bomapp.SaveItemCommand) error {
	return nil
}
func (r *apiFakeRepo) DeleteItem(context.Context, bomapp.DeleteItemCommand) error {
	return nil
}
func (r *apiFakeRepo) SaveBagSpecMapping(context.Context, bomapp.SaveBagSpecMappingCommand) error {
	return nil
}
func (r *apiFakeRepo) DeleteBagSpecMapping(context.Context, int64) error { return nil }
func (r *apiFakeRepo) ListVersions(context.Context, int64) ([]bomapp.Version, error) {
	return nil, nil
}
func (r *apiFakeRepo) CreateVersion(context.Context, bomapp.CreateVersionCommand) (bomapp.Version, error) {
	return bomapp.Version{}, nil
}
func (r *apiFakeRepo) ActivateVersion(context.Context, int64) error { return nil }

func TestBomDeleteAPIInvalidatesCurrentBom(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodDelete, "/api/bom/7", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.deactivatedBomProductID != 7 {
		t.Fatalf("deactivated product id = %d, want 7", repo.deactivatedBomProductID)
	}
}
