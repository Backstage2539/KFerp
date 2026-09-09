package materials

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	materialsapp "orderapp/internal/application/materials"
	postgresmaterials "orderapp/internal/infrastructure/postgres/materials"
)

func TestMaterialDeprecateRequiresZeroInventoryPostgres(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit) VALUES(88,'UG','生豆-乌干达（罗）','bean','kg','kg');
 CREATE TABLE %[1]s.material_batches(id BIGINT,material_id BIGINT,remaining_g BIGINT DEFAULT 0,remaining_units BIGINT DEFAULT 0);
 CREATE TABLE %[1]s.material_batch_locations(material_id BIGINT,qty_g BIGINT DEFAULT 0,qty_units BIGINT DEFAULT 0);
 CREATE TABLE %[1]s.stock_batches(item_type TEXT,item_id BIGINT,remaining_g BIGINT DEFAULT 0,remaining_units BIGINT DEFAULT 0);
 CREATE TABLE %[1]s.customer_inventory_items(item_type TEXT,item_id BIGINT,qty_g BIGINT DEFAULT 0,qty_units BIGINT DEFAULT 0);
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	registerMaterialsAPI(e, materialsapp.NewService(postgresmaterials.NewRepository(pool, schema)))
	cases := []struct{ name, sql string }{
		{"master_weight", `UPDATE %[1]s.materials SET onhand_g=60500 WHERE id=88`},
		{"master_units", `UPDATE %[1]s.materials SET onhand_units=2 WHERE id=88`},
		{"batch", `INSERT INTO %[1]s.material_batches(id,material_id,remaining_g) VALUES(1,88,60500)`},
		{"location_net_zero", `INSERT INTO %[1]s.material_batch_locations(material_id,qty_g) VALUES(88,10),(88,-10)`},
		{"stock_mirror", `INSERT INTO %[1]s.stock_batches(item_type,item_id,remaining_units) VALUES('material',88,4)`},
		{"customer", `INSERT INTO %[1]s.customer_inventory_items(item_type,item_id,qty_units) VALUES('material',88,2)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.materials SET onhand_g=0,onhand_units=0,deprecated_at=NULL WHERE id=88; TRUNCATE %[1]s.material_batches,%[1]s.material_batch_locations,%[1]s.stock_batches,%[1]s.customer_inventory_items,%[1]s.audit_logs;`, schema))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, fmt.Sprintf(tc.sql, schema)); err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/materials/88/deprecate", strings.NewReader(`{}`)))
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "库存调整") {
				t.Fatalf("nonzero inventory must block deprecation: %d %s", rec.Code, rec.Body.String())
			}
			var active bool
			var audits int
			if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT deprecated_at IS NULL,(SELECT count(*) FROM %[1]s.audit_logs) FROM %[1]s.materials WHERE id=88`, schema)).Scan(&active, &audits); err != nil || !active || audits != 0 {
				t.Fatalf("failed deprecation wrote state: %v %d %v", active, audits, err)
			}
		})
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %[1]s.materials SET onhand_g=0,onhand_units=0,deprecated_at=NULL WHERE id=88; TRUNCATE %[1]s.material_batches,%[1]s.material_batch_locations,%[1]s.stock_batches,%[1]s.customer_inventory_items,%[1]s.audit_logs;`, schema))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/materials/88/deprecate", strings.NewReader(`{}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("zero stock: %d %s", rec.Code, rec.Body.String())
	}
	var audits int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE action='deprecate' AND entity_id=88`, schema)).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audit=%d err=%v", audits, err)
	}
}
