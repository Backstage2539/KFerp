package costing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestPricingRuleDraftAPIBindsWholeDraft(t *testing.T) {
	e := echo.New()
	svc := &capturingPricingRuleTrialService{}
	RegisterRoutes(e, Dependencies{Costing: svc})
	body := `{"pricing_rule_id":7,"product_id":10,"customer_id":90,"bom_spec_id":101,"bom_variant_id":201,"pricing_rule_draft":{"id":7,"name":"草稿","margin_rate":0,"rounding_mode":"yuan","calculation_json":{"tax_mode":"none","other_costs":{}}}}`
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trial", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	wire, _ := json.Marshal(svc.last)
	var got map[string]any
	_ = json.Unmarshal(wire, &got)
	draft, ok := got["pricing_rule_draft"].(map[string]any)
	if !ok || draft["rounding_mode"] != "yuan" || draft["margin_rate"] != float64(0) {
		t.Fatalf("draft dropped: %s", wire)
	}
	if svc.last.CustomerID != 90 || svc.last.BomSpecID != 101 || svc.last.BomVariantID != 201 {
		t.Fatalf("identity dropped: %+v", svc.last)
	}
}

func TestPricingRuleDraftBatchAPIRejectsDraftBeforeBatchWork(t *testing.T) {
	e := echo.New()
	svc := &capturingPricingRuleTrialBatchService{}
	RegisterRoutes(e, Dependencies{Costing: svc})
	body := `{"requests":[{"pricing_rule_id":7,"product_id":10,"pricing_rule_draft":{"id":7,"name":"草稿"}}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/costing/pricing-rule-trials", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "only supported by single") {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if svc.calls != 0 {
		t.Fatalf("batch service calls=%d, want 0", svc.calls)
	}
}
