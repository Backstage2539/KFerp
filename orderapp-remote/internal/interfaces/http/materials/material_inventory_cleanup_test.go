package materials

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestOrphanMaterialInventoryCleanupPostgres(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, fmt.Sprintf(`
 INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,onhand_g,deprecated_at) VALUES(1,'ACTIVE','Active','bean','kg','kg',500,NULL),(2,'DELETED','Deleted','bean','kg','kg',600,now());
 CREATE TABLE %[1]s.material_batches(id BIGINT,material_id BIGINT,remaining_g BIGINT DEFAULT 0,remaining_units BIGINT DEFAULT 0,qty_g BIGINT,status TEXT);
 CREATE TABLE %[1]s.material_batch_locations(material_batch_id BIGINT,material_id BIGINT,warehouse TEXT,qty_g BIGINT DEFAULT 0,qty_units BIGINT DEFAULT 0);
 CREATE TABLE %[1]s.stock_batches(id BIGINT,item_type TEXT,item_id BIGINT,remaining_g BIGINT DEFAULT 0,remaining_units BIGINT DEFAULT 0,qty_g BIGINT);
 CREATE TABLE %[1]s.customer_inventory_items(id BIGINT,item_type TEXT,item_id BIGINT,qty_g BIGINT DEFAULT 0,qty_units BIGINT DEFAULT 0);
 INSERT INTO %[1]s.material_batches VALUES(1,1,500,0,1000,'active'),(2,2,600,0,1000,'active'),(3,3,700,0,1000,'active');
 INSERT INTO %[1]s.material_batch_locations VALUES(1,1,'raw_materials',500,0),(2,2,'raw_materials',600,0),(3,3,'wip',700,0);
 INSERT INTO %[1]s.stock_batches VALUES(1,'material',1,500,0,1000),(2,'material',2,600,0,1000),(3,'material',3,700,0,1000),(4,'product',2,100,0,100);
 INSERT INTO %[1]s.customer_inventory_items VALUES(1,'material',1,500,0),(2,'material',2,600,0),(3,'material',3,700,0);
 `, schema))
	if err != nil {
		t.Fatal(err)
	}
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = os.Getenv("DATABASE_URL")
	}
	run := func(apply bool, manifest string) (string, error) {
		output, err := exec.Command("psql", dsn, "-X", "-qAt", "-v", "schema="+schema, "-v", fmt.Sprintf("apply=%v", apply), "-v", "expected_manifest="+manifest, "-f", "scripts/database/cleanup_orphan_material_inventory.sql").CombinedOutput()
		return strings.TrimSpace(string(output)), err
	}
	preview := func() string {
		output, err := run(false, "")
		if err != nil {
			t.Fatalf("preview: %s %v", output, err)
		}
		var result struct {
			Manifest string `json:"manifest"`
			Count    int    `json:"count"`
		}
		if err = json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("parse preview: %s %v", output, err)
		}
		return result.Manifest
	}
	manifest := preview()
	// Preview is read-only; stale approval cannot operate on changed quantities.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.materials SET onhand_g=601 WHERE id=2`, schema)); err != nil {
		t.Fatal(err)
	}
	if out, err := run(true, manifest); err == nil {
		t.Fatalf("stale manifest accepted: %s", out)
	}
	manifest = preview()
	if _, err = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.audit_logs ADD CONSTRAINT reject_cleanup CHECK(action<>'remove_orphan_inventory')`, schema)); err != nil {
		t.Fatal(err)
	}
	if out, err := run(true, manifest); err == nil {
		t.Fatalf("audit failure must reject cleanup: %s", out)
	}
	var remaining int64
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT remaining_g FROM %s.material_batches WHERE id=2`, schema)).Scan(&remaining)
	if remaining != 600 {
		t.Fatalf("failed repair changed stock: %d", remaining)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`ALTER TABLE %s.audit_logs DROP CONSTRAINT reject_cleanup`, schema)); err != nil {
		t.Fatal(err)
	}
	// Fail the last inventory deletion after the header, batches, locations and
	// audit have already been written, proving that the whole transaction rolls back.
	if _, err = pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.reject_inventory_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'late cleanup failure'; END $$;
	CREATE TRIGGER reject_inventory_cleanup BEFORE DELETE ON %[1]s.customer_inventory_items FOR EACH ROW EXECUTE FUNCTION %[1]s.reject_inventory_cleanup();`, schema)); err != nil {
		t.Fatal(err)
	}
	if out, err := run(true, manifest); err == nil {
		t.Fatalf("late failure must roll back cleanup: %s", out)
	}
	var untouched bool
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT
	(SELECT onhand_g=601 FROM %[1]s.materials WHERE id=2) AND
	(SELECT remaining_g=600 FROM %[1]s.material_batches WHERE id=2) AND
	(SELECT qty_g=600 FROM %[1]s.material_batch_locations WHERE material_id=2) AND
	(SELECT remaining_g=600 FROM %[1]s.stock_batches WHERE id=2) AND
	NOT EXISTS(SELECT 1 FROM %[1]s.audit_logs WHERE action='remove_orphan_inventory')`, schema)).Scan(&untouched); err != nil || !untouched {
		t.Fatalf("late failure left partial changes: %v %v", untouched, err)
	}
	if _, err = pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER reject_inventory_cleanup ON %s.customer_inventory_items`, schema)); err != nil {
		t.Fatal(err)
	}
	if out, err := run(true, manifest); err != nil {
		t.Fatalf("apply: %s %v", out, err)
	}
	var active, orphans, audits, history int
	if err = pool.QueryRow(ctx, fmt.Sprintf(`SELECT
 (SELECT count(*) FROM %[1]s.material_batch_locations WHERE material_id=1 AND qty_g=500),
 (SELECT count(*) FROM %[1]s.material_batches WHERE material_id IN (2,3) AND remaining_g<>0)+(SELECT count(*) FROM %[1]s.material_batch_locations WHERE material_id IN (2,3))+(SELECT count(*) FROM %[1]s.stock_batches WHERE item_type='material' AND item_id IN (2,3) AND remaining_g<>0)+(SELECT count(*) FROM %[1]s.customer_inventory_items WHERE item_id IN (2,3))+(SELECT count(*) FROM %[1]s.materials WHERE id=2 AND onhand_g<>0),
 (SELECT count(*) FROM %[1]s.audit_logs WHERE action='remove_orphan_inventory' AND old_value::jsonb->'batches' IS NOT NULL),
 (SELECT count(*) FROM %[1]s.material_batches WHERE qty_g=1000)+(SELECT count(*) FROM %[1]s.stock_batches WHERE item_type='product' AND item_id=2 AND remaining_g=100)
 `, schema)).Scan(&active, &orphans, &audits, &history); err != nil || active != 1 || orphans != 0 || audits != 2 || history != 4 {
		t.Fatalf("active/orphans/audits/history=%d/%d/%d/%d err=%v", active, orphans, audits, history, err)
	}
	if out, err := run(true, preview()); err != nil {
		t.Fatalf("repeat: %s %v", out, err)
	}
	pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE action='remove_orphan_inventory'`, schema)).Scan(&audits)
	if audits != 2 {
		t.Fatalf("repeat created extra writes: %d", audits)
	}
}
