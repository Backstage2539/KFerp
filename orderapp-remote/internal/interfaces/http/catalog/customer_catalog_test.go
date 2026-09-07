package catalog

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/catalog"
	"strings"
	"testing"
)

type customerCatalogAPIRepo struct {
	productSettingsRepo
	command app.CopyCustomerCatalogCommand
}

func (r *customerCatalogAPIRepo) CopyCustomerCatalog(_ context.Context, c app.CopyCustomerCatalogCommand) (app.CopyCustomerCatalogResult, error) {
	r.command = c
	return app.CopyCustomerCatalogResult{CustomerID: c.CustomerID, Created: 1}, nil
}
func (r *customerCatalogAPIRepo) CustomerCatalog(_ context.Context, c int64) (app.CustomerCatalog, error) {
	return app.CustomerCatalog{CustomerID: c}, nil
}
func (r *customerCatalogAPIRepo) RenameCustomerCatalogNode(context.Context, app.RenameCustomerCatalogNodeCommand) error {
	return nil
}
func (r *customerCatalogAPIRepo) MigrateCustomerCatalog(context.Context, bool, string) (app.CustomerCatalogMigrationResult, error) {
	return app.CustomerCatalogMigrationResult{}, nil
}
func TestCustomerCatalogCopyAPI(t *testing.T) {
	r := &customerCatalogAPIRepo{}
	e := echo.New()
	registerProductRoutes(e, app.NewService(r))
	for _, tc := range []struct {
		body   string
		status int
	}{{`{"customer_id":42,"mode":"all"}`, 200}, {`{"customer_id":42,"mode":"selected","product_ids":[1]}`, 200}, {`{"customer_id":42,"mode":"selected"}`, 400}, {`{"customer_id":0,"mode":"all"}`, 400}} {
		req := httptest.NewRequest(http.MethodPost, "/api/product-settings/customer-catalog/copy", strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
	}
}
