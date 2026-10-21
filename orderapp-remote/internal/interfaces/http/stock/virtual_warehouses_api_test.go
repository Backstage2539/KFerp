package stock

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	stockapp "orderapp/internal/application/stock"
)

func TestVirtualWarehouseAPIRejectsInvalidCustomerFilter(t *testing.T) {
	e := echo.New()
	RegisterRoutes(e, Dependencies{Stock: stockapp.NewService(&fakeStockRepo{})})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stock/virtual-warehouses?customer_id=invalid", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid customer filter status=%d body=%s, want 400", rec.Code, rec.Body.String())
	}
}
