package stock

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	stockapp "orderapp/internal/application/stock"
)

func virtualWarehouseTestDB(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	pool, schema := newStockTestDB(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
		pool.Close()
	})
	mustExecStockSQL(t, context.Background(), pool, fmt.Sprintf(`
CREATE TABLE %[1]s.customers(id BIGINT PRIMARY KEY,name TEXT NOT NULL,active BOOLEAN NOT NULL DEFAULT true);
INSERT INTO %[1]s.customers(id,name) VALUES(1,'客户甲'),(2,'客户乙');
CREATE TABLE %[1]s.audit_logs(id BIGSERIAL PRIMARY KEY,actor TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id BIGINT,action TEXT NOT NULL,field TEXT,old_value TEXT,new_value TEXT,meta JSONB,created_at TIMESTAMPTZ NOT NULL DEFAULT now());
CREATE TABLE %[1]s.business_groups(id BIGINT PRIMARY KEY,name TEXT);
CREATE TABLE %[1]s.business_group_items(id BIGINT PRIMARY KEY,name TEXT);
CREATE TABLE %[1]s.business_group_assignments(id BIGINT PRIMARY KEY,group_id BIGINT,group_item_id BIGINT,object_ref TEXT,object_id BIGINT,usage_key TEXT,object_key TEXT);
`, schema))
	if err := ensureWarehouseTables(context.Background(), pool, schema); err != nil {
		t.Fatal(err)
	}
	return pool, schema
}

