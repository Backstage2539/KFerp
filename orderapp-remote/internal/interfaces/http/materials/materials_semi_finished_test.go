package materials

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	materialsapp "orderapp/internal/application/materials"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"

	"github.com/labstack/echo/v4"
)

func TestMaterialsAPIAutoZeroesSemiFinishedPriceAndAuditsTogglePostgres(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("actor", "semi-finished-test")
			return next(c)
		}
	})
	registerMaterialsAPI(e, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))

	create := httptest.NewRequest(http.MethodPost, "/api/materials", strings.NewReader(
		`{"code":"WIP-API","name":"半成品切换测试","kind":"bean","unit":"kg","purchase_price":288}`,
	))
	create.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	created := httptest.NewRecorder()
	e.ServeHTTP(created, create)
	if created.Code != http.StatusOK {
		t.Fatalf("create material status=%d body=%s", created.Code, created.Body.String())
	}
	var id int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.materials WHERE code='WIP-API'`, schema)).Scan(&id); err != nil {
		t.Fatal(err)
	}

	update := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d", id), strings.NewReader(
		`{"code":"WIP-API","name":"半成品切换测试","kind":"bean","unit":"kg","is_semi_finished":true}`,
	))
	update.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	updated := httptest.NewRecorder()
	e.ServeHTTP(updated, update)
	if updated.Code != http.StatusOK || !strings.Contains(updated.Body.String(), `"is_semi_finished":true`) || !strings.Contains(updated.Body.String(), `"purchase_price":0`) {
		t.Fatalf("toggle semi-finished status=%d body=%s", updated.Code, updated.Body.String())
	}

	var auditCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT count(*) FROM %s.audit_logs
		WHERE entity_type='material' AND entity_id=$1 AND actor='semi-finished-test'
		  AND action='update' AND field IN ('is_semi_finished','purchase_price')
	`, schema), id).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 2 {
		t.Fatalf("semi-finished toggle audit count = %d, want 2", auditCount)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.materials
		SET is_semi_finished=false,purchase_price=288
		WHERE id=$1
	`, schema), id); err != nil {
		t.Fatal(err)
	}
	explicitNonZero := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/materials/%d", id), strings.NewReader(
		`{"code":"WIP-API","name":"半成品切换测试","kind":"bean","unit":"kg","is_semi_finished":true,"purchase_price":288}`,
	))
	explicitNonZero.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	explicitRejected := httptest.NewRecorder()
	e.ServeHTTP(explicitRejected, explicitNonZero)
	if explicitRejected.Code != http.StatusBadRequest || !strings.Contains(explicitRejected.Body.String(), "半成品只能通过生产入库") {
		t.Fatalf("explicit non-zero semi-finished price status=%d body=%s", explicitRejected.Code, explicitRejected.Body.String())
	}
	var isSemiFinished bool
	var purchasePrice float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT is_semi_finished,purchase_price::float8 FROM %s.materials WHERE id=$1`, schema), id).Scan(&isSemiFinished, &purchasePrice); err != nil {
		t.Fatal(err)
	}
	if isSemiFinished || purchasePrice != 288 {
		t.Fatalf("rejected explicit non-zero update changed material semi/price=%t/%.2f", isSemiFinished, purchasePrice)
	}

	invalid := httptest.NewRequest(http.MethodPost, "/api/materials", strings.NewReader(
		`{"code":"WIP-INVALID","name":"非法半成品采购价","kind":"bean","unit":"kg","is_semi_finished":true,"purchase_price":1}`,
	))
	invalid.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rejected := httptest.NewRecorder()
	e.ServeHTTP(rejected, invalid)
	if rejected.Code != http.StatusBadRequest || !strings.Contains(rejected.Body.String(), "半成品只能通过生产入库") {
		t.Fatalf("invalid semi-finished price status=%d body=%s", rejected.Code, rejected.Body.String())
	}
}
