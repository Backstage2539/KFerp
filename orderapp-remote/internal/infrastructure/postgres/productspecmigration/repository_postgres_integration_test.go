package productspecmigration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPerProductMigrationPostgresFlow(t *testing.T) {
	if os.Getenv("KF_RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set KF_RUN_POSTGRES_INTEGRATION=1 to run against a disposable PostgreSQL schema")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("codex_pr600_product_spec_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")

	ddl := []string{
		fmt.Sprintf(`CREATE TABLE %s.products(
			id BIGINT PRIMARY KEY,name TEXT NOT NULL DEFAULT '',active BOOLEAN NOT NULL DEFAULT true,
			parent_product_id BIGINT NOT NULL DEFAULT 0,base_product_id BIGINT NOT NULL DEFAULT 0,
			custom_type TEXT NOT NULL DEFAULT '',auto_derived_sku BOOLEAN NOT NULL DEFAULT false,
			derived_spec_key TEXT NOT NULL DEFAULT '',derived_spec_name TEXT NOT NULL DEFAULT '',
			derived_sales_unit TEXT NOT NULL DEFAULT '',derived_unit_template_id BIGINT NOT NULL DEFAULT 0,
			derived_spec_status TEXT NOT NULL DEFAULT '',spec_label TEXT NOT NULL DEFAULT '',sku_code TEXT NOT NULL DEFAULT '',
			sku_name TEXT NOT NULL DEFAULT '',barcode TEXT NOT NULL DEFAULT '',net_content_qty NUMERIC(14,6) NOT NULL DEFAULT 0,
			net_content_unit TEXT NOT NULL DEFAULT ''
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.audit_logs(
			id BIGSERIAL PRIMARY KEY,actor TEXT,entity_type TEXT,entity_id BIGINT,action TEXT,field TEXT,
			old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_boms(id BIGINT PRIMARY KEY,output_type TEXT,output_product_id BIGINT,status TEXT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_versions(
			id BIGINT PRIMARY KEY,bom_id BIGINT,version_no TEXT,status TEXT,published_at TIMESTAMPTZ,
			source_spec_template_version_id BIGINT NOT NULL DEFAULT 900,
			main_input_material_id BIGINT NOT NULL DEFAULT 901
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_spec_template_versions(
			id BIGINT PRIMARY KEY,status TEXT NOT NULL,published_at TIMESTAMPTZ
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.materials(
			id BIGINT PRIMARY KEY,deprecated_at TIMESTAMPTZ
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_output_bindings(
			output_type TEXT,output_id BIGINT,bom_id BIGINT,bom_version_id BIGINT,is_default BOOLEAN,updated_at TIMESTAMPTZ DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_specs(
			id BIGINT PRIMARY KEY,bom_id BIGINT,code TEXT,barcode TEXT NOT NULL DEFAULT '',spec_key TEXT,name TEXT,inventory_unit TEXT,
			created_at TIMESTAMPTZ DEFAULT now(),updated_at TIMESTAMPTZ DEFAULT now(),created_by TEXT,updated_by TEXT
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_version_variants(
			id BIGINT PRIMARY KEY,version_id BIGINT,bom_spec_id BIGINT,spec_name_snapshot TEXT,inventory_unit TEXT,
			is_default BOOLEAN,sort_order INT,material_loss_rate NUMERIC,process_route_id BIGINT,created_at TIMESTAMPTZ DEFAULT now()
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_version_items(id BIGINT PRIMARY KEY,version_id BIGINT,variant_id BIGINT,material_id BIGINT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.product_production_configs(
			product_id BIGINT PRIMARY KEY,production_bom_id BIGINT NOT NULL DEFAULT 0,production_bom_version_id BIGINT NOT NULL DEFAULT 0,
			process_route_id BIGINT NOT NULL DEFAULT 0,expected_loss_rate NUMERIC NOT NULL DEFAULT 0,note TEXT NOT NULL DEFAULT '',
			industry_field_template_id BIGINT NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.finished_inventory(
			product_id BIGINT,spec_g BIGINT,warehouse TEXT,onhand_units BIGINT,onhand_loose_g BIGINT,
			PRIMARY KEY(product_id,spec_g,warehouse)
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.ship_statuses(id BIGINT PRIMARY KEY,name TEXT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.orders(id BIGINT PRIMARY KEY,ship_status_id BIGINT,is_void BOOLEAN)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_items(id BIGINT PRIMARY KEY,order_id BIGINT,product_id BIGINT,unit_bean_g BIGINT NOT NULL DEFAULT 0,price_source_json JSONB)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_plans(id BIGINT PRIMARY KEY,status TEXT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_plan_items(id BIGINT PRIMARY KEY,production_plan_id BIGINT,product_id BIGINT,spec_g BIGINT NOT NULL DEFAULT 0)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.work_orders(id BIGINT PRIMARY KEY,product_id BIGINT,spec_g BIGINT NOT NULL DEFAULT 0,status TEXT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.work_order_material_reservations(
			id BIGINT PRIMARY KEY,component_type TEXT,component_id BIGINT,component_spec_g BIGINT NOT NULL DEFAULT 0,status TEXT
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customer_direct_ship_requests(id BIGINT PRIMARY KEY,status TEXT)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customer_direct_ship_request_items(
			id BIGINT PRIMARY KEY,request_id BIGINT,product_id BIGINT,spec_g BIGINT NOT NULL DEFAULT 0
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.customer_processing_production_demands(
			id BIGINT PRIMARY KEY,product_id BIGINT,spec_g BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT ''
		)`, schema),
	}
	for _, statement := range ddl {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.products(id,name) VALUES(42,'parent');
			INSERT INTO %[1]s.products(id,name,parent_product_id,auto_derived_sku,derived_spec_key,derived_spec_name,derived_sales_unit,spec_label,sku_code,barcode,net_content_qty,net_content_unit)
			VALUES(43,'227g',42,true,'bag-227g','227g袋','袋','227g','SKU-227','BAR-227',227,'g'),
			      (44,'454g',42,false,'bag-454g','454g袋','袋','454g','SKU-454','BAR-454',454,'g');
		INSERT INTO %[1]s.production_bom_spec_template_versions(id,status,published_at) VALUES(900,'archived',now());
		INSERT INTO %[1]s.materials(id,deprecated_at) VALUES(901,NULL);
		INSERT INTO %[1]s.production_boms VALUES(10,'product',42,'active');
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,published_at)
		VALUES(11,10,'V001','published',now());
		INSERT INTO %[1]s.production_bom_output_bindings VALUES('product',42,10,11,true,now());
		INSERT INTO %[1]s.production_bom_specs(id,bom_id,code,barcode,spec_key,name,inventory_unit) VALUES
			(100,10,'BOM-SPEC-000100','6900000000100','bag-227g','227g袋','袋'),(101,10,'BOM-SPEC-000101','6900000000101','bag-454g','454g袋','袋');
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id,is_default,sort_order) VALUES
			(200,11,100,true,1),(201,11,101,false,2);
		INSERT INTO %[1]s.production_bom_version_items VALUES(300,11,200,900);
		INSERT INTO %[1]s.product_production_configs(product_id,production_bom_id,production_bom_version_id,process_route_id,expected_loss_rate,note,industry_field_template_id)
		VALUES(42,10,11,70,0.05,'父商品配置',80),(43,12,13,71,0.08,'227g 单独配置',81);
		INSERT INTO %[1]s.finished_inventory VALUES(43,227,'finished_goods',1,0);
			INSERT INTO %[1]s.ship_statuses VALUES(1,'已发货'),(2,'待发货');
			INSERT INTO %[1]s.orders VALUES(500,1,false),(510,2,false);
			INSERT INTO %[1]s.order_items(id,order_id,product_id,unit_bean_g,price_source_json)
			VALUES(501,500,43,227,'{"sku_id":43,"spec_g":227}'::jsonb),
			      (511,510,42,454,'{"product_id":42,"spec_g":454}'::jsonb);
			INSERT INTO %[1]s.customer_processing_production_demands(id,product_id,spec_g,status)
			VALUES(520,42,454,'pending');
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	prepared, err := repo.Prepare(ctx, productspecmigrationapp.PrepareCommand{ProductID: 42, Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if prepared.State != productspecmigrationapp.StatePreparing || len(prepared.Mappings) != 2 {
		t.Fatalf("prepared = %+v", prepared)
	}
	if prepared.Mappings[0].LegacySpecName != "227g袋" || prepared.Mappings[0].LegacySalesUnit != "袋" || !strings.Contains(prepared.Mappings[0].MetadataSnapshot, `"expected_loss_rate": 0.08`) || !strings.Contains(prepared.Mappings[0].MetadataSnapshot, `"parent_production_config"`) {
		t.Fatalf("prepared legacy comparison metadata = %+v", prepared.Mappings[0])
	}
	var recipeRows int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.production_bom_version_items`, schema)).Scan(&recipeRows); err != nil || recipeRows != 1 {
		t.Fatalf("prepare generated recipe rows: count=%d err=%v", recipeRows, err)
	}
	assessed, err := repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 42, Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if assessed.Readiness.Ready || assessed.Readiness.LegacyStockCount == 0 {
		t.Fatalf("stock must block cutover: %+v", assessed.Readiness)
	}
	if assessed.Readiness.UnfinishedOrderCount == 0 {
		t.Fatalf("parent + legacy unit order must block cutover: %+v", assessed.Readiness)
	}
	if assessed.Readiness.UnfinishedFulfillmentCount == 0 {
		t.Fatalf("legacy customer-processing demand must block cutover: %+v", assessed.Readiness)
	}
	_, err = repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 42, Actor: "tester"})
	var blocked *productspecmigrationapp.CutoverBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("blocked cutover err=%v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.finished_inventory SET onhand_units=0,onhand_loose_g=0`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_status_id=1 WHERE id=510`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_processing_production_demands SET status='completed' WHERE id=520`, schema)); err != nil {
		t.Fatal(err)
	}
	assessed, err = repo.Assess(ctx, productspecmigrationapp.AssessCommand{ProductID: 42, Actor: "tester"})
	if err != nil || !assessed.Readiness.Ready {
		t.Fatalf("ready assessment=%+v err=%v", assessed.Readiness, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.orders(id,ship_status_id,is_void) VALUES(530,2,false)`, schema)); err != nil {
		t.Fatal(err)
	}
	businessTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := businessTx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.order_items(id,order_id,product_id,unit_bean_g,price_source_json)
		VALUES(531,530,43,227,'{"sku_id":43,"spec_g":227}'::jsonb)
	`, schema)); err != nil {
		_ = businessTx.Rollback(ctx)
		t.Fatal(err)
	}
	cutoverDone := make(chan error, 1)
	go func() {
		_, cutoverErr := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 42, Actor: "tester"})
		cutoverDone <- cutoverErr
	}()
	select {
	case cutoverErr := <-cutoverDone:
		_ = businessTx.Rollback(ctx)
		t.Fatalf("cutover completed while a legacy business write held the migration row: %v", cutoverErr)
	case <-time.After(150 * time.Millisecond):
	}
	if err := businessTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case cutoverErr := <-cutoverDone:
		if cutoverErr == nil {
			t.Fatal("cutover ignored a legacy order committed during readiness assessment")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cutover remained blocked after concurrent business write committed")
	}
	var concurrentState string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT state FROM %s.product_bom_spec_migrations WHERE product_id=42`, schema)).Scan(&concurrentState); err != nil {
		t.Fatal(err)
	}
	if concurrentState == string(productspecmigrationapp.StateCutover) {
		t.Fatalf("concurrent cutover state=%q, want non-cutover", concurrentState)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.orders SET ship_status_id=1 WHERE id=530`, schema)); err != nil {
		t.Fatal(err)
	}
	cutover, err := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 42, Actor: "tester"})
	if err != nil || cutover.State != productspecmigrationapp.StateCutover {
		t.Fatalf("cutover=%+v err=%v", cutover, err)
	}
	if _, err := repo.Cutover(ctx, productspecmigrationapp.CutoverCommand{ProductID: 42, Actor: "tester"}); err != nil {
		t.Fatalf("idempotent cutover: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit)
			VALUES(102,10,'BOM-SPEC-000102','bag-100g','100g袋','袋'),
			      (110,20,'BOM-SPEC-000110','replacement','替代规格','袋');
			INSERT INTO %s.production_boms(id,output_type,output_product_id,status)
			VALUES(20,'product',42,'active');
			INSERT INTO %s.production_bom_versions(id,bom_id,version_no,status,published_at)
			VALUES(12,10,'V002','published',now()),(13,10,'V003','published',now()),(21,20,'V001','published',now());
			INSERT INTO %s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order)
			VALUES(210,12,100,'227g袋','袋',true,1),
			      (211,12,101,'454g袋','袋',false,2),
			      (212,12,102,'100g袋','袋',false,3),
			      (213,13,101,'454g袋','袋',true,1),
			      (214,13,102,'100g袋','袋',false,2),
			      (220,21,110,'替代规格','袋',true,1);
		`, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	switchDefaultBinding := func(bomID, versionID int64) error {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)
		if err := GuardDefaultProductBOMSwitchTx(ctx, tx, schema, 42, bomID, versionID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.production_bom_versions
			SET status=CASE WHEN id=$2 THEN 'published' ELSE 'archived' END
			WHERE bom_id=$1 AND status IN ('published','archived')
		`, schema), bomID, versionID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.production_bom_output_bindings
			SET bom_id=$2,bom_version_id=$3,updated_at=now()
			WHERE output_type='product' AND output_id=$1 AND is_default=true
		`, schema), 42, bomID, versionID); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.finished_inventory(product_id,spec_g,warehouse,onhand_units,onhand_loose_g,bom_spec_id,bom_variant_id)
		VALUES(42,0,'default_bom_switch_guard',1,0,100,200);
			INSERT INTO %s.orders(id,ship_status_id,is_void) VALUES(600,2,false);
		INSERT INTO %s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(601,600,42,100,200);
		INSERT INTO %s.production_plans(id,status) VALUES(700,'submitted');
		INSERT INTO %s.production_plan_items(id,production_plan_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(701,700,42,100,200);
		INSERT INTO %s.work_orders(id,product_id,bom_spec_id,bom_variant_id,status)
		VALUES(800,42,100,200,'running');
		INSERT INTO %s.work_order_material_reservations(id,component_type,component_id,bom_spec_id,bom_variant_id,status)
		VALUES(801,'product',42,100,200,'reserved');
		INSERT INTO %s.customer_direct_ship_requests(id,status) VALUES(900,'reserved');
		INSERT INTO %s.customer_direct_ship_request_items(id,request_id,product_id,bom_spec_id,bom_variant_id)
		VALUES(901,900,42,100,200);
		`, schema, schema, schema, schema, schema, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	if err := switchDefaultBinding(10, 12); err != nil {
		t.Fatalf("same BOM version retaining existing specs must switch: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.order_items SET bom_variant_id=bom_variant_id WHERE id=601
	`, schema)); err != nil {
		t.Fatalf("old variant snapshot must remain writable after retained-spec version switch: %v", err)
	}
	if err := switchDefaultBinding(10, 13); err == nil {
		t.Fatal("same BOM version removing an active spec unexpectedly switched")
	}
	var guardedVersionID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT bom_version_id FROM %s.production_bom_output_bindings
		WHERE output_type='product' AND output_id=42 AND is_default=true
	`, schema)).Scan(&guardedVersionID); err != nil {
		t.Fatal(err)
	}
	if guardedVersionID != 12 {
		t.Fatalf("blocked same-BOM version changed binding to %d, want 12", guardedVersionID)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = GuardDefaultProductBOMSwitchTx(ctx, tx, schema, 42, 20, 21)
	_ = tx.Rollback(ctx)
	var switchBlocked *productspecmigrationapp.DefaultBOMSwitchBlockedError
	if !errors.As(err, &switchBlocked) {
		t.Fatalf("default BOM switch with old spec stock err=%v blocker=%+v", err, switchBlocked)
	}
	blockerCodes := make(map[string]bool, len(switchBlocked.Blockers))
	for _, blocker := range switchBlocked.Blockers {
		blockerCodes[blocker.Code] = true
	}
	for _, code := range []string{
		"old_bom_stock",
		"old_bom_reservations",
		"old_bom_unfinished_orders",
		"old_bom_unfinished_plans",
		"old_bom_unfinished_work_orders",
		"old_bom_unfinished_fulfillment",
	} {
		if !blockerCodes[code] {
			t.Fatalf("default BOM switch blockers=%+v, missing %s", switchBlocked.Blockers, code)
		}
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.finished_inventory
		SET onhand_units=0,onhand_loose_g=0
		WHERE product_id=42 AND bom_spec_id=100 AND warehouse='default_bom_switch_guard';
		UPDATE %s.orders SET ship_status_id=1 WHERE id=600;
		UPDATE %s.production_plans SET status='completed' WHERE id=700;
		UPDATE %s.work_orders SET status='completed' WHERE id=800;
		UPDATE %s.work_order_material_reservations SET status='released' WHERE id=801;
		UPDATE %s.customer_direct_ship_requests SET status='completed' WHERE id=900;
	`, schema, schema, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = GuardDefaultProductBOMSwitchTx(ctx, tx, schema, 42, 20, 21)
	_ = tx.Rollback(ctx)
	if err != nil {
		t.Fatalf("default BOM switch after old spec activity cleared: %v", err)
	}
	if err := switchDefaultBinding(10, 13); err != nil {
		t.Fatalf("same BOM version removing a cleared spec must switch: %v", err)
	}
	if err := switchDefaultBinding(10, 12); err != nil {
		t.Fatalf("same BOM version adding back a spec must switch: %v", err)
	}
	options, err := repo.ListOptions(ctx, 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(options) != 3 {
		t.Fatalf("default published BOM options=%+v, want all three variants", options)
	}
	var newSpec *productspecmigrationapp.ProductSpecOption
	for idx := range options {
		if options[idx].BomSpecID == 102 {
			newSpec = &options[idx]
			break
		}
	}
	if newSpec == nil || newSpec.LegacyChildProductID != 0 || newSpec.WriteProductID != 42 || newSpec.WriteBomSpecID != 102 || newSpec.WriteBomVariantID != 212 || newSpec.BomVersionID != 12 || newSpec.BomVersionNo != "V002" || newSpec.SpecCode != "BOM-SPEC-000102" || newSpec.SortOrder != 3 || !newSpec.Published {
		t.Fatalf("new BOM-only spec option=%+v, want canonical writable identity without legacy child", newSpec)
	}
	var active bool
	var status, barcode string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active,derived_spec_status,barcode FROM %s.products WHERE id=43`, schema)).Scan(&active, &status, &barcode); err != nil {
		t.Fatal(err)
	}
	if active || status != "bom_spec_cutover" || barcode != "BAR-227" {
		t.Fatalf("legacy tombstone active=%v status=%q barcode=%q", active, status, barcode)
	}
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active,derived_spec_status FROM %s.products WHERE id=44`, schema)).Scan(&active, &status); err != nil {
		t.Fatal(err)
	}
	if active || status != "bom_spec_cutover" {
		t.Fatalf("derived-spec legacy tombstone active=%v status=%q", active, status)
	}
	var historicalProductID int64
	var historicalSnapshot string
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT product_id,price_source_json::text FROM %s.order_items WHERE id=501`, schema)).Scan(&historicalProductID, &historicalSnapshot); err != nil {
		t.Fatal(err)
	}
	if historicalProductID != 43 || historicalSnapshot == "" {
		t.Fatalf("historical order was rewritten: product=%d snapshot=%q", historicalProductID, historicalSnapshot)
	}
	var cutoverAudits int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.audit_logs WHERE entity_type='product_bom_spec_migration' AND action='cutover'`, schema)).Scan(&cutoverAudits); err != nil || cutoverAudits != 1 {
		t.Fatalf("cutover audits=%d err=%v", cutoverAudits, err)
	}
	identity, err := repo.ResolveIdentity(ctx, productspecmigrationapp.ResolveIdentityCommand{ProductID: 43, Mode: productspecmigrationapp.ResolveRead})
	if err != nil || identity.ProductID != 42 || identity.BomSpecID == nil || *identity.BomSpecID != 100 {
		t.Fatalf("legacy read identity=%+v err=%v", identity, err)
	}
	_, err = repo.ResolveIdentity(ctx, productspecmigrationapp.ResolveIdentityCommand{ProductID: 43, Mode: productspecmigrationapp.ResolveWrite})
	if !errors.Is(err, productspecmigrationapp.ErrLegacyWriteRejected) {
		t.Fatalf("legacy write err=%v", err)
	}
	specID := int64(100)
	identity, err = repo.ResolveIdentity(ctx, productspecmigrationapp.ResolveIdentityCommand{ProductID: 42, BomSpecID: &specID, Mode: productspecmigrationapp.ResolveWrite})
	if err != nil || identity.BomVariantID == nil || *identity.BomVariantID != 210 {
		t.Fatalf("canonical identity=%+v err=%v", identity, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(id,order_id,product_id) VALUES(502,500,43)`, schema)); err == nil {
		t.Fatal("legacy child write after cutover unexpectedly succeeded")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(id,order_id,product_id) VALUES(503,500,42)`, schema)); err == nil {
		t.Fatal("parent write without bom_spec_id after cutover unexpectedly succeeded")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.order_items(id,order_id,product_id,bom_spec_id,bom_variant_id) VALUES(504,500,42,100,210)`, schema)); err != nil {
		t.Fatalf("canonical parent + BOM spec write: %v", err)
	}

	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.production_boms(id,output_type,output_product_id,status) VALUES(30,'product',42,'active');
		INSERT INTO %s.production_bom_versions(id,bom_id,version_no,status,published_at) VALUES(31,30,'V001','published',now());
		INSERT INTO %s.production_bom_specs(id,bom_id,code,spec_key,name,inventory_unit) VALUES(130,30,'BOM-SPEC-000130','concurrent','并发规格','袋');
		INSERT INTO %s.production_bom_version_variants(id,version_id,bom_spec_id,spec_name_snapshot,inventory_unit,is_default,sort_order) VALUES(230,31,130,'并发规格','袋',true,1);
	`, schema, schema, schema, schema)); err != nil {
		t.Fatal(err)
	}
	type switchTarget struct{ bomID, versionID int64 }
	targets := []switchTarget{{bomID: 20, versionID: 21}, {bomID: 30, versionID: 31}}
	start := make(chan struct{})
	errs := make(chan error, len(targets))
	var wg sync.WaitGroup
	for _, target := range targets {
		target := target
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			tx, err := pool.Begin(ctx)
			if err == nil {
				err = GuardDefaultProductBOMSwitchTx(ctx, tx, schema, 42, target.bomID, target.versionID)
			}
			if err == nil {
				_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.production_bom_output_bindings SET bom_id=$2,bom_version_id=$3,updated_at=now() WHERE output_type='product' AND output_id=$1`, schema), 42, target.bomID, target.versionID)
			}
			if err == nil {
				err = tx.Commit(ctx)
			} else if tx != nil {
				_ = tx.Rollback(ctx)
			}
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent default BOM switch: %v", err)
		}
	}
	var bindingCount int
	var concurrentBOMID int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*),max(bom_id) FROM %s.production_bom_output_bindings WHERE output_type='product' AND output_id=42 AND is_default=true`, schema)).Scan(&bindingCount, &concurrentBOMID); err != nil {
		t.Fatal(err)
	}
	if bindingCount != 1 || (concurrentBOMID != 20 && concurrentBOMID != 30) {
		t.Fatalf("concurrent default binding count=%d bom=%d", bindingCount, concurrentBOMID)
	}
}

func TestDefaultBOMSwitchSerializesConcurrentRemovedSpecWritePostgres(t *testing.T) {
	if os.Getenv("KF_RUN_POSTGRES_INTEGRATION") != "1" {
		t.Skip("set KF_RUN_POSTGRES_INTEGRATION=1 to run against a disposable PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	schema := fmt.Sprintf("codex_pr600_default_switch_write_%d_%d", os.Getpid(), time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")

	ddl := []string{
		fmt.Sprintf(`CREATE TABLE %s.production_bom_versions(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL,status TEXT NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_output_bindings(
			output_type TEXT NOT NULL,output_id BIGINT NOT NULL,bom_id BIGINT NOT NULL,
			bom_version_id BIGINT NOT NULL,is_default BOOLEAN NOT NULL DEFAULT false
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_specs(
			id BIGINT PRIMARY KEY,bom_id BIGINT NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.production_bom_version_variants(
			id BIGINT PRIMARY KEY,version_id BIGINT NOT NULL,bom_spec_id BIGINT NOT NULL
		)`, schema),
		fmt.Sprintf(`CREATE TABLE %s.order_items(
			id BIGINT PRIMARY KEY,product_id BIGINT NOT NULL
		)`, schema),
	}
	for _, statement := range ddl {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := EnsureSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,status)
		VALUES(11,10,'published'),(21,20,'published');
		INSERT INTO %[1]s.production_bom_output_bindings(output_type,output_id,bom_id,bom_version_id,is_default)
		VALUES('product',42,10,11,true);
		INSERT INTO %[1]s.production_bom_specs(id,bom_id) VALUES(100,10),(200,20);
		INSERT INTO %[1]s.production_bom_version_variants(id,version_id,bom_spec_id)
		VALUES(110,11,100),(210,21,200);
	`, schema)); err != nil {
		t.Fatal(err)
	}
	legacyTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := GuardDefaultProductBOMSwitchTx(ctx, legacyTx, schema, 42, 20, 21); err != nil {
		_ = legacyTx.Rollback(ctx)
		t.Fatalf("product without migration row must keep legacy switch compatibility: %v", err)
	}
	if err := legacyTx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state) VALUES(42,'cutover')
	`, schema)); err != nil {
		t.Fatal(err)
	}

	switchTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer switchTx.Rollback(context.Background())
	if err := GuardDefaultProductBOMSwitchTx(ctx, switchTx, schema, 42, 20, 21); err != nil {
		t.Fatalf("guard default BOM switch: %v", err)
	}

	writeDone := make(chan error, 1)
	go func() {
		writeTx, beginErr := pool.Begin(ctx)
		if beginErr != nil {
			writeDone <- beginErr
			return
		}
		defer writeTx.Rollback(context.Background())
		_, writeErr := writeTx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.order_items(id,product_id,bom_spec_id,bom_variant_id)
			VALUES(1,42,100,110)
		`, schema))
		if writeErr == nil {
			writeErr = writeTx.Commit(ctx)
		}
		writeDone <- writeErr
	}()

	var writeCompletedBeforeSwitch bool
	var writeErr error
	select {
	case writeErr = <-writeDone:
		writeCompletedBeforeSwitch = true
	case <-time.After(200 * time.Millisecond):
	}
	if _, err := switchTx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.production_bom_output_bindings
		SET bom_id=20,bom_version_id=21
		WHERE output_type='product' AND output_id=42 AND is_default=true
	`, schema)); err != nil {
		t.Fatal(err)
	}
	if err := switchTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if !writeCompletedBeforeSwitch {
		select {
		case writeErr = <-writeDone:
		case <-ctx.Done():
			t.Fatalf("concurrent business write did not resume after default switch: %v", ctx.Err())
		}
	}
	if writeCompletedBeforeSwitch {
		t.Fatalf("removed-spec business write completed before the default switch committed: %v", writeErr)
	}
	if writeErr == nil || !strings.Contains(writeErr.Error(), "bom_spec_not_published") {
		t.Fatalf("concurrent removed-spec write error=%v, want bom_spec_not_published after serialized switch", writeErr)
	}
	var orderCount int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM %s.order_items`, schema)).Scan(&orderCount); err != nil {
		t.Fatal(err)
	}
	if orderCount != 0 {
		t.Fatalf("removed-spec business rows=%d, want 0", orderCount)
	}
}
