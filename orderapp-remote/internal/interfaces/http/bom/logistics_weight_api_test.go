package bom

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	bomapp "orderapp/internal/application/bom"
)

func TestPR706BOMDraftAPIForwardsNullableManualLogisticsWeight(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})
	for _, raw := range []string{"227.125", "null", "0", "-1", "1e-8", "1000000001"} {
		body := `{"variants":[{"spec_key":"bag","name":"袋装","inventory_unit":"袋","is_default":true,"logistics_weight_g":` + raw + `}]}`
		req := httptest.NewRequest(http.MethodPut, "/api/production-bom-versions/103/draft", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if raw == "0" || raw == "-1" || raw == "1e-8" || raw == "1000000001" {
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "logistics_weight_g") {
				t.Fatalf("weight %s status=%d body=%s", raw, rec.Code, rec.Body.String())
			}
			continue
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("weight %s status=%d body=%s", raw, rec.Code, rec.Body.String())
		}
		weight := repo.updatedProductionDraftCommand.Variants[0].LogisticsWeightG
		if raw == "null" && weight != nil || raw != "null" && (weight == nil || *weight != 227.125) {
			t.Fatalf("weight %s forwarded=%v", raw, weight)
		}
	}
}

func TestPR706PublishedBOMSpecAPIReturnsManualWeightAndLegacyNull(t *testing.T) {
	weight := 227.125
	repo := &apiFakeRepo{productionBomPublishedSpecs: []bomapp.ProductionBomPublishedSpec{{BomSpecID: 1, LogisticsWeightG: &weight}, {BomSpecID: 2}}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/production-bom-product-specs/42", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"logistics_weight_g":227.125`) || !strings.Contains(rec.Body.String(), `"logistics_weight_g":null`) {
		t.Fatalf("published logistics weight status=%d body=%s", rec.Code, rec.Body.String())
	}
}
