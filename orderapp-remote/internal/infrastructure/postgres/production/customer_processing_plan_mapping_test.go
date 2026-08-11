package production

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	productionapp "orderapp/internal/application/production"
)

func TestCustomerProcessingPlanAndWorkOrderKeepRequestItemIdentity(t *testing.T) {
	schemaText, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	planText, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"processing_request_item_id BIGINT", "target_warehouse TEXT", "customer_id BIGINT"} {
		if strings.Count(string(schemaText), want) < 2 {
			t.Fatalf("production plan items and work orders must both persist %q", want)
		}
	}
	for _, want := range []string{
		"UPDATE %s.processing_job_request_items",
		"UPDATE %s.customer_processing_production_demands",
		"UPDATE %s.customer_processing_material_reservations",
		"processing_request_item_id",
	} {
		if !strings.Contains(string(planText), want) {
			t.Fatalf("production plan/work order mapping missing %q", want)
		}
	}
}

func TestCustomerProcessingSchemaAddsRequestItemColumnsBeforeCreatingIndexes(t *testing.T) {
	schemaText, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(schemaText)
	for _, table := range []string{"production_plan_items", "work_orders"} {
		alter := "ALTER TABLE %s." + table + " ADD COLUMN IF NOT EXISTS processing_request_item_id"
		index := " ON %s." + table + "(processing_request_item_id)"
		alterAt := strings.Index(source, alter)
		indexAt := strings.Index(source, index)
		if alterAt < 0 || indexAt < 0 {
			t.Fatalf("%s migration must contain request-item ALTER and index", table)
		}
		if alterAt > indexAt {
			t.Fatalf("%s creates request-item index before upgrading an existing table", table)
		}
	}
}

func TestCustomerProcessingSchemaUpgradesLegacyPlanAndWorkOrderTables(t *testing.T) {
	pool, schema := newProductionTestDB(t)
	ctx := context.Background()
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecProductionSQL(t, ctx, pool, fmt.Sprintf(`
		CREATE TABLE %[1]s.production_plan_items (
			id BIGSERIAL PRIMARY KEY,
			production_plan_id BIGINT NOT NULL
		);
		CREATE TABLE %[1]s.work_orders (
			id BIGSERIAL PRIMARY KEY,
			running_item_id BIGINT NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'running',
			created_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`, schema))

	if err := ensureWorkOrderTables(ctx, pool, schema); err != nil {
		t.Fatalf("upgrade legacy production tables: %v", err)
	}
	for _, table := range []string{"production_plan_items", "work_orders"} {
		var columnCount int
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM information_schema.columns
			WHERE table_schema=$1 AND table_name=$2 AND column_name='processing_request_item_id'
		`, schema, table).Scan(&columnCount); err != nil {
			t.Fatal(err)
		}
		if columnCount != 1 {
			t.Fatalf("%s processing_request_item_id columns=%d, want 1", table, columnCount)
		}
	}
	for _, indexName := range []string{"production_plan_items_processing_request_idx", "work_orders_processing_request_idx"} {
		var indexCount int
		if err := pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM pg_indexes WHERE schemaname=$1 AND indexname=$2
		`, schema, indexName).Scan(&indexCount); err != nil {
			t.Fatal(err)
		}
		if indexCount != 1 {
			t.Fatalf("%s count=%d, want 1", indexName, indexCount)
		}
	}
}

func TestCustomerProcessingCompletionWarehouseIsFixed(t *testing.T) {
	wo := productionapp.WorkOrderRow{ProcessingRequestItemID: 101, TargetWarehouse: "CUSTOMER-8"}
	if _, err := completionWarehouseForWorkOrder(wo, "finished_goods"); err == nil {
		t.Fatal("customer processing completion must reject a different warehouse")
	}
	got, err := completionWarehouseForWorkOrder(wo, "")
	if err != nil || got != "CUSTOMER-8" {
		t.Fatalf("completion warehouse=%q err=%v, want CUSTOMER-8", got, err)
	}
	if got, err := completionWarehouseForWorkOrder(productionapp.WorkOrderRow{}, ""); err != nil || got != "finished_goods" {
		t.Fatalf("ordinary completion warehouse=%q err=%v, want finished_goods", got, err)
	}
	ordinaryFrozen := productionapp.WorkOrderRow{TargetWarehouse: "finished_shop"}
	if got, err := completionWarehouseForWorkOrder(ordinaryFrozen, ""); err != nil || got != "finished_shop" {
		t.Fatalf("ordinary frozen completion warehouse=%q err=%v, want finished_shop", got, err)
	}
	if _, err := completionWarehouseForWorkOrder(ordinaryFrozen, "finished_goods"); err == nil {
		t.Fatal("ordinary completion must reject a warehouse different from the frozen target")
	}
}

func TestDerivedWorkOrderStatusIncludesPaused(t *testing.T) {
	if got := deriveWorkOrderStatusFromJobCardCounts("running", 2, 0, 0, 2); got != "paused" {
		t.Fatalf("status=%q, want paused", got)
	}
	if got := deriveWorkOrderStatusFromJobCardCounts("running", 2, 0, 1, 1); got != "running" {
		t.Fatalf("mixed running/paused status=%q, want running", got)
	}
}
