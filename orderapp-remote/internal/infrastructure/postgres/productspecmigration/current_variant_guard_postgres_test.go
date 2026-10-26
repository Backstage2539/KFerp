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
		CREATE TABLE %[1]s.products(
			id BIGINT PRIMARY KEY,active BOOLEAN NOT NULL DEFAULT true,
			parent_product_id BIGINT NOT NULL DEFAULT 0,auto_derived_sku BOOLEAN NOT NULL DEFAULT false
		);
		CREATE TABLE %[1]s.production_boms(
			id BIGINT PRIMARY KEY,output_type TEXT NOT NULL,output_product_id BIGINT NOT NULL,
			status TEXT NOT NULL
		);
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
			batch_id TEXT NOT NULL DEFAULT '',product_id BIGINT NOT NULL,bom_version_id BIGINT NOT NULL DEFAULT 0,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.finished_inventory(
			product_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL DEFAULT 0,
			bom_variant_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL DEFAULT 0,
			warehouse TEXT NOT NULL DEFAULT '',owner_customer_id BIGINT NOT NULL DEFAULT 0,
			onhand_units BIGINT NOT NULL DEFAULT 0,onhand_loose_g BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.stock_batches(
			id BIGINT PRIMARY KEY,item_type TEXT NOT NULL,item_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,
			source_doc_type TEXT NOT NULL DEFAULT '',source_doc_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.stock_ledger_entries(
			id BIGINT PRIMARY KEY,item_type TEXT NOT NULL,item_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,
			source_doc_type TEXT NOT NULL DEFAULT '',source_doc_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.production_operation_events(
			id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL
		);
		CREATE TABLE %[1]s.produce_running_items(
			id BIGINT PRIMARY KEY,batch_id TEXT NOT NULL DEFAULT '',product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.produce_running_outputs(
			id BIGINT PRIMARY KEY,running_item_id BIGINT NOT NULL DEFAULT 0,product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		CREATE TABLE %[1]s.stock_entries(
			id BIGINT PRIMARY KEY,work_order_id BIGINT NOT NULL DEFAULT 0,running_item_id BIGINT NOT NULL DEFAULT 0,
			entry_type TEXT NOT NULL DEFAULT '',purpose TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.stock_entry_items(
			id BIGINT PRIMARY KEY,stock_entry_id BIGINT NOT NULL,product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0,
			item_type TEXT NOT NULL DEFAULT ''
		);
		CREATE TABLE %[1]s.production_logs(
			id BIGINT PRIMARY KEY,running_item_id BIGINT NOT NULL DEFAULT 0,product_id BIGINT NOT NULL,
			bom_spec_id BIGINT NOT NULL DEFAULT 0,bom_variant_id BIGINT NOT NULL DEFAULT 0
		);
		INSERT INTO %[1]s.products(id) VALUES(42);
		INSERT INTO %[1]s.production_boms(id,output_type,output_product_id,status)
		VALUES(10,'product',42,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,status)
		VALUES(11,10,'archived'),(12,10,'published');
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',42,10,12,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id) VALUES(100,10);
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id)
		VALUES(110,11,100),(115,11,100),(120,12,100);
		INSERT INTO %[1]s.orders(id,order_no) VALUES(1,'SO-V1'),(2,'SO-V2');
		INSERT INTO %[1]s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(1,1,42,100,110),(4,2,42,100,110);
		INSERT INTO %[1]s.processing_job_request_items(id,product_id,bom_spec_id,bom_variant_id)
		VALUES(500,42,100,110),(501,42,100,120);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	// Match app startup: the final authority projection runs only after all
	// business tables exist, so it must preserve the historical-production
	// allowances installed by EnsureSchema.
	if err := EnsureAuthorityProjection(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state) VALUES(42,'cutover')
	`, schema)); err != nil {
		t.Fatal(err)
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
		) VALUES(14,42,'SO-V1, SO-V2',0,100,110)
	`, schema)); err != nil {
		t.Fatalf("historical plan derived from multiple frozen order identities: %v", err)
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
			id,production_plan_item_id,batch_id,product_id,bom_version_id,bom_spec_id,bom_variant_id
		) VALUES(20,10,'RUN-V1',42,11,100,110)
	`, schema)); err != nil {
		t.Fatalf("historical work order derived from frozen plan identity: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_entries(id,work_order_id,entry_type,purpose)
		VALUES(100,20,'finished_receipt','manufacture')
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_entry_items(id,stock_entry_id,product_id,bom_spec_id,bom_variant_id,item_type)
		VALUES(1000,100,42,100,110,'finished_product')
	`, schema)); err != nil {
		t.Fatalf("finished receipt derived from exact frozen work-order identity: %v", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_entry_items(id,stock_entry_id,product_id,bom_spec_id,bom_variant_id,item_type)
		VALUES(1001,100,42,100,115,'finished_product')
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("finished receipt with a different variant error=%v, want bom_variant_not_current", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_entries(id,work_order_id,entry_type,purpose)
		VALUES(101,999,'finished_receipt','manufacture')
	`, schema)); err != nil {
		t.Fatal(err)
	}

	// Aggregate finished inventory has no source-document columns of its own.
	// The production completion path must therefore supply its exact frozen
	// work-order context while applying this receipt. Without that context, an
	// archived identity must still be rejected.
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,warehouse,onhand_units)
		VALUES(42,100,110,'finished_goods',1)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("finished inventory without work-order receipt context error=%v, want bom_variant_not_current", err)
	}

	finishTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = finishTx.Exec(ctx, `SELECT set_config('kferp.production_frozen_work_order_id','20',true)`); err != nil {
		_ = finishTx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = finishTx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,warehouse,onhand_units)
		VALUES(42,100,110,'finished_goods',1)
	`, schema)); err != nil {
		_ = finishTx.Rollback(ctx)
		t.Fatalf("finished inventory derived from exact frozen work order: %v", err)
	}
	if err = finishTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	// Operation-completion receipts are sourced by the operation event rather
	// than by a running item. They must retain the exact historical identity of
	// their frozen work order across finished batch and ledger writes.
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_operation_events(id,work_order_id) VALUES(80,20)`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(id,item_type,item_id,bom_spec_id,bom_variant_id,source_doc_type,source_doc_id)
		VALUES(80,'finished_product',42,100,110,'production_operation_event',80)
	`, schema)); err != nil {
		t.Fatalf("finished batch derived from exact operation work-order identity: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(id,item_type,item_id,bom_spec_id,bom_variant_id,source_doc_type,source_doc_id)
		VALUES(80,'finished_product',42,100,110,'production_operation_event',80)
	`, schema)); err != nil {
		t.Fatalf("finished ledger derived from exact operation work-order identity: %v", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_batches(id,item_type,item_id,bom_spec_id,bom_variant_id,source_doc_type,source_doc_id)
		VALUES(81,'finished_product',42,100,110,'production_operation_event',999)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("finished batch without matching operation work order error=%v, want bom_variant_not_current", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_ledger_entries(id,item_type,item_id,bom_spec_id,bom_variant_id,source_doc_type,source_doc_id)
		VALUES(81,'finished_product',42,100,110,'production_operation_event',999)
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("finished ledger without matching operation work order error=%v, want bom_variant_not_current", err)
	}

	wrongFinishTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = wrongFinishTx.Exec(ctx, `SELECT set_config('kferp.production_frozen_work_order_id','20',true)`); err != nil {
		_ = wrongFinishTx.Rollback(ctx)
		t.Fatal(err)
	}
	_, err = wrongFinishTx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,bom_spec_id,bom_variant_id,warehouse,onhand_units)
		VALUES(42,100,115,'finished_goods',1)
	`, schema))
	_ = wrongFinishTx.Rollback(ctx)
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("finished inventory with mismatched frozen variant error=%v, want bom_variant_not_current", err)
	}
	_, err = pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.stock_entry_items(id,stock_entry_id,product_id,bom_spec_id,bom_variant_id,item_type)
		VALUES(1002,101,42,100,110,'finished_product')
	`, schema))
	if err == nil || !strings.Contains(err.Error(), "bom_variant_not_current") {
		t.Fatalf("finished receipt without a matching source work order error=%v, want bom_variant_not_current", err)
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
