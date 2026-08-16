package productspecmigration

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBusinessIdentityGuardRequiresCurrentVariantForNewIdentityPostgres(t *testing.T) {
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
	schema := fmt.Sprintf("pr600_current_variant_guard_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE") })

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		CREATE TABLE %[1]s.production_bom_versions(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL
		);
		CREATE TABLE %[1]s.production_bom_output_bindings(
			output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,
			bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT false
		);
		CREATE TABLE %[1]s.production_bom_specs(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL
		);
		CREATE TABLE %[1]s.production_bom_version_variants(
			id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL
		);
		CREATE TABLE %[1]s.orders(
			id BIGINT PRIMARY KEY,order_no TEXT NOT NULL UNIQUE
		);
		CREATE TABLE %[1]s.order_items(
			id BIGINT PRIMARY KEY,order_id BIGINT NOT NULL,product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.processing_job_request_items(
			id BIGINT PRIMARY KEY,product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.production_plan_items(
			id BIGINT PRIMARY KEY,product_id BIGINT NOT NULL,order_nos TEXT NOT NULL DEFAULT '',
			processing_request_item_id BIGINT NOT NULL DEFAULT 0,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.work_orders(
			id BIGINT PRIMARY KEY,production_plan_item_id BIGINT NOT NULL DEFAULT 0,
			batch_id TEXT NOT NULL DEFAULT '',product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.produce_running_items(
			id BIGINT PRIMARY KEY,batch_id TEXT NOT NULL DEFAULT '',product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.produce_running_outputs(
			id BIGINT PRIMARY KEY,running_item_id BIGINT NOT NULL DEFAULT 0,product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.production_logs(
			id BIGINT PRIMARY KEY,running_item_id BIGINT NOT NULL DEFAULT 0,product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,status)
		VALUES(11,10,'archived'),(12,10,'published');
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',42,10,12,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id) VALUES(100,10);
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id)
		VALUES(110,11,100),(120,12,100);
		INSERT INTO %[1]s.orders(id,order_no) VALUES(1,'SO-V1');
		INSERT INTO %[1]s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(1,1,42,100,110);
		INSERT INTO %[1]s.processing_job_request_items(id,product_id,bom_spec_id,bom_variant_id)
		VALUES(500,42,100,110),(501,42,100,120);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state) VALUES(42,'cutover')
	`, schema)); err != nil {
		t.Fatal(err)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.order_items SET bom_variant_id=bom_variant_id WHERE id=1
	`, schema)); err != nil {
		t.Fatalf("unchanged historical identity update must remain compatible: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(2,1,42,100,120)
	`, schema)); err != nil {
		t.Fatalf("current default variant insert: %v", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(3,1,42,100,110)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("historical variant insert error=%v, want bom_variant_not_current", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.order_items SET bom_variant_id=110 WHERE id=2
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("identity-changing historical variant update error=%v, want bom_variant_not_current", err)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_items(
			id,product_id,order_nos,processing_request_item_id,bom_spec_id,bom_variant_id
		) VALUES(10,42,'SO-V1',0,100,110)
	`, schema)); err != nil {
		t.Fatalf("historical plan derived from frozen order identity: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_items(
			id,product_id,order_nos,processing_request_item_id,bom_spec_id,bom_variant_id
		) VALUES(11,42,'',500,100,110)
	`, schema)); err != nil {
		t.Fatalf("historical plan derived from exact processing request identity: %v", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_items(
			id,product_id,order_nos,processing_request_item_id,bom_spec_id,bom_variant_id
		) VALUES(12,42,'MISSING',0,100,110)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("isolated historical plan insert error=%v, want bom_variant_not_current", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_plan_items(
			id,product_id,order_nos,processing_request_item_id,bom_spec_id,bom_variant_id
		) VALUES(13,42,'',501,100,110)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("mismatched processing identity plan error=%v, want bom_variant_not_current", err)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_orders(
			id,production_plan_item_id,batch_id,product_id,bom_spec_id,bom_variant_id
		) VALUES(20,10,'RUN-V1',42,100,110)
	`, schema)); err != nil {
		t.Fatalf("historical work order derived from frozen plan identity: %v", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_orders(
			id,production_plan_item_id,batch_id,product_id,bom_spec_id,bom_variant_id
		) VALUES(21,999,'RUN-ISOLATED',42,100,110)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("isolated historical work order insert error=%v, want bom_variant_not_current", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.produce_running_items(id,batch_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(30,'RUN-V1',42,100,110)
	`, schema)); err != nil {
		t.Fatalf("historical running item derived from frozen work order identity: %v", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.produce_running_items(id,batch_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(31,'RUN-ISOLATED',42,100,110)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("isolated historical running insert error=%v, want bom_variant_not_current", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.produce_running_outputs(id,running_item_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(40,30,42,100,110)
	`, schema)); err != nil {
		t.Fatalf("historical output derived from frozen running identity: %v", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.produce_running_outputs(id,running_item_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(41,999,42,100,110)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("isolated historical output insert error=%v, want bom_variant_not_current", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_logs(id,running_item_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(50,30,42,100,110)
	`, schema)); err != nil {
		t.Fatalf("historical production log derived from frozen running identity: %v", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_logs(id,running_item_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(51,999,42,100,110)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("isolated historical production log error=%v, want bom_variant_not_current", err)
	}
}
