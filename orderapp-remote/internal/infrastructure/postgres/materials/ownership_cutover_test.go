package materials

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMaterialOwnershipCutoverConflictTransactionIdempotencyAndRollback(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ORDERAPP_TEST_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		t.Skip("ORDERAPP_TEST_DATABASE_URL or DATABASE_URL is required")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("pr639_material_owner_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.customers(id BIGINT PRIMARY KEY,name TEXT NOT NULL,active BOOLEAN NOT NULL DEFAULT true);
		CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL DEFAULT '',entity_type TEXT NOT NULL DEFAULT '',entity_id BIGINT,action TEXT NOT NULL DEFAULT '',field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
		INSERT INTO %[1]s.customers(id,name) VALUES(74,'客户A');
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.material_batches(id BIGSERIAL PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,material_id BIGINT NOT NULL,owner_customer_id BIGINT NOT NULL DEFAULT 0,qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0,unit_cost NUMERIC(18,6) NOT NULL DEFAULT 0,received_at TIMESTAMPTZ NOT NULL DEFAULT now());
		CREATE TABLE %[1]s.material_batch_locations(material_batch_id BIGINT NOT NULL,batch_code TEXT NOT NULL,material_id BIGINT NOT NULL,warehouse TEXT NOT NULL,qty_g BIGINT NOT NULL DEFAULT 0,qty_units BIGINT NOT NULL DEFAULT 0,updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),PRIMARY KEY(material_batch_id,warehouse));
		CREATE TABLE %[1]s.work_orders(id BIGINT PRIMARY KEY,status TEXT NOT NULL);
		CREATE TABLE %[1]s.work_order_material_reservations(id BIGSERIAL PRIMARY KEY,work_order_id BIGINT NOT NULL,material_id BIGINT NOT NULL,status TEXT NOT NULL DEFAULT 'reserved',source_owner_customer_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.stock_entries(id BIGINT PRIMARY KEY,status TEXT NOT NULL);
		CREATE TABLE %[1]s.stock_entry_items(id BIGINT PRIMARY KEY,stock_entry_id BIGINT NOT NULL,material_id BIGINT NOT NULL,owner_customer_id BIGINT NOT NULL DEFAULT 0);
		CREATE TABLE %[1]s.business_group_assignments(id BIGSERIAL PRIMARY KEY,group_id BIGINT NOT NULL,group_item_id BIGINT NOT NULL,usage_key TEXT NOT NULL,object_key TEXT NOT NULL,object_id BIGINT NOT NULL,object_ref TEXT NOT NULL DEFAULT '',sort_order INT NOT NULL DEFAULT 100,created_by TEXT NOT NULL DEFAULT '',updated_by TEXT NOT NULL DEFAULT '',UNIQUE(usage_key,object_key,object_id,object_ref));
		INSERT INTO %[1]s.materials(id,code,name,kind,unit,cost_unit,owner_customer_id) VALUES(1,'PR639-MAT','同规格物料','bean','kg','kg',0);
		SELECT setval(pg_get_serial_sequence('%[1]s.materials','id'),1,true);
		INSERT INTO %[1]s.business_group_assignments(group_id,group_item_id,usage_key,object_key,object_id) VALUES(5,6,'material_catalog','material',1);
		INSERT INTO %[1]s.material_batches(id,batch_code,material_id,owner_customer_id,qty_g,remaining_g,unit_cost,received_at) VALUES(11,'FACTORY-BATCH',1,0,500,500,20,'2026-01-01'),(12,'CUSTOMER-BATCH',1,74,1000,1000,30,'2026-02-01');
		INSERT INTO %[1]s.material_batch_locations(material_batch_id,batch_code,material_id,warehouse,qty_g) VALUES(11,'FACTORY-BATCH',1,'raw_materials',500),(12,'CUSTOMER-BATCH',1,'customer_raw',1000);
		UPDATE %[1]s.materials SET onhand_g=1500 WHERE id=1;
		INSERT INTO %[1]s.work_orders(id,status) VALUES(90,'running');
		INSERT INTO %[1]s.work_order_material_reservations(work_order_id,material_id,source_owner_customer_id) VALUES(90,1,74);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	preview, err := repo.PreviewMaterialOwnershipCutover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Candidates) != 1 || len(preview.Conflicts) != 1 {
		t.Fatalf("preview=%+v", preview)
	}
	if _, err := repo.ApplyMaterialOwnershipCutover(ctx, "pr639-test", preview.ManifestID); err == nil || !strings.Contains(err.Error(), "冲突") {
		t.Fatalf("conflict apply err=%v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.work_order_material_reservations`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.work_order_material_reservations(work_order_id,material_id,source_owner_customer_id) VALUES(90,1,0)`, schema)); err != nil {
		t.Fatal(err)
	}
	preview, err = repo.PreviewMaterialOwnershipCutover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Conflicts) != 0 {
		t.Fatalf("factory-owned reservation must not block customer-owned stock split: %+v", preview)
	}
	if _, err := repo.ApplyMaterialOwnershipCutover(ctx, "pr639-test", "stale"); err == nil || !strings.Contains(err.Error(), "清单已变化") {
		t.Fatalf("stale manifest err=%v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE FUNCTION %[1]s.fail_pr639_location_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced failure'; END $$; CREATE TRIGGER fail_pr639_location_update BEFORE UPDATE ON %[1]s.material_batch_locations FOR EACH ROW EXECUTE FUNCTION %[1]s.fail_pr639_location_update();`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ApplyMaterialOwnershipCutover(ctx, "pr639-test", preview.ManifestID); err == nil || !strings.Contains(err.Error(), "forced failure") {
		t.Fatalf("transaction failure err=%v", err)
	}
	var materialCount, mappingCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.materials`, schema)).Scan(&materialCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.material_owner_migrations`, schema)).Scan(&mappingCount); err != nil {
		t.Fatal(err)
	}
	if materialCount != 1 || mappingCount != 0 {
		t.Fatalf("failed transaction leaked materials=%d mappings=%d", materialCount, mappingCount)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`DROP TRIGGER fail_pr639_location_update ON %[1]s.material_batch_locations; DROP FUNCTION %[1]s.fail_pr639_location_update();`, schema)); err != nil {
		t.Fatal(err)
	}
	applied, err := repo.ApplyMaterialOwnershipCutover(ctx, "pr639-test", preview.ManifestID)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Applied != 1 {
		t.Fatalf("applied=%+v", applied)
	}
	var targetID, sourceOnhand, targetOnhand, owner int64
	var received string
	var cost float64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT target_material_id FROM %s.material_owner_migrations WHERE source_material_id=1 AND owner_customer_id=74`, schema)).Scan(&targetID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT owner_customer_id,onhand_g FROM %s.materials WHERE id=$1`, schema), targetID).Scan(&owner, &targetOnhand); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT onhand_g FROM %s.materials WHERE id=1`, schema)).Scan(&sourceOnhand); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT unit_cost::float8,to_char(received_at,'YYYY-MM-DD') FROM %s.material_batches WHERE id=12 AND material_id=$1`, schema), targetID).Scan(&cost, &received); err != nil {
		t.Fatal(err)
	}
	if owner != 74 || sourceOnhand != 500 || targetOnhand != 1000 || cost != 30 || received != "2026-02-01" {
		t.Fatalf("owner/source/target/cost/date=%d/%d/%d/%v/%s", owner, sourceOnhand, targetOnhand, cost, received)
	}
	var copiedGroupItemID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT group_item_id FROM %s.business_group_assignments WHERE object_id=$1`, schema), targetID).Scan(&copiedGroupItemID); err != nil || copiedGroupItemID != 6 {
		t.Fatalf("copied material classification=%d err=%v", copiedGroupItemID, err)
	}
	repeatPreview, err := repo.PreviewMaterialOwnershipCutover(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repeat, err := repo.ApplyMaterialOwnershipCutover(ctx, "pr639-test", repeatPreview.ManifestID)
	if err != nil {
		t.Fatal(err)
	}
	if repeat.Applied != 0 {
		t.Fatalf("repeat=%+v", repeat)
	}
	rolledBack, err := repo.RollbackMaterialOwnershipCutover(ctx, "pr639-test")
	if err != nil {
		t.Fatal(err)
	}
	if rolledBack.RolledBack != 1 {
		t.Fatalf("rollback=%+v", rolledBack)
	}
	var originalMaterialID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT material_id FROM %s.material_batches WHERE id=12`, schema)).Scan(&originalMaterialID); err != nil || originalMaterialID != 1 {
		t.Fatalf("rollback material=%d err=%v", originalMaterialID, err)
	}
}
