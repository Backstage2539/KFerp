package catalog

import (
	"bytes"
	"context"
	"github.com/labstack/echo/v4"
	"net/http"
	"net/http/httptest"
	app "orderapp/internal/application/catalog"
	"testing"
)

func (*productSettingsRepo) ListSpecPricingGroups(context.Context) ([]app.SpecPricingGroup, error) {
	return []app.SpecPricingGroup{{ID: 3, Name: "测试", Revision: 1, Active: true}}, nil
}
func (*productSettingsRepo) SaveSpecPricingGroup(_ context.Context, g app.SpecPricingGroup) (app.SpecPricingGroup, error) {
	g.ID = 3
	g.Revision++
	return g, nil
}
func (*productSettingsRepo) SpecPricingGroupOptions(context.Context) (app.SpecPricingOptions, error) {
	return app.SpecPricingOptions{Templates: []app.SpecPricingOptionTemplate{{ID: 1, Name: "规格模板"}}}, nil
}
func TestSpecPricingGroupAPI(t *testing.T) {
	e := echo.New()
	registerProductRoutes(e, app.NewService(&productSettingsRepo{}))
	for _, tc := range []struct {
		method, path, body, want string
		status                   int
	}{
		{"GET", "/api/spec-pricing-groups", "", `"name":"测试"`, 200},
		{"GET", "/api/spec-pricing-groups/options", "", `"name":"规格模板"`, 200},
		{"POST", "/api/spec-pricing-groups", `{"name":"测试","spec_template_id":1,"spec_template_version_id":2,"active":true,"items":[{"spec_key":"227g","inventory_unit":"袋","pricing_rule_id":7}]}`, `"revision":1`, 200},
		{"POST", "/api/spec-pricing-groups", `{"name":"测试","spec_template_id":1,"spec_template_version_id":2,"items":[]}`, `error`, 400},
	} {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status || !bytes.Contains(rec.Body.Bytes(), []byte(tc.want)) {
			t.Errorf("%s %s: status %d body %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	_ = http.MethodGet
}
