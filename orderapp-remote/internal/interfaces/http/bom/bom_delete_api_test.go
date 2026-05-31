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
	deactivatedBomProductID            int64
	listRows                           []bomapp.ListItem
	productRows                        []bomapp.Option
	detail                             bomapp.Detail
	syncedYield                        bomapp.SyncProductYieldCommand
	savedItem                          bomapp.SaveItemCommand
	deletedItem                        bomapp.DeleteItemCommand
	derivedOwned                       bomapp.DeriveOwnedCommand
	productionBomGroups                []bomapp.ProductionBomGroup
	includeInactiveProductionBomGroups bool
	updatedProductionBomGroup          bomapp.UpdateProductionBomGroupCommand
	disabledProductionBomGroupID       int64
	productionBomRows                  []bomapp.ProductionBomSummary
	productionBomDetail                bomapp.ProductionBomDetail
	copiedProductionBom                bomapp.ProductionBomSummary
	createdProductionBom               bomapp.ProductionBomSummary
	updatedProductionBom               bomapp.ProductionBomSummary
	createdProductionVersion           bomapp.ProductionBomVersion
	updatedProductionDraft             bomapp.ProductionBomVersion
	updatedProductionDraftCommand      bomapp.UpdateProductionBomVersionDraftCommand
	productBomBinding                  bomapp.ProductProductionBomBinding
	boundProductBom                    bomapp.BindProductProductionBomCommand
	publishedProductionVersionID       int64
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
func (r *apiFakeRepo) DeriveOwned(_ context.Context, cmd bomapp.DeriveOwnedCommand) (bomapp.Detail, error) {
	r.derivedOwned = cmd
	return bomapp.Detail{ProductID: cmd.ProductID, BomSourceType: "derived_owned", CanEditBOM: true}, nil
}

func (r *apiFakeRepo) SetBomSource(context.Context, bomapp.SetBomSourceCommand) (bomapp.Detail, error) {
	return bomapp.Detail{}, nil
}

func (r *apiFakeRepo) ListProductionBomGroups(_ context.Context, includeInactive bool) ([]bomapp.ProductionBomGroup, error) {
	r.includeInactiveProductionBomGroups = includeInactive
	if includeInactive {
		return r.productionBomGroups, nil
	}
	out := make([]bomapp.ProductionBomGroup, 0, len(r.productionBomGroups))
	for _, row := range r.productionBomGroups {
		if row.Active {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r *apiFakeRepo) CreateProductionBomGroup(_ context.Context, cmd bomapp.CreateProductionBomGroupCommand) (bomapp.ProductionBomGroup, error) {
	return bomapp.ProductionBomGroup{ID: 99, Name: cmd.Name, SortOrder: cmd.SortOrder, Active: true}, nil
}

func (r *apiFakeRepo) UpdateProductionBomGroup(_ context.Context, cmd bomapp.UpdateProductionBomGroupCommand) (bomapp.ProductionBomGroup, error) {
	r.updatedProductionBomGroup = cmd
	return bomapp.ProductionBomGroup{ID: cmd.ID, Name: cmd.Name, SortOrder: cmd.SortOrder, Active: true}, nil
}

func (r *apiFakeRepo) DisableProductionBomGroup(_ context.Context, cmd bomapp.DisableProductionBomGroupCommand) error {
	r.disabledProductionBomGroupID = cmd.ID
	return nil
}

func (r *apiFakeRepo) ListProductionBoms(context.Context) ([]bomapp.ProductionBomSummary, error) {
	return r.productionBomRows, nil
}

func (r *apiFakeRepo) GetProductionBomDetail(context.Context, int64) (bomapp.ProductionBomDetail, error) {
	return r.productionBomDetail, nil
}

func (r *apiFakeRepo) CreateProductionBom(_ context.Context, cmd bomapp.CreateProductionBomCommand) (bomapp.ProductionBomSummary, error) {
	if r.createdProductionBom.ID > 0 {
		return r.createdProductionBom, nil
	}
	return bomapp.ProductionBomSummary{ID: 98, Code: "BOM-098", Name: cmd.Name, GroupID: cmd.GroupID, Status: "active", LatestVersionNo: "V001"}, nil
}

func (r *apiFakeRepo) UpdateProductionBom(_ context.Context, cmd bomapp.UpdateProductionBomCommand) (bomapp.ProductionBomSummary, error) {
	if r.updatedProductionBom.ID > 0 {
		return r.updatedProductionBom, nil
	}
	return bomapp.ProductionBomSummary{ID: cmd.ID, Code: "BOM-098", Name: cmd.Name, GroupID: cmd.GroupID, Status: cmd.Status, LatestVersionNo: "V001"}, nil
}

func (r *apiFakeRepo) CopyProductionBom(context.Context, bomapp.CopyProductionBomCommand) (bomapp.ProductionBomSummary, error) {
	return r.copiedProductionBom, nil
}

func (r *apiFakeRepo) CreateProductionBomVersion(context.Context, bomapp.CreateProductionBomVersionCommand) (bomapp.ProductionBomVersion, error) {
	return r.createdProductionVersion, nil
}

func (r *apiFakeRepo) UpdateProductionBomVersionDraft(_ context.Context, cmd bomapp.UpdateProductionBomVersionDraftCommand) (bomapp.ProductionBomVersion, error) {
	r.updatedProductionDraftCommand = cmd
	if r.updatedProductionDraft.ID > 0 {
		return r.updatedProductionDraft, nil
	}
	return bomapp.ProductionBomVersion{ID: cmd.VersionID, Status: "draft", SpecialAttrsSchemaJSON: cmd.SpecialAttrsSchemaJSON, SpecialAttrsJSON: cmd.SpecialAttrsJSON}, nil
}

func (r *apiFakeRepo) PublishProductionBomVersion(_ context.Context, cmd bomapp.PublishProductionBomVersionCommand) error {
	r.publishedProductionVersionID = cmd.VersionID
	return nil
}

func (r *apiFakeRepo) BindProductProductionBom(_ context.Context, cmd bomapp.BindProductProductionBomCommand) (bomapp.ProductProductionBomBinding, error) {
	r.boundProductBom = cmd
	return r.productBomBinding, nil
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
