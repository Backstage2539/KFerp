package main

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

	"github.com/labstack/echo/v4"
)

func TestNormalizeMaterialInputRejectsInvalidValues(t *testing.T) {
	_, err := normalizeMaterialInput(MaterialInput{
		Code:          "bean-a",
		Name:          "豆子A",
		Kind:          "bean",
		Unit:          "g",
		PurchasePrice: -1,
	})
	if err == nil || !strings.Contains(err.Error(), "negative price") {
		t.Fatalf("normalizeMaterialInput() error = %v, want negative price", err)
	}
}

func TestNormalizeMaterialInputDefaultsKindAndUnit(t *testing.T) {
	got, err := normalizeMaterialInput(MaterialInput{Code: " m-1 ", Name: " 物料1 "})
	if err != nil {
		t.Fatal(err)
	}
	if got.Code != "m-1" || got.Name != "物料1" || got.Kind != "other" || got.Unit != "g" {
		t.Fatalf("normalizeMaterialInput() = %+v", got)
	}
}

func TestVueShellUsesInternalMaterialsView(t *testing.T) {
	app, err := os.ReadFile("frontend-vue-shell/src/App.vue")
	if err != nil {
		t.Fatal(err)
	}
	src := string(app)
	for _, want := range []string{
		`import MaterialsView from './views/MaterialsView.vue'`,
		`materials: { title: '物料档案/库存', url: '/vue-shell?view=materials', internal: true }`,
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
		`v-model.trim="row.code"`,
		`v-model.number="row.onhand_g"`,
		`操作日志`,
	} {
		if !strings.Contains(viewSrc, want) {
			t.Fatalf("MaterialsView.vue missing %q", want)
		}
	}
}

func TestMaterialsAPIInlineUpdateWritesAuditLog(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	if err := upsertMaterial(ctx, pool, schema, "m-api-1", "测试物料", "bean", "g", 10, 20, 1000, 0, 100, 0); err != nil {
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
	registerMaterialsAPI(e, pool, schema)

	body, err := json.Marshal(MaterialInput{
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
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/materials/:id status = %d body=%s", rec.Code, rec.Body.String())
	}

	var count int
	q := fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE actor=$1 AND entity_type='material' AND entity_id=$2 AND action='update' AND field IN ('name','sale_price')`, schema)
	if err := pool.QueryRow(ctx, q, "api-test", id).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("audit log count = %d, want 2", count)
	}
}
