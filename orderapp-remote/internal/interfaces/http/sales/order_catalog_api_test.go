package sales

import (
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	salesapp "orderapp/internal/application/sales"
	"testing"
)

type catalogFormRepository struct {
	salesapp.Repository
	form salesapp.OrderFormData
}

func (r *catalogFormRepository) OrderForm(context.Context, int64) (salesapp.OrderFormData, error) {
	return r.form, nil
}

func TestOrderCatalogAPIUsesTheSharedPublicationProjection(t *testing.T) {
	form := salesapp.OrderFormData{Customers: []salesapp.CustomerOption{{ID: 487, DefaultOrderTypeID: 2}}, Products: []salesapp.ProductOption{{ID: 1, Name: "商品", Visibility: "public", ProductKind: "generic", Tiers: []salesapp.ProductTierOption{{PublicationID: 10, ListType: "commercial", UnitPrice: 38}, {PublicationID: 11, ListType: "retail", UnitPrice: 48}}}}, BeanListVersionOptions: []salesapp.BeanListVersionOption{{ID: 10, ClassificationTemplateID: 56, ClassificationTemplateName: "任意新类型", ListType: "commercial", ReleaseID: "C", IsDefault: true}, {ID: 11, ClassificationTemplateID: 56, ClassificationTemplateName: "任意新类型", ListType: "retail", ReleaseID: "R", IsDefault: true}}}
	e := echo.New()
	registerOrderAPI(e, salesapp.NewService(&catalogFormRepository{form: form}), nil)
	for _, tc := range []struct {
		query  string
		want   int64
		status int
	}{{"customer_id=487", 10, 200}, {"customer_id=487&selected_price_table_ids=11", 11, 200}, {"customer_id=487&selected_price_table_ids=999", 0, 400}, {"customer_id=999", 0, 404}, {"customer_id=0", 0, 400}} {
		req := httptest.NewRequest(http.MethodGet, "/api/order/catalog?"+tc.query, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Fatalf("%s: status=%d body=%s", tc.query, rec.Code, rec.Body.String())
		}
		if rec.Code != 200 {
			continue
		}
		var got struct {
			Families []map[string]any                 `json:"product_families"`
			IDs      []int64                          `json:"selected_price_table_ids"`
			Options  []salesapp.BeanListVersionOption `json:"price_table_options"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Families) != 1 || len(got.IDs) != 1 || got.IDs[0] != tc.want || len(got.Options) != 2 {
			t.Fatalf("response=%s", rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("order catalog must not be cached")
		}
	}
}
