package materials

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	materialsapp "orderapp/internal/application/materials"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"

	"github.com/labstack/echo/v4"
)

func TestVueShellUsesInternalMaterialsView(t *testing.T) {
	app, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	src := string(app)
	for _, want := range []string{
		`import MaterialsView from './views/MaterialsView.vue'`,
		`materials: MaterialsView`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("App.vue missing %q", want)
		}
	}
	menuIA, err := os.ReadFile("frontend-vue-shell/src/lib/menu-ia.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(menuIA), `key: 'materials'`) || !strings.Contains(string(menuIA), `物料档案`) {
		t.Fatal("menu-ia.js missing materials primary menu entry")
	}

	view, err := os.ReadFile("frontend-vue-shell/src/views/MaterialsView.vue")
	if err != nil {
		t.Fatal(err)
	}
	viewSrc := string(view)
	for _, want := range []string{
		`/api/materials`,
		`saveMaterial`,
		`selectMaterial(row)`,
		`deprecateSelectedMaterial`,
		`操作日志`,
	} {
		if !strings.Contains(viewSrc, want) {
			t.Fatalf("MaterialsView.vue missing %q", want)
		}
	}
	if strings.Contains(viewSrc, `copySelectedMaterial`) {
		t.Fatal("MaterialsView.vue must not restore copySelectedMaterial")
	}
}

