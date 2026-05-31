package bom

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"

	"github.com/labstack/echo/v4"
)

func TestProductionBomAPIsExposeGroupsCopyVersionsAndBinding(t *testing.T) {
	repo := &apiFakeRepo{
		productionBomGroups: []bomapp.ProductionBomGroup{
			{ID: 1, Name: "常用配方", Active: true, SortOrder: 10},
			{ID: 2, Name: "停用分组", Active: false, SortOrder: 99},
		},
		productionBomRows: []bomapp.ProductionBomSummary{{
			ID: 11, Code: "BOM-001", Name: "精品拼配", GroupID: 1, GroupName: "常用配方",
			LatestVersionID: 101, LatestVersionNo: "V003", Status: "active", ReferenceProductCount: 2,
		}},
		productionBomDetail: bomapp.ProductionBomDetail{
			ProductionBomSummary: bomapp.ProductionBomSummary{
				ID: 11, Code: "BOM-001", Name: "精品拼配", GroupID: 1, GroupName: "常用配方",
				LatestVersionID: 101, LatestVersionNo: "V003", Status: "active",
			},
			Versions: []bomapp.ProductionBomVersion{{
				ID: 101, BomID: 11, VersionNo: "V003", Status: "published", YieldRate: 0.82, IsLatest: true,
				SpecialAttrsSchemaJSON: `[{"key":"roast_level","label":"烘焙度","show_in_price_list":true}]`,
				SpecialAttrsJSON:       `{"roast_level":"中深烘"}`,
			}},
		},
		copiedProductionBom: bomapp.ProductionBomSummary{
			ID: 12, Code: "BOM-002", Name: "精品拼配-包装改版", GroupID: 1, GroupName: "常用配方",
			LatestVersionID: 102, LatestVersionNo: "V001", Status: "active",
		},
		createdProductionVersion: bomapp.ProductionBomVersion{ID: 103, BomID: 11, VersionNo: "V004", Status: "draft", YieldRate: 0.82},
		productBomBinding: bomapp.ProductProductionBomBinding{
			ProductID: 7, BomID: 11, BomCode: "BOM-001", BomName: "精品拼配",
			BomVersionID: 100, BomVersionNo: "V002", LatestBomVersionID: 101, LatestBomVersionNo: "V003",
			IsLatestBomVersion: false,
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	for _, tc := range []struct {
		method string
		path   string
		body   string
		want   []string
	}{
		{method: http.MethodGet, path: "/api/production-bom-groups", want: []string{`"name":"常用配方"`}},
		{method: http.MethodPut, path: "/api/production-bom-groups/1", body: `{"name":"常用配方改名","sort_order":20}`, want: []string{`"name":"常用配方改名"`, `"sort_order":20`}},
		{method: http.MethodPost, path: "/api/production-bom-groups/1/move", body: `{"sort_order":5}`, want: []string{`"ok":true`}},
		{method: http.MethodDelete, path: "/api/production-bom-groups/1", want: []string{`"ok":true`}},
		{method: http.MethodGet, path: "/api/production-boms", want: []string{`"code":"BOM-001"`, `"latest_version_no":"V003"`, `"reference_product_count":2`}},
		{method: http.MethodGet, path: "/api/production-boms/11", want: []string{`"versions"`, `"version_no":"V003"`, `"special_attrs_schema_json"`, `"special_attrs_json"`, `"is_latest":true`}},
		{method: http.MethodPost, path: "/api/production-boms/11/copy", body: `{"name":"精品拼配-包装改版","group_id":1}`, want: []string{`"code":"BOM-002"`, `"name":"精品拼配-包装改版"`}},
		{method: http.MethodPost, path: "/api/production-boms/11/versions", body: `{"note":"新版配方"}`, want: []string{`"version_no":"V004"`, `"status":"draft"`}},
		{method: http.MethodPut, path: "/api/production-bom-versions/103/draft", body: `{"expected_loss_rate":0.18,"special_attrs_schema_json":"[{\"key\":\"roast_level\",\"label\":\"烘焙度\",\"show_in_price_list\":true}]","special_attrs_json":"{\"roast_level\":\"深烘\"}","items":[]}`, want: []string{`"status":"draft"`, `"special_attrs_json":"{\"roast_level\":\"深烘\"}"`}},
		{method: http.MethodPost, path: "/api/production-bom-versions/103/publish", want: []string{`"ok":true`}},
		{method: http.MethodPut, path: "/api/products/7/production-bom-binding", body: `{"bom_id":11,"bom_version_id":100}`, want: []string{`"product_id":7`, `"production_bom_version_no":"V002"`, `"latest_bom_version_no":"V003"`, `"is_latest_bom_version":false`}},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s %s status = %d, body = %s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		for _, want := range tc.want {
			if !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("%s %s body missing %s: %s", tc.method, tc.path, want, rec.Body.String())
			}
		}
	}

	if repo.boundProductBom.ProductID != 7 || repo.boundProductBom.BomID != 11 || repo.boundProductBom.BomVersionID != 100 {
		t.Fatalf("binding command = %+v", repo.boundProductBom)
	}
	if repo.publishedProductionVersionID != 103 {
		t.Fatalf("published version id = %d, want 103", repo.publishedProductionVersionID)
	}
	if repo.updatedProductionBomGroup.ID != 1 || repo.updatedProductionBomGroup.Name != "常用配方改名" || repo.updatedProductionBomGroup.SortOrder != 20 {
		t.Fatalf("updated group command = %+v", repo.updatedProductionBomGroup)
	}
	if repo.movedProductionBomGroup.ID != 1 || repo.movedProductionBomGroup.SortOrder != 5 {
		t.Fatalf("moved group command = %+v", repo.movedProductionBomGroup)
	}
	if repo.deletedProductionBomGroupID != 1 {
		t.Fatalf("deleted group id = %d, want 1", repo.deletedProductionBomGroupID)
	}
	if !strings.Contains(repo.updatedProductionDraftCommand.SpecialAttrsJSON, "深烘") {
		t.Fatalf("draft special attrs command = %+v", repo.updatedProductionDraftCommand)
	}
}
