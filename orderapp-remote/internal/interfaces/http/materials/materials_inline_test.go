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
		`materials: { title: '物料档案/库存' }`,
		`materials: MaterialsView`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("App.vue missing %q", want)
		}
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
		`copySelectedMaterial`,
		`deprecateSelectedMaterial`,
		`操作日志`,
	} {
		if !strings.Contains(viewSrc, want) {
			t.Fatalf("MaterialsView.vue missing %q", want)
		}
	}
}

func TestMaterialsAPIUpdateKeepsBaseFieldsImmutable(t *testing.T) {
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
		Unit:          "g",
		PurchasePrice: 10,
		SalePrice:     21,
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
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "copy material") {
		t.Fatalf("POST /api/materials/:id status = %d body=%s, want immutable field rejection", rec.Code, rec.Body.String())
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
