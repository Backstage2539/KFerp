package costing

import (
	"context"
	"encoding/json"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	appcosting "orderapp/internal/application/costing"
	"testing"
)

type editorPublicationService struct {
	fakeService
	query appcosting.BeanListPublicationQuery
	row   appcosting.BeanListPublication
}

func (s *editorPublicationService) LoadBeanListPublication(_ context.Context, query appcosting.BeanListPublicationQuery, id int64) (*appcosting.BeanListPublication, error) {
	s.query = query
	return &s.row, nil
}

// The editor depends on public persisted fields, not browser-private draft fields.
func TestPublicationEditorDetailPreservesCanonicalConfigAndFinalPriceSnapshots(t *testing.T) {
	svc := &editorPublicationService{row: appcosting.BeanListPublication{
		ID: 152, Version: "V3.0.14", Status: "published", OwnerType: "customer", OwnerKey: "471",
		PublicationTableMetadata: appcosting.PublicationTableMetadata{ReleaseID: "release", TableKey: "drip", TableName: "挂耳盒装"},
		Config: map[string]any{
			"product_spec_selections":       []any{map[string]any{"product_id": 940, "bom_id": 18277, "bom_version_id": 423, "bom_spec_id": 309, "bom_variant_id": 607}},
			"price_list_template_selection": map[string]any{"defaults": map[string]any{"pricing_mode": "fixed_price"}},
			"customizers":                   map[string]any{"title": "挂耳盒装"},
		},
		Content: map[string]any{"price_rows": []any{
			map[string]any{"product_id": 940, "bom_spec_id": 309, "bom_variant_id": 607, "pricing_mode": "fixed_price", "final_unit_price": 32.8, "price_unit": "盒"},
			map[string]any{"product_id": 940, "bom_spec_id": 310, "bom_variant_id": 608, "pricing_mode": "fixed_price", "final_unit_price": 40.2, "price_unit": "袋"},
		}},
	}}
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error { c.Set("basic_auth_admin", true); return next(c) }
	})
	RegisterRoutes(e, Dependencies{Costing: svc})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/costing/bean-list/publications/152?list_type=drip&scope=customer&customer_id=471&classification_template_id=163&publication_purpose=factory_supply", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result appcosting.BeanListPublication
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.ID != 152 {
		t.Fatalf("detail response missing row: %s", rec.Body.String())
	}
	for name, pair := range map[string][2]any{"config": {svc.row.Config, result.Config}, "content": {svc.row.Content, result.Content}} {
		before, _ := json.Marshal(pair[0])
		after, _ := json.Marshal(pair[1])
		if string(before) != string(after) {
			t.Fatalf("%s changed: %s != %s", name, before, after)
		}
	}
	if svc.query.OwnerKey != "471" || svc.query.ClassificationTemplateID != 163 || svc.query.PublicationPurpose != "factory_supply" {
		t.Fatalf("wrong customer/type scope: %+v", svc.query)
	}
}
