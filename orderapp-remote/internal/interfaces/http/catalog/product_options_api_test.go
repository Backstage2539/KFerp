package catalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	catalogapp "orderapp/internal/application/catalog"

	"github.com/labstack/echo/v4"
)

func TestProductOptionsAPIUsesPagedIdentityPayload(t *testing.T) {
	repo := &productSettingsRepo{productOptions: catalogapp.ProductOptionPage{
		Rows:  []catalogapp.ProductOption{{ID: 527, Name: "埃塞日晒", SKUCode: "SKU-527", Active: true}},
		Total: 527, HasNext: true,
	}}
	e := echo.New()
	registerProductRoutes(e, catalogapp.NewService(repo))
	req := httptest.NewRequest(http.MethodGet, "/api/products/options?q=埃塞&page=2&limit=20", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload catalogapp.ProductOptionPage
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Total != 527 || payload.Page != 2 || payload.Limit != 20 || !payload.HasNext || len(payload.Rows) != 1 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Rows[0].Name != "埃塞日晒" || payload.Rows[0].SKUCode != "SKU-527" {
		t.Fatalf("unexpected row: %+v", payload.Rows[0])
	}
}

var _ interface {
	ListProductOptions(context.Context, catalogapp.ProductOptionQuery) (catalogapp.ProductOptionPage, error)
} = (*productSettingsRepo)(nil)
