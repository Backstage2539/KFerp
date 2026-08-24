package bom

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bomapp "orderapp/internal/application/bom"

	"github.com/labstack/echo/v4"
)

type apiComponentSpecUnitRepo struct {
	*apiFakeRepo
	specUnits map[int64]string
}

func (r *apiComponentSpecUnitRepo) ProductionBomSpecInventoryUnits(_ context.Context, specIDs []int64) (map[int64]string, error) {
	units := make(map[int64]string, len(specIDs))
	for _, specID := range specIDs {
		if unit, ok := r.specUnits[specID]; ok {
			units[specID] = unit
		}
	}
	return units, nil
}

func TestProductionBomAPIsExposeGroupsCopyVersionsAndBinding(t *testing.T) {
	repo := &apiFakeRepo{
		materialRows: []bomapp.Option{
			{ID: 7, Name: "拼配原料", InventoryUnit: "kg"},
			{ID: 8, Name: "包装袋", InventoryUnit: "个"},
		},
		productionBomGroups: []bomapp.ProductionBomGroup{
			{ID: 1, Name: "常用配方", Active: true, SortOrder: 10, Categories: []bomapp.ProductionBomGroupCategory{{ID: 31, GroupID: 1, Name: "浅烘", SortOrder: 10}}},
			{ID: 2, Name: "停用分组", Active: false, SortOrder: 99},
		},
		productionBomRows: []bomapp.ProductionBomSummary{{
			ID: 11, Code: "BOM-001", Name: "精品拼配", GroupID: 1, GroupName: "常用配方",
			GroupCategoryID: 31, GroupCategoryName: "浅烘",
			OutputProductID: 7, OutputProductName: "10条盒装速溶咖啡", OutputProductCode: "SKU-0007",
			LatestVersionID: 101, LatestVersionNo: "V003", Status: "active", ReferenceProductCount: 2,
		}},
		productionBomDetail: bomapp.ProductionBomDetail{
			ProductionBomSummary: bomapp.ProductionBomSummary{
				ID: 11, Code: "BOM-001", Name: "精品拼配", GroupID: 1, GroupName: "常用配方",
				GroupCategoryID: 31, GroupCategoryName: "浅烘",
				OutputProductID: 7, OutputProductName: "10条盒装速溶咖啡", OutputProductCode: "SKU-0007",
				LatestVersionID: 101, LatestVersionNo: "V003", Status: "active",
			},
			ReferencedProducts: []bomapp.ProductionBomReferencedProduct{
				{ProductID: 7, ProductName: "初晓2.5kg装", ProductCode: "SKU-0007", Active: true},
				{ProductID: 8, ProductName: "Karen贴牌", ProductCode: "SKU-0008", Active: false},
			},
			Versions: []bomapp.ProductionBomVersion{{
				ID: 101, BomID: 11, VersionNo: "V003", Status: "published", YieldRate: 0.82, OutputQty: 1, OutputUnit: "盒", IsLatest: true,
				ProcessRouteID: 77, ProcessRouteName: "挂耳包装路线", IsLatestUsable: true,
				SpecialAttrsSchemaJSON: `[{"key":"roast_level","label":"烘焙度","show_in_price_list":true}]`,
				SpecialAttrsJSON:       `{"roast_level":"中深烘"}`,
			}},
		},
		copiedProductionBom: bomapp.ProductionBomSummary{
			ID: 12, Code: "BOM-002", Name: "精品拼配-包装改版", GroupID: 1, GroupName: "常用配方",
			LatestVersionID: 102, LatestVersionNo: "V001", Status: "active",
		},
		createdProductionBom: bomapp.ProductionBomSummary{
			ID: 13, Code: "BOM-003", Name: "新配方", GroupID: 1, GroupName: "常用配方",
			GroupCategoryID: 31, GroupCategoryName: "浅烘",
			OutputProductID: 7, OutputProductName: "10条盒装速溶咖啡", OutputProductCode: "SKU-0007",
			LatestVersionID: 103, LatestVersionNo: "V001", LatestVersionStatus: "draft", Status: "active",
		},
		updatedProductionBom: bomapp.ProductionBomSummary{
			ID: 11, Code: "BOM-001", Name: "精品拼配改名", GroupID: 1, GroupName: "常用配方",
			GroupCategoryID: 0, GroupCategoryName: "",
			LatestVersionID: 101, LatestVersionNo: "V003", Status: "inactive",
		},
		createdProductionVersion:          bomapp.ProductionBomVersion{ID: 103, BomID: 11, VersionNo: "V004", Status: "draft", YieldRate: 0.82, OutputQty: 1, OutputUnit: "盒", ProcessRouteID: 77, ProcessRouteName: "挂耳包装路线"},
		createdProductionBomGroupCategory: bomapp.ProductionBomGroupCategory{ID: 32, GroupID: 1, Name: "中烘", SortOrder: 20},
		updatedProductionBomGroupCategory: bomapp.ProductionBomGroupCategory{ID: 31, GroupID: 1, Name: "浅中烘", SortOrder: 15},
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
	}{
		{method: http.MethodPut, path: "/api/production-bom-groups/1", body: `{"name":"常用配方改名","sort_order":20}`},
		{method: http.MethodPost, path: "/api/production-bom-groups/1/categories", body: `{"name":"中烘","sort_order":20}`},
		{method: http.MethodPut, path: "/api/production-bom-group-categories/31", body: `{"name":"浅中烘","sort_order":15}`},
		{method: http.MethodDelete, path: "/api/production-bom-group-categories/31"},
		{method: http.MethodPost, path: "/api/production-bom-groups/1/move", body: `{"sort_order":5}`},
		{method: http.MethodDelete, path: "/api/production-bom-groups/1"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		if tc.body != "" {
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusGone || !strings.Contains(rec.Body.String(), "production BOM groups are legacy readonly") {
			t.Fatalf("%s %s should be legacy readonly, status=%d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}

	for _, tc := range []struct {
		method string
		path   string
		body   string
		want   []string
	}{
		{method: http.MethodGet, path: "/api/production-bom-groups", want: []string{`"name":"常用配方"`, `"categories"`, `"name":"浅烘"`}},
		{method: http.MethodGet, path: "/api/production-boms", want: []string{`"code":"BOM-001"`, `"latest_version_no":"V003"`, `"reference_product_count":2`, `"output_product_id":7`, `"output_product_name":"10条盒装速溶咖啡"`, `"business_group_id":1`, `"group_item_id":31`, `"group_category_id":31`, `"group_category_name":"浅烘"`}},
		{method: http.MethodPost, path: "/api/production-boms", body: `{"name":"BOM000643 新配方 生产 BOM / V001","output_product_id":7,"output_qty":1,"output_unit":"盒","group_id":1,"group_category_id":31,"expected_loss_rate":0.25}`, want: []string{`"code":"BOM-003"`, `"name":"新配方"`, `"output_product_id":7`, `"status":"active"`, `"latest_version_status":"draft"`}},
		{method: http.MethodGet, path: "/api/production-boms/11?version_id=101", want: []string{`"versions"`, `"version_no":"V003"`, `"output_qty":1`, `"output_unit":"盒"`, `"process_route_id":77`, `"process_route_name":"挂耳包装路线"`, `"is_latest_usable":true`, `"special_attrs_schema_json"`, `"special_attrs_json"`, `"is_latest":true`, `"referenced_products"`, `"product_name":"初晓2.5kg装"`, `"active":false`, `"group_category_name":"浅烘"`}},
		{method: http.MethodPut, path: "/api/production-boms/11", body: `{"name":"BOM-000659 精品拼配改名 生产 BOM / V003","group_id":1,"group_category_id":0,"status":"inactive"}`, want: []string{`"name":"精品拼配改名"`, `"status":"inactive"`}},
		{method: http.MethodPost, path: "/api/production-boms/11/copy", body: `{"name":"BOM000643 精品拼配-包装改版 生产 BOM / V001","group_id":1}`, want: []string{`"code":"BOM-002"`, `"name":"精品拼配-包装改版"`}},
		{method: http.MethodPost, path: "/api/production-boms/11/versions", body: `{"note":"新版配方"}`, want: []string{`"version_no":"V004"`, `"status":"draft"`}},
		{method: http.MethodPut, path: "/api/production-bom-versions/103/draft", body: `{"material_loss_rate":0.2,"output_qty":1,"output_unit":"盒","process_route_id":77,"special_attrs_schema_json":"[{\"key\":\"roast_level\",\"label\":\"烘焙度\",\"show_in_price_list\":true}]","special_attrs_json":"{\"roast_level\":\"深烘\"}","items":[{"component_type":"material","material_id":7,"consume_unit":"ratio_pct","ratio_pct":40},{"component_type":"material","material_id":8,"consume_unit":"ratio_pct","ratio_pct":60}]}`, want: []string{`"status":"draft"`, `"output_unit":"盒"`, `"process_route_id":77`, `"special_attrs_json":"{\"roast_level\":\"深烘\"}"`}},
		{method: http.MethodPost, path: "/api/production-bom-versions/103/publish", want: []string{`"ok":true`}},
		{method: http.MethodPut, path: "/api/products/7/production-bom-binding", body: `{"default_production_bom_id":11}`, want: []string{`"product_id":7`, `"production_bom_id":11`, `"latest_bom_version_no":"V003"`}},
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

	if repo.boundProductBom.ProductID != 7 || repo.boundProductBom.BomID != 11 || repo.boundProductBom.BomVersionID != 0 {
		t.Fatalf("binding command = %+v", repo.boundProductBom)
	}
	if repo.publishedProductionVersionID != 103 {
		t.Fatalf("published version id = %d, want 103", repo.publishedProductionVersionID)
	}
	if repo.updatedProductionDraftCommand.MaterialLossRate == nil || *repo.updatedProductionDraftCommand.MaterialLossRate != 0.2 {
		t.Fatalf("draft material loss = %v, want 0.2", repo.updatedProductionDraftCommand.MaterialLossRate)
	}
	if items := repo.updatedProductionDraftCommand.Items; len(items) != 2 || items[0].MaterialLossRate != 0.2 || items[1].ConsumeUnit != "ratio_pct" || items[1].RatioPct != 60 || items[1].MaterialLossRate != 0.2 {
		t.Fatalf("all-ratio loss recipe draft = %+v", items)
	}
	if repo.updatedProductionBomGroup.ID != 0 || repo.movedProductionBomGroup.ID != 0 || repo.deletedProductionBomGroupID != 0 {
		t.Fatalf("legacy production BOM group writes should not reach repo: update=%+v move=%+v delete=%d", repo.updatedProductionBomGroup, repo.movedProductionBomGroup, repo.deletedProductionBomGroupID)
	}
	if repo.createdProductionBomCommand.Name != "新配方" || repo.createdProductionBomCommand.GroupID != 1 {
		t.Fatalf("created production bom command = %+v", repo.createdProductionBomCommand)
	}
	if repo.createdProductionBomCommand.GroupCategoryID != 31 {
		t.Fatalf("created production bom group category = %d, want 31", repo.createdProductionBomCommand.GroupCategoryID)
	}
	if repo.createdProductionBomCommand.OutputProductID != 7 || repo.createdProductionBomCommand.OutputQty != 1 || repo.createdProductionBomCommand.OutputUnit != "盒" {
		t.Fatalf("created production bom output command = %+v", repo.createdProductionBomCommand)
	}
	if repo.createdProductionBomCommand.ExpectedLossRate == nil || *repo.createdProductionBomCommand.ExpectedLossRate != 0 {
		t.Fatalf("legacy create loss must normalize to zero: %+v", repo.createdProductionBomCommand)
	}
	if repo.updatedProductionBomCommand.ID != 11 || repo.updatedProductionBomCommand.Name != "精品拼配改名" || repo.updatedProductionBomCommand.Status != "inactive" {
		t.Fatalf("updated production bom command = %+v", repo.updatedProductionBomCommand)
	}
	if repo.updatedProductionBomCommand.GroupCategoryID != 0 {
		t.Fatalf("updated production bom group category = %d, want 0", repo.updatedProductionBomCommand.GroupCategoryID)
	}
	if repo.createdProductionBomGroupCategoryCommand.GroupID != 0 || repo.updatedProductionBomGroupCategoryCommand.ID != 0 || repo.deletedProductionBomGroupCategoryID != 0 {
		t.Fatalf("legacy production BOM group category writes should not reach repo: create=%+v update=%+v delete=%d", repo.createdProductionBomGroupCategoryCommand, repo.updatedProductionBomGroupCategoryCommand, repo.deletedProductionBomGroupCategoryID)
	}
	if repo.copiedProductionBomCommand.ID != 11 || repo.copiedProductionBomCommand.Name != "精品拼配-包装改版" || repo.copiedProductionBomCommand.GroupID != 1 {
		t.Fatalf("copied production bom command = %+v", repo.copiedProductionBomCommand)
	}
	if !strings.Contains(repo.updatedProductionDraftCommand.SpecialAttrsJSON, "深烘") {
		t.Fatalf("draft special attrs command = %+v", repo.updatedProductionDraftCommand)
	}
	if repo.updatedProductionDraftCommand.OutputQty != 1 || repo.updatedProductionDraftCommand.OutputUnit != "盒" {
		t.Fatalf("draft output basis command = %+v", repo.updatedProductionDraftCommand)
	}
	if repo.updatedProductionDraftCommand.ProcessRouteID != 77 {
		t.Fatalf("draft process route command = %+v", repo.updatedProductionDraftCommand)
	}
	if repo.updatedProductionDraftCommand.ExpectedLossRate == nil || *repo.updatedProductionDraftCommand.ExpectedLossRate != 0 {
		t.Fatalf("legacy draft loss must normalize to zero: %+v", repo.updatedProductionDraftCommand)
	}
}

func TestProductionBomAPIReappliesPublishedSpecTemplateToDraftAtomically(t *testing.T) {
	repo := &apiFakeRepo{
		updatedProductionDraft: bomapp.ProductionBomVersion{
			ID: 103, BomID: 11, VersionNo: "V004", Status: "draft", OutputUnit: "袋",
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPost, "/api/production-bom-versions/103/spec-template", strings.NewReader(`{"spec_template_version_id":902,"main_input_material_id":7}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("reapply specification template status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":103`) || !strings.Contains(rec.Body.String(), `"status":"draft"`) {
		t.Fatalf("reapply specification template response=%s", rec.Body.String())
	}
	if repo.reappliedProductionDraftCommand.VersionID != 103 || repo.reappliedProductionDraftCommand.SpecTemplateVersionID != 902 || repo.reappliedProductionDraftCommand.MainInputMaterialID != 7 {
		t.Fatalf("reapply specification template command=%+v", repo.reappliedProductionDraftCommand)
	}
}

func TestProductionBomDraftAPIUsesSelectedComponentSpecInventoryUnit(t *testing.T) {
	baseRepo := &apiFakeRepo{
		productRows: []bomapp.Option{{ID: 77, Name: "父商品", InventoryUnit: "kg"}},
	}
	repo := &apiComponentSpecUnitRepo{apiFakeRepo: baseRepo, specUnits: map[int64]string{701: "盒"}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	request := func(consumeUnit string) *httptest.ResponseRecorder {
		body := `{"items":[{"component_type":"product","component_product_id":77,"component_bom_spec_id":701,"consume_unit":"` + consumeUnit + `","qty_per_unit":1}]}`
		req := httptest.NewRequest(http.MethodPut, "/api/production-bom-versions/103/draft", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	if rec := request("盒"); rec.Code != http.StatusOK {
		t.Fatalf("selected specification unit status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec := request("kg"); rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "component BOM specification inventory_unit") {
		t.Fatalf("parent product unit bypass status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestProductionBomDraftWorkspaceAPINormalizesRecipeBranchByOutputType(t *testing.T) {
	repo := &apiFakeRepo{}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPut, "/api/production-boms/11/draft-workspace", strings.NewReader(`{
		"name":"物料草稿","output_type":"material","output_id":95,"version_id":103,
		"items":[{"component_type":"material","material_id":7,"consume_unit":"kg","qty_per_unit":1}],
		"variants":[{"spec_key":"ignored","name":"不应传入物料分支","inventory_unit":"袋","is_default":true}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("draft workspace status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(repo.workspaceCommand.Version.Items) != 1 || repo.workspaceCommand.Version.Variants != nil {
		t.Fatalf("material workspace recipe branch = items=%+v variants=%#v", repo.workspaceCommand.Version.Items, repo.workspaceCommand.Version.Variants)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/production-boms/11/draft-workspace", strings.NewReader(`{
		"name":"商品草稿","output_type":"product","output_id":7,"version_id":103,
		"items":[{"component_type":"material","material_id":7,"consume_unit":"kg","qty_per_unit":1}],
		"variants":[{"spec_key":"spec-1","name":"227g","inventory_unit":"袋","is_default":true}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("product draft workspace status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.workspaceCommand.Version.Items != nil || len(repo.workspaceCommand.Version.Variants) != 1 {
		t.Fatalf("product workspace recipe branch = items=%#v variants=%+v", repo.workspaceCommand.Version.Items, repo.workspaceCommand.Version.Variants)
	}
}

func TestProductionBomDraftWorkspaceReturnsStablePublishedIdentityConflict(t *testing.T) {
	repo := &apiFakeRepo{workspaceErr: bomapp.ErrPublishedOutputIdentityImmutable}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})
	req := httptest.NewRequest(http.MethodPut, "/api/production-boms/11/draft-workspace", strings.NewReader(`{
		"name":"初晓","output_type":"material","output_id":95,"version_id":103,
		"output_qty":1,"output_unit":"kg","items":[]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("draft identity conflict status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"code":"published_output_identity_immutable"`) {
		t.Fatalf("draft identity conflict body=%s", rec.Body.String())
	}
}

func TestProductionBomReplacementDraftAPIAcceptsFullWorkspace(t *testing.T) {
	repo := &apiFakeRepo{materialRows: []bomapp.Option{{ID: 95, Name: "初晓-半成品", InventoryUnit: "kg"}}}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})
	req := httptest.NewRequest(http.MethodPost, "/api/production-boms/11/replacement-draft", strings.NewReader(`{
		"source_version_id":102,"name":"初晓","output_type":"material","output_id":95,
		"output_qty":1,"output_unit":"lb","material_loss_rate":0.195,
		"group_id":165,"group_category_id":885,
		"items":[{"component_type":"material","material_id":56,"consume_unit":"ratio_pct","ratio_pct":50}]
	}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("replacement draft status=%d body=%s", rec.Code, rec.Body.String())
	}
	got := repo.replacementCommand
	if got.SourceBomID != 11 || got.SourceVersionID != 102 || got.Workspace.Bom.OutputMaterialID != 95 || got.Workspace.Version.OutputUnit != "kg" {
		t.Fatalf("replacement command = %+v", got)
	}
	if !strings.Contains(rec.Body.String(), `"latest_version_status":"draft"`) {
		t.Fatalf("replacement response body=%s", rec.Body.String())
	}
}

func TestProductionBomUpdateDoesNotTouchGroupAssignmentWhenGroupFieldsOmitted(t *testing.T) {
	repo := &apiFakeRepo{
		updatedProductionBom: bomapp.ProductionBomSummary{
			ID: 11, Code: "BOM-001", Name: "只改名称", GroupID: 1, GroupName: "常用配方",
			GroupCategoryID: 31, GroupCategoryName: "浅烘", Status: "active",
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodPut, "/api/production-boms/11", strings.NewReader(`{"name":"只改名称","status":"active"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update production bom status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.updatedProductionBomCommand.ID != 11 || repo.updatedProductionBomCommand.Name != "只改名称" {
		t.Fatalf("updated production bom command = %+v", repo.updatedProductionBomCommand)
	}
	if repo.updatedProductionBomCommand.UpdateGroupAssignment {
		t.Fatalf("omitted group fields must not update business group assignment: %+v", repo.updatedProductionBomCommand)
	}
}

func TestProductionBomAPIUsesUnifiedMaterialOutputAndFilters(t *testing.T) {
	repo := &apiFakeRepo{
		materialRows: []bomapp.Option{{ID: 95, Name: "湿豆", InventoryUnit: "kg"}},
		productionBomRows: []bomapp.ProductionBomSummary{{
			ID: 21, Code: "BOM-021", Name: "湿豆配方", OutputType: "material", OutputID: 95,
			OutputName: "湿豆", OutputCode: "WIP-095", OutputUnit: "kg", OutputMaterialID: 95, OutputMaterialName: "湿豆", OutputMaterialCode: "WIP-095",
		}},
		createdProductionBom: bomapp.ProductionBomSummary{
			ID: 22, Code: "BOM-022", Name: "湿豆新配方", OutputType: "material", OutputID: 95,
			OutputName: "湿豆", OutputCode: "WIP-095", OutputUnit: "kg", OutputMaterialID: 95, OutputMaterialName: "湿豆", OutputMaterialCode: "WIP-095",
		},
	}
	e := echo.New()
	RegisterRoutes(e, Dependencies{Bom: bomapp.NewService(repo)})

	req := httptest.NewRequest(http.MethodGet, "/api/production-boms?output_type=material&output_id=95&component_type=product&component_id=88", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"output_type":"material"`) || !strings.Contains(rec.Body.String(), `"output_material_id":95`) {
		t.Fatalf("filtered list status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.productionBomFilter != (bomapp.ProductionBomFilter{OutputType: "material", OutputID: 95, ComponentType: "product", ComponentID: 88}) {
		t.Fatalf("filter = %+v", repo.productionBomFilter)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/production-boms", strings.NewReader(`{"name":"湿豆新配方","output_type":"material","output_material_id":95,"output_qty":1,"output_unit":"g"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create material BOM status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.createdProductionBomCommand.OutputType != "material" || repo.createdProductionBomCommand.OutputID != 95 || repo.createdProductionBomCommand.OutputMaterialID != 95 || repo.createdProductionBomCommand.OutputProductID != 0 || repo.createdProductionBomCommand.OutputUnit != "kg" {
		t.Fatalf("create command = %+v", repo.createdProductionBomCommand)
	}

	req = httptest.NewRequest(http.MethodPut, "/api/materials/95/default-production-bom", strings.NewReader(`{"production_bom_id":22}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"is_default":true`) {
		t.Fatalf("bind material default status=%d body=%s", rec.Code, rec.Body.String())
	}
	if repo.boundProductionBomOutput.OutputType != "material" || repo.boundProductionBomOutput.OutputID != 95 || repo.boundProductionBomOutput.BomID != 22 {
		t.Fatalf("bound output command = %+v", repo.boundProductionBomOutput)
	}
}
