package productcreator

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	postgresinfra "orderapp/internal/infrastructure/postgres"
)

func TestCreatorAssignsBusinessGroupRoot(t *testing.T) {
	dsn := os.Getenv("ORDERAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires isolated test database")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("pc_group_root_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	_, err = pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %[1]s.business_groups(id bigint PRIMARY KEY, active boolean NOT NULL);
CREATE TABLE %[1]s.business_group_items(id bigint PRIMARY KEY, group_id bigint NOT NULL, active boolean NOT NULL);
CREATE TABLE %[1]s.business_group_usages(group_id bigint NOT NULL, usage_key text NOT NULL, active boolean NOT NULL);
CREATE TABLE %[1]s.business_group_assignments(
	id bigserial PRIMARY KEY, group_id bigint NOT NULL, group_item_id bigint NOT NULL DEFAULT 0,
	usage_key text NOT NULL, object_key text NOT NULL, object_id bigint NOT NULL,
	object_ref text NOT NULL DEFAULT '', sort_order int NOT NULL DEFAULT 100,
	created_by text NOT NULL DEFAULT '', updated_by text NOT NULL DEFAULT ''
);
CREATE TABLE %[1]s.audit_logs(
	id bigserial PRIMARY KEY, actor text, entity_type text, entity_id bigint,
	action text, field text, old_value text, new_value text, meta jsonb
);
INSERT INTO %[1]s.business_groups VALUES(30, true);
INSERT INTO %[1]s.business_group_usages VALUES(30, 'material_catalog', true);`, schema))
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	txCtx := postgresinfra.WithTransaction(ctx, tx)
	if err := (BusinessExecutor{pool: pool, schema: schema}).assignBusinessGroup(txCtx, "tester", "material_catalog", "material", 15, 30, 0); err != nil {
		t.Fatal(err)
	}
	var groupID, itemID int64
	if err := tx.QueryRow(ctx, "SELECT group_id,group_item_id FROM "+schema+".business_group_assignments").Scan(&groupID, &itemID); err != nil {
		t.Fatal(err)
	}
	if groupID != 30 || itemID != 0 {
		t.Fatalf("root assignment=(%d,%d), want (30,0)", groupID, itemID)
	}
	var auditCount int
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+schema+".audit_logs WHERE action='assign_product_creator_classification'").Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("root assignment audit count=%d, want 1", auditCount)
	}
}
