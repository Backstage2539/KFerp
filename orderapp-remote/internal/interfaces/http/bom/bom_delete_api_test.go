package bom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"

	"github.com/labstack/echo/v4"
)

type apiFakeRepo struct {
	deactivatedBomProductID int64
	listRows                []bomapp.ListItem
	productRows             []bomapp.Option
	detail                  bomapp.Detail
	syncedYield             bomapp.SyncProductYieldCommand
	savedItem               bomapp.SaveItemCommand
	deletedItem             bomapp.DeleteItemCommand
}

func (r *apiFakeRepo) List(context.Context) ([]bomapp.ListItem, error) { return r.listRows, nil }
func (r *apiFakeRepo) Detail(context.Context, int64) (bomapp.Detail, error) {
	return r.detail, nil
}
func (r *apiFakeRepo) Products(context.Context) ([]bomapp.Option, error)  { return r.productRows, nil }
func (r *apiFakeRepo) Materials(context.Context) ([]bomapp.Option, error) { return nil, nil }
func (r *apiFakeRepo) BagSpecMappings(context.Context) ([]bomapp.BagSpecMapping, error) {
	return nil, nil
}
func (r *apiFakeRepo) SyncProductYield(_ context.Context, cmd bomapp.SyncProductYieldCommand) error {
	r.syncedYield = cmd
	return nil
}
func (r *apiFakeRepo) DeactivateBom(_ context.Context, cmd bomapp.DeactivateBomCommand) error {
	r.deactivatedBomProductID = cmd.ProductID
	return nil
}
func (r *apiFakeRepo) SaveItem(_ context.Context, cmd bomapp.SaveItemCommand) error {
	r.savedItem = cmd
	return nil
}
func (r *apiFakeRepo) DeleteItem(_ context.Context, cmd bomapp.DeleteItemCommand) error {
	r.deletedItem = cmd
	return nil
}
func (r *apiFakeRepo) SaveBagSpecMapping(context.Context, bomapp.SaveBagSpecMappingCommand) error {
	return nil
}
func (r *apiFakeRepo) DeleteBagSpecMapping(context.Context, bomapp.DeleteBagSpecMappingCommand) error {
	return nil
}
func (r *apiFakeRepo) ListVersions(context.Context, int64) ([]bomapp.Version, error) {
	return nil, nil
}
func (r *apiFakeRepo) CreateVersion(context.Context, bomapp.CreateVersionCommand) (bomapp.Version, error) {
	return bomapp.Version{}, nil
}
func (r *apiFakeRepo) ActivateVersion(context.Context, bomapp.ActivateVersionCommand) error {
	return nil
}

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

func TestBomDeleteItemAPIPassesProductID(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/bom/item/delete", strings.NewReader(`{"product_id":7,"id":9}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.deletedItem.ProductID != 7 || repo.deletedItem.ID != 9 {
		t.Fatalf("deleted item command = %+v, want product 7 item 9", repo.deletedItem)
	}
}

func TestBomDeleteItemAPIRequiresProductID(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/bom/item/delete", strings.NewReader(`{"id":9}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "product_id required") {
		t.Fatalf("body missing product_id error: %s", rec.Body.String())
	}
}