func createVirtualForTest(t *testing.T, repo Repository, customerID int64) stockapp.WarehouseRow {
	t.Helper()
	row, err := repo.CreateVirtualWarehouse(context.Background(), stockapp.CreateVirtualWarehouseCommand{Name: "客户成品仓", Kind: "finished", CustomerID: customerID, Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func TestVirtualWarehouseMigrationPreservesIdentityBalancesAndVisibility(t *testing.T) {
	pool, schema := virtualWarehouseTestDB(t)
	ctx := context.Background()
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`INSERT INTO %[1]s.warehouses(code,name,kind,customer_id)VALUES('historical','历史自定义名称','finished',1);
CREATE TABLE %[1]s.finished_inventory(warehouse TEXT,onhand_units BIGINT,onhand_loose_g BIGINT);
INSERT INTO %[1]s.finished_inventory VALUES('historical',7,0);`, schema))
	if err := ensureWarehouseTables(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := ensureWarehouseTables(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	var virtual bool
	var name string
	var qty int64
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT w.is_virtual,w.name,i.onhand_units FROM %[1]s.warehouses w JOIN %[1]s.finished_inventory i ON i.warehouse=w.code WHERE w.code='historical'`, schema)).Scan(&virtual, &name, &qty); err != nil {
		t.Fatal(err)
	}
	if !virtual || name != "历史自定义名称" || qty != 7 {
		t.Fatalf("migration changed identity/balance: %v %q %d", virtual, name, qty)
	}
	repo := NewRepository(pool, schema)
	unbound := createVirtualForTest(t, repo, 0)
	rows, err := repo.ListWarehouses(ctx, stockapp.WarehouseListQuery{CustomerID: 1})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.Code] = true
	}
	if seen[unbound.Code] || !seen["historical"] || !seen["finished_goods"] {
		t.Fatalf("customer scope contains unbound virtual or lost physical: %#v", seen)
	}
	zero := int64(0)
	result, err := repo.ListVirtualWarehouses(ctx, stockapp.VirtualWarehouseQuery{CustomerID: &zero, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Rows) != 1 || result.Rows[0].Code != unbound.Code {
		t.Fatalf("unbound virtual paging=%+v", result)
	}
}

func TestVirtualWarehouseCRUDRevisionAuditAndEnsurePreserveNames(t *testing.T) {
	pool, schema := virtualWarehouseTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	row := createVirtualForTest(t, repo, 1)
	if !row.IsVirtual || row.Revision != 1 || row.Kind != "finished" {
		t.Fatalf("created=%+v", row)
	}
	updated, err := repo.UpdateVirtualWarehouse(ctx, stockapp.UpdateVirtualWarehouseCommand{Code: row.Code, Name: "新名称", Description: "独立说明", CustomerID: 2, ExpectedRevision: 1, Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Revision != 2 || updated.CustomerID != 2 || updated.Code != row.Code || updated.Kind != row.Kind {
		t.Fatalf("updated=%+v", updated)
	}
	_, err = repo.UpdateVirtualWarehouse(ctx, stockapp.UpdateVirtualWarehouseCommand{Code: row.Code, Name: "旧版本覆盖", CustomerID: 2, ExpectedRevision: 1})
	if !errors.Is(err, stockapp.ErrWarehouseConflict) {
		t.Fatalf("stale update err=%v", err)
	}
	_, err = repo.DeleteVirtualWarehouse(ctx, stockapp.DeleteVirtualWarehouseCommand{Code: row.Code, ExpectedRevision: 1})
	if !errors.Is(err, stockapp.ErrWarehouseConflict) {
		t.Fatalf("stale delete err=%v", err)
	}
	deleted, err := repo.DeleteVirtualWarehouse(ctx, stockapp.DeleteVirtualWarehouseCommand{Code: row.Code, ExpectedRevision: 2, Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	if deleted.Active || deleted.Revision != 3 {
		t.Fatalf("delete=%+v", deleted)
	}
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.audit_logs WHERE entity_type='warehouse' AND meta->>'warehouse'=$1`, schema), row.Code).Scan(&count); err != nil || count != 3 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
	ensured, err := repo.EnsureCustomerWarehouse(ctx, stockapp.EnsureCustomerWarehouseCommand{CustomerID: 1, Kind: "finished", Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	ensured, err = repo.UpdateVirtualWarehouse(ctx, stockapp.UpdateVirtualWarehouseCommand{Code: ensured.Code, Name: "保留自定义仓名", CustomerID: 1, ExpectedRevision: ensured.Revision, Actor: "tester"})
	if err != nil {
		t.Fatal(err)
	}
	again, err := repo.EnsureCustomerWarehouse(ctx, stockapp.EnsureCustomerWarehouseCommand{CustomerID: 1, Kind: "finished", Actor: "tester"})
	if err != nil || again.Name != ensured.Name || !again.IsVirtual || again.Revision != ensured.Revision {
		t.Fatalf("ensure overwrote custom name: %+v err=%v", again, err)
	}
}

func TestVirtualWarehouseReferenceGuardsAndLegacyBinding(t *testing.T) {
	for _, tc := range []struct{ name, ddl, insert string }{
		{"stock", `CREATE TABLE %s.finished_inventory(warehouse TEXT,onhand_units BIGINT,onhand_loose_g BIGINT)`, `INSERT INTO %s.finished_inventory VALUES($1,2,0)`},
		{"material", `CREATE TABLE %s.material_batch_locations(warehouse TEXT,qty_g BIGINT,qty_units BIGINT)`, `INSERT INTO %s.material_batch_locations VALUES($1,200,0)`},
		{"reservation", `CREATE TABLE %[1]s.customer_direct_ship_request_allocations(warehouse_code TEXT,status TEXT,order_id BIGINT);CREATE TABLE %[1]s.orders(id BIGINT PRIMARY KEY,ship_status_id BIGINT,is_void BOOLEAN DEFAULT false)`, `WITH doc AS(INSERT INTO %[1]s.orders(id,ship_status_id)VALUES(10,1) RETURNING id)INSERT INTO %[1]s.customer_direct_ship_request_allocations SELECT $1,'reserved',id FROM doc`},
		{"entitlement", `CREATE TABLE %s.customer_stock_entitlements(warehouse_code TEXT,promised_qty BIGINT,received_qty BIGINT,status TEXT)`, `INSERT INTO %s.customer_stock_entitlements VALUES($1,2,0,'active')`},
		{"order", `CREATE TABLE %s.orders(delivery_warehouse_code TEXT,is_void BOOLEAN,delivery_status TEXT)`, `INSERT INTO %s.orders VALUES($1,false,'pending')`},
		{"work_order", `CREATE TABLE %s.work_orders(target_warehouse TEXT,status TEXT)`, `INSERT INTO %s.work_orders VALUES($1,'released')`},
		{"fulfillment", `CREATE TABLE %s.customer_fulfillment_settings(modes_json JSONB)`, `INSERT INTO %s.customer_fulfillment_settings VALUES(jsonb_build_array(jsonb_build_object('mode','stock','warehouse_code',$1::text,'enabled',true)))`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, schema := virtualWarehouseTestDB(t)
			ctx := context.Background()
			repo := NewRepository(pool, schema)
			row := createVirtualForTest(t, repo, 1)
			mustExecStockSQL(t, ctx, pool, fmt.Sprintf(tc.ddl, schema))
			if _, err := pool.Exec(ctx, fmt.Sprintf(tc.insert, schema), row.Code); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.DeleteVirtualWarehouse(ctx, stockapp.DeleteVirtualWarehouseCommand{Code: row.Code, ExpectedRevision: 1}); err == nil {
				t.Fatal("deleted warehouse with outstanding reference")
			}
			if _, err := repo.UpdateVirtualWarehouse(ctx, stockapp.UpdateVirtualWarehouseCommand{Code: row.Code, Name: row.Name, CustomerID: 2, ExpectedRevision: 1}); err == nil {
				t.Fatal("rebound used virtual warehouse")
			}
			if _, err := repo.BindWarehouseCustomer(ctx, stockapp.BindWarehouseCustomerCommand{WarehouseCode: row.Code, CustomerID: 2}); err == nil {
				t.Fatal("legacy binding bypassed history guard")
			}
			if _, err := repo.UpdateVirtualWarehouse(ctx, stockapp.UpdateVirtualWarehouseCommand{Code: row.Code, Name: "仍可改仓名", CustomerID: 1, ExpectedRevision: 1, Actor: "tester"}); err != nil {
				t.Fatalf("metadata edit unnecessarily blocked: %v", err)
			}
		})
	}
}

func TestVirtualWarehouseHistoricalLedgerBlocksRebindingButAllowsRetirement(t *testing.T) {
	pool, schema := virtualWarehouseTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	row := createVirtualForTest(t, repo, 1)
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.stock_ledger_entries(warehouse TEXT);`, schema))
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.stock_ledger_entries VALUES($1)`, schema), row.Code); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.BindWarehouseCustomer(ctx, stockapp.BindWarehouseCustomerCommand{WarehouseCode: row.Code, CustomerID: 0}); err == nil {
		t.Fatal("unbound historical warehouse")
	}
	if _, err := repo.DeleteVirtualWarehouse(ctx, stockapp.DeleteVirtualWarehouseCommand{Code: row.Code, ExpectedRevision: 1, Actor: "tester"}); err != nil {
		t.Fatalf("retire fully historical warehouse: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.stock_ledger_entries WHERE warehouse=$1`, schema), row.Code).Scan(&count); err != nil || count != 1 {
		t.Fatalf("history lost: %d %v", count, err)
	}
}

func TestVirtualWarehouseAuditFailureRollsBackCreateUpdateAndDelete(t *testing.T) {
	for _, action := range []string{"create", "update", "delete"} {
		t.Run(action, func(t *testing.T) {
			pool, schema := virtualWarehouseTestDB(t)
			ctx := context.Background()
			repo := NewRepository(pool, schema)
			row := createVirtualForTest(t, repo, 1)
			mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`ALTER TABLE %s.audit_logs ADD CONSTRAINT audit_unavailable CHECK(action <> '%s') NOT VALID`, schema, action))
			var err error
			switch action {
			case "create":
				_, err = repo.CreateVirtualWarehouse(ctx, stockapp.CreateVirtualWarehouseCommand{Name: "失败新仓", Kind: "finished", Actor: "tester"})
			case "update":
				_, err = repo.UpdateVirtualWarehouse(ctx, stockapp.UpdateVirtualWarehouseCommand{Code: row.Code, Name: "不应保存", CustomerID: 1, ExpectedRevision: 1, Actor: "tester"})
			case "delete":
				_, err = repo.DeleteVirtualWarehouse(ctx, stockapp.DeleteVirtualWarehouseCommand{Code: row.Code, ExpectedRevision: 1, Actor: "tester"})
			}
			if err == nil {
				t.Fatal("audit failure was ignored")
			}
			var active bool
			var revision int64
			var name string
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT active,revision,name FROM %s.warehouses WHERE code=$1`, schema), row.Code).Scan(&active, &revision, &name); err != nil {
				t.Fatal(err)
			}
			if !active || revision != 1 || name != row.Name {
				t.Fatalf("write survived audit failure: %v %d %s", active, revision, name)
			}
			var count int
			if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FROM %s.warehouses WHERE is_virtual`, schema)).Scan(&count); err != nil || count != 1 {
				t.Fatalf("failed creation leaked: %d %v", count, err)
			}
		})
	}
}

