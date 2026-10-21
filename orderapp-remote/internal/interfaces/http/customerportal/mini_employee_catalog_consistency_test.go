package customerportal

import (
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

func TestMiniEmployeeRetailCustomerSeesAllSelectedClassifiedPriceTables(t *testing.T) {
	form := salesapp.OrderFormData{
		Customers:  []salesapp.CustomerOption{{ID: 363, DefaultOrderTypeID: 4}},
		OrderTypes: []salesapp.Option{{ID: 4, Name: "零售"}},
		Products: []salesapp.ProductOption{
			{ID: 1, Name: "烘焙商品", Visibility: "public", ProductKind: "roasted", Tiers: []salesapp.ProductTierOption{{ID: 11, PublicationID: 158, ListType: "commercial", UnitPrice: 38}}},
			{ID: 2, Name: "生豆商品", Visibility: "public", ProductKind: "green_bean", Tiers: []salesapp.ProductTierOption{{ID: 21, PublicationID: 73, ListType: "green", UnitPrice: 68}}},
		},
		BeanListVersionOptions: []salesapp.BeanListVersionOption{
			{ID: 158, CustomerID: 0, ClassificationTemplateID: 56, ClassificationTemplateName: "烘焙咖啡豆", ListType: "commercial", IsDefault: true},
			{ID: 73, CustomerID: 0, ClassificationTemplateID: 162, ClassificationTemplateName: "咖啡生豆", ListType: "green", IsDefault: true},
		},
	}
	e := echo.New()
	registerMiniEmployeeAPI(e, employeePortalService(), &miniEmployeeSalesFake{orderFormResult: &form})
	req := httptest.NewRequest(http.MethodGet, "/api/mini/employee/order-form?customer_id=363", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer token")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result struct {
		Families []map[string]any `json:"product_families"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Families) != 2 {
		t.Fatalf("only-green regression: families=%+v", result.Families)
	}
}
