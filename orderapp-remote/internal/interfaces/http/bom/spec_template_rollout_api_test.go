package bom

import (
	"context"
	"github.com/labstack/echo/v4"
	"net/http/httptest"
	bomapp "orderapp/internal/application/bom"
	"strings"
	"testing"
)

type rolloutAPIRepo struct {
	*apiFakeRepo
	received bomapp.TemplateRolloutCommand
}

func (r *rolloutAPIRepo) PreviewSpecTemplateRollout(_ context.Context, id int64) (bomapp.TemplateRolloutPreview, error) {
	return bomapp.TemplateRolloutPreview{TemplateVersionID: id, Rows: []bomapp.TemplateRolloutRow{{BomID: 1, Status: "ready"}}}, nil
}
func (r *rolloutAPIRepo) ApplySpecTemplateRollout(_ context.Context, c bomapp.TemplateRolloutCommand) (bomapp.TemplateRolloutResult, error) {
	r.received = c
	return bomapp.TemplateRolloutResult{BatchID: "batch", Rows: []bomapp.TemplateRolloutRow{{BomID: 1, Status: "updated", NewVersionID: 3}}}, nil
}
func TestSpecTemplateRolloutAPI(t *testing.T) {
	r := &rolloutAPIRepo{apiFakeRepo: &apiFakeRepo{}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(r)})
	for _, tc := range []struct {
		method, path, body string
		code               int
		contains           string
	}{
		{"GET", "/api/production-bom-spec-template-versions/9/rollout-preview", "", 200, `"template_version_id":9`},
		{"POST", "/api/production-bom-spec-template-versions/9/rollout", `{"targets":[{"bom_id":1,"expected_version_id":2}],"actor":"forged","version_id":99}`, 200, `"new_version_id":3`},
		{"POST", "/api/production-bom-spec-template-versions/9/rollout", `{"targets":[{"bom_id":1,"expected_version_id":2},{"bom_id":1,"expected_version_id":2}]}`, 400, "重复"},
		{"POST", "/api/production-bom-spec-template-versions/0/rollout", `{}`, 400, "invalid"},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Content-Type", "application/json")
		e.ServeHTTP(rec, req)
		if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.contains) {
			t.Fatalf("%s %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if r.received.VersionID != 9 || r.received.Actor == "forged" {
		t.Fatal(r.received)
	}
}