func TestMaterialsAPIUpdateAllowsBaseFieldsAndWritesAudit(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,purchase_price,sale_price,onhand_g,onhand_units,min_level_g,min_level_units,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,now())`, schema), "m-api-1", "测试物料", "bean", "g", 10, 20, 1000, 0, 100, 0); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE code=$1`, schema), "m-api-1").Scan(&id); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "api-test")
			return next(c)
		}
	})
	registerMaterialsAPI(e, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))

	body, err := json.Marshal(materialsapp.MaterialInput{
		Code:          "m-api-1",
		Name:          "测试物料改名",
		Kind:          "bean",
		Unit:          "kg",
		PurchasePrice: 12,
		SalePrice:     20,
		OnhandG:       1000,
		OnhandUnits:   0,
		MinLevelG:     100,
		MinLevelUnits: 0,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d", id), bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/materials/:id status = %d body=%s", rec.Code, rec.Body.String())
	}
	var updated materialsapp.Material
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Name != "测试物料改名" || updated.Unit != "kg" || updated.PurchasePrice != 12 {
		t.Fatalf("updated material = %+v", updated)
	}
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='material' AND entity_id=$1 AND action='update' AND field IN ('name','unit','purchase_price')`, schema), id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count < 3 {
		t.Fatalf("audit update count = %d, want at least 3", count)
	}
}

func TestMaterialsAPIUpdateRejectsInlineStockChange(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,batch_no,purchase_price,sale_price,onhand_g,onhand_units,min_level_g,min_level_units,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())`, schema), "m-api-stock-1", "库存物料", "bean", "g", "20260427", 10, 20, 1000, 3, 100, 1); err != nil {
		t.Fatal(err)
	}
	var id int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE code=$1`, schema), "m-api-stock-1").Scan(&id); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "api-test")
			return next(c)
		}
	})
	registerMaterialsAPI(e, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))

	body, err := json.Marshal(materialsapp.MaterialInput{
		Code:          "m-api-stock-1",
		Name:          "库存物料",
		Kind:          "bean",
		Unit:          "g",
		BatchNo:       "20260427",
		PurchasePrice: 10,
		SalePrice:     20,
		OnhandG:       1200,
		OnhandUnits:   3,
		MinLevelG:     100,
		MinLevelUnits: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d", id), bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "stock adjustment") {
		t.Fatalf("POST /api/materials/:id status = %d body=%s, want stock adjustment rejection", rec.Code, rec.Body.String())
	}
}

func TestMaterialsAPIClassificationAndIndustryFields(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "api-test")
			return next(c)
		}
	})
	registerMaterialsAPI(e, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))

	req := httptest.NewRequest(http.MethodPost, "/api/material-classification-groups", strings.NewReader(`{"name":"咖啡生豆","sort_order":10}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST group status=%d body=%s", rec.Code, rec.Body.String())
	}
	var group materialsapp.MaterialClassificationGroup
	if err := json.Unmarshal(rec.Body.Bytes(), &group); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/material-classification-groups/%d/categories", group.ID), strings.NewReader(`{"name":"云南","sort_order":20}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST category status=%d body=%s", rec.Code, rec.Body.String())
	}
	var category materialsapp.MaterialClassificationCategory
	if err := json.Unmarshal(rec.Body.Bytes(), &category); err != nil {
		t.Fatal(err)
	}

	createBody := `{"code":"green-1","name":"云南生豆","unit":"kg","industry_field_template_id":3,"industry_fields":[{"field_key":"产地","value_text":"云南"}]}`
	req = httptest.NewRequest(http.MethodPost, "/api/materials", strings.NewReader(createBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST material status=%d body=%s", rec.Code, rec.Body.String())
	}
	var material materialsapp.Material
	if err := json.Unmarshal(rec.Body.Bytes(), &material); err != nil {
		t.Fatal(err)
	}
	if material.IndustryFieldTemplateID != 3 || len(material.IndustryFields) != 1 || material.IndustryFields[0].FieldKey != "产地" {
		t.Fatalf("created material industry fields = %+v template=%d", material.IndustryFields, material.IndustryFieldTemplateID)
	}

	assign := fmt.Sprintf(`{"material_ids":[%d],"group_id":%d,"category_id":%d}`, material.ID, group.ID, category.ID)
	req = httptest.NewRequest(http.MethodPost, "/api/material-classification-assignments", strings.NewReader(assign))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST assignment status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/materials?active=all&q=云南", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET materials status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"classification_group_name":"咖啡生豆"`) || !strings.Contains(rec.Body.String(), `"classification_category_name":"云南"`) {
		t.Fatalf("material classification missing in response: %s", rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/material-classification-group-categories/%d", category.ID), nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE category status=%d body=%s", rec.Code, rec.Body.String())
	}
	req = httptest.NewRequest(http.MethodGet, "/api/materials?active=all&q=云南", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"classification_category_id":0`) {
		t.Fatalf("deleted category should move material to group unclassified: %s", rec.Body.String())
	}
}

func TestMaterialsAPICreateCopyDeprecateAndPackProfile(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.materials(code,name,kind,unit,batch_no,purchase_price,sale_price,onhand_g,onhand_units,min_level_g,min_level_units,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now())`, schema), "pack-old", "旧袋子", "pack", "个", "20260427", 1.2, 2.5, 0, 100, 0, 20); err != nil {
		t.Fatal(err)
	}
	var sourceID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE code=$1`, schema), "pack-old").Scan(&sourceID); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "api-test")
			return next(c)
		}
	})
	registerMaterialsAPI(e, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))

	createBody, err := json.Marshal(materialsapp.MaterialInput{
		Code:          "pack-new",
		Name:          "新袋子",
		Kind:          "pack",
		Unit:          "个",
		BatchNo:       "20260428",
		PurchasePrice: 1.5,
		SalePrice:     2.8,
		OnhandUnits:   100,
		MinLevelUnits: 20,
		PackProfile: &materialsapp.PackProfile{
			SizeSpec:   "227g袋",
			Dimensions: "12x20cm",
			Material:   "牛皮纸",
			Capacity:   "227g",
			Color:      "白色",
			Note:       "带气阀",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/materials", bytes.NewReader(createBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/materials status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created materialsapp.Material
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 || created.PackProfile == nil || created.PackProfile.SizeSpec != "227g袋" || created.BeanProfile != nil {
		t.Fatalf("created material = %+v", created)
	}

	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d/deprecate", sourceID), bytes.NewReader([]byte(`{}`)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/materials/:id/deprecate status = %d body=%s", rec.Code, rec.Body.String())
	}
	var deprecated materialsapp.Material
	if err := json.Unmarshal(rec.Body.Bytes(), &deprecated); err != nil {
		t.Fatal(err)
	}
	if deprecated.DeprecatedAt == "" {
		t.Fatalf("deprecated_at empty: %+v", deprecated)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/materials?limit=20", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/materials status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "pack-old") {
		t.Fatalf("deprecated material should be hidden by default: %s", rec.Body.String())
	}
}