func TestVirtualWarehouseReferenceLocksSerializeRebindAndValidateStaleOwner(t *testing.T) {
	pool, schema := virtualWarehouseTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	// These are the real customer inventory columns; no synthetic status or ownership fields.
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`CREATE TABLE %s.customer_inventory_items(id BIGSERIAL PRIMARY KEY,customer_id BIGINT,warehouse TEXT,qty_g BIGINT DEFAULT 0,qty_units BIGINT DEFAULT 0,status TEXT DEFAULT 'available')`, schema))
	if err := EnsureVirtualWarehouseReferenceGuards(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	if err := EnsureVirtualWarehouseReferenceGuards(ctx, pool, schema); err != nil {
		t.Fatalf("guard reinstall: %v", err)
	}
	row := createVirtualForTest(t, repo, 1)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_inventory_items(customer_id,warehouse,qty_units)VALUES(1,$1,1)`, schema), row.Code); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := repo.UpdateVirtualWarehouse(ctx, stockapp.UpdateVirtualWarehouseCommand{Code: row.Code, Name: row.Name, CustomerID: 2, ExpectedRevision: 1, Actor: "tester"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("rebind did not wait for in-flight reference: %v", err)
	case <-time.After(80 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("rebound warehouse after committed customer inventory")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("rebind never resumed")
	}

	stale := createVirtualForTest(t, repo, 1)
	if _, err := repo.UpdateVirtualWarehouse(ctx, stockapp.UpdateVirtualWarehouseCommand{Code: stale.Code, Name: stale.Name, CustomerID: 2, ExpectedRevision: 1, Actor: "tester"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_inventory_items(customer_id,warehouse,qty_units)VALUES(1,$1,1)`, schema), stale.Code); err == nil {
		t.Fatal("stale customer-A writer entered customer-B warehouse")
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_inventory_items(customer_id,warehouse,qty_units)VALUES(2,$1,1)`, schema), stale.Code); err != nil {
		t.Fatalf("current owner rejected: %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_inventory_items SET customer_id=1 WHERE warehouse=$1`, schema), stale.Code); err == nil {
		t.Fatal("business owner update bypassed warehouse ownership guard")
	}

	retired := createVirtualForTest(t, repo, 1)
	if _, err := repo.DeleteVirtualWarehouse(ctx, stockapp.DeleteVirtualWarehouseCommand{Code: retired.Code, ExpectedRevision: 1, Actor: "tester"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.customer_inventory_items(customer_id,warehouse,qty_units)VALUES(1,$1,1)`, schema), retired.Code); err == nil {
		t.Fatal("new reference accepted for deleted warehouse")
	}
}

func TestVirtualWarehouseCompletedDocumentsAllowRetirementButNeverRebinding(t *testing.T) {
	pool, schema := virtualWarehouseTestDB(t)
	ctx := context.Background()
	repo := NewRepository(pool, schema)
	row := createVirtualForTest(t, repo, 1)
	mustExecStockSQL(t, ctx, pool, fmt.Sprintf(`
CREATE TABLE %[1]s.orders(id BIGINT PRIMARY KEY,source_warehouse TEXT,delivery_warehouse_code TEXT DEFAULT '',delivery_mode TEXT DEFAULT 'ship',ship_status_id BIGINT DEFAULT 2,is_void BOOLEAN DEFAULT false);
CREATE TABLE %[1]s.ship_statuses(id BIGINT PRIMARY KEY,name TEXT);INSERT INTO %[1]s.ship_statuses VALUES(1,'未发货'),(2,'已发货');
CREATE TABLE %[1]s.order_stock_batch_allocations(order_id BIGINT,warehouse TEXT);
CREATE TABLE %[1]s.customer_direct_ship_request_allocations(warehouse_code TEXT,status TEXT,order_id BIGINT);
CREATE TABLE %[1]s.production_plans(id BIGINT PRIMARY KEY,status TEXT);
CREATE TABLE %[1]s.production_plan_items(id BIGINT PRIMARY KEY,production_plan_id BIGINT,target_warehouse TEXT,replan_status TEXT DEFAULT 'active');
CREATE TABLE %[1]s.production_plan_component_sources(production_plan_item_id BIGINT,source_warehouse TEXT);
CREATE TABLE %[1]s.customer_processing_production_demands(target_warehouse TEXT,status TEXT);
`, schema))
	for _, statement := range []string{`INSERT INTO %s.orders(id,source_warehouse)VALUES(10,$1)`, `INSERT INTO %s.order_stock_batch_allocations VALUES(10,$1)`, `INSERT INTO %s.customer_direct_ship_request_allocations VALUES($1,'reserved',10)`, `WITH selected AS (SELECT $1::text) INSERT INTO %s.production_plans SELECT 20,'completed' FROM selected`, `INSERT INTO %s.production_plan_items VALUES(30,20,$1,'active')`, `INSERT INTO %s.production_plan_component_sources VALUES(30,$1)`, `INSERT INTO %s.customer_processing_production_demands VALUES($1,'done')`} {
		if _, err := pool.Exec(ctx, fmt.Sprintf(statement, schema), row.Code); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := repo.BindWarehouseCustomer(ctx, stockapp.BindWarehouseCustomerCommand{WarehouseCode: row.Code, CustomerID: 2}); err == nil {
		t.Fatal("rebound warehouse with completed business history")
	}
	if _, err := repo.DeleteVirtualWarehouse(ctx, stockapp.DeleteVirtualWarehouseCommand{Code: row.Code, ExpectedRevision: 1, Actor: "tester"}); err != nil {
		t.Fatalf("completed records incorrectly block retirement: %v", err)
	}
}
