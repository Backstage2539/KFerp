package customerportal

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestAuthoritativeProcessingTargetSpecG(t *testing.T) {
	for _, tc := range []struct {
		qty   float64
		unit  string
		label string
		want  int64
	}{
		{qty: 227, unit: "g", label: "227g袋装", want: 227},
		{qty: 1, unit: "kg", label: "1kg", want: 1000},
		{qty: 1, unit: "lb", label: "1lb", want: 454},
		{label: "454g袋装", want: 454},
	} {
		if got := authoritativeProcessingTargetSpecG(tc.qty, tc.unit, tc.label); got != tc.want {
			t.Fatalf("authoritative spec %v%s/%q = %d, want %d", tc.qty, tc.unit, tc.label, got, tc.want)
		}
	}
}

func TestValidateProcessingTargetSpecGRejectsClientMismatch(t *testing.T) {
	if err := validateProcessingTargetSpecG(700, 454, 454); err != nil {
		t.Fatalf("matching concrete SKU spec rejected: %v", err)
	}
	if err := validateProcessingTargetSpecG(700, 227, 454); err == nil || !strings.Contains(err.Error(), "target specification mismatch") {
		t.Fatalf("client spec mismatch error = %v", err)
	}
	if err := validateProcessingTargetSpecG(700, 454, 0); err == nil || !strings.Contains(err.Error(), "no authoritative specification") {
		t.Fatalf("missing SKU authority error = %v", err)
	}
}

func TestCustomerProcessingFinishedWarehouseKindsAreExplicit(t *testing.T) {
	for _, kind := range []string{"finished", "customer_processing", "customer_finished", "customer"} {
		if !isCustomerProcessingFinishedWarehouseKind(kind) {
			t.Fatalf("expected compatible finished warehouse kind %q", kind)
		}
	}
	for _, kind := range []string{"raw", "wip", "packaging", "loss", ""} {
		if isCustomerProcessingFinishedWarehouseKind(kind) {
			t.Fatalf("non-finished warehouse kind %q must be rejected", kind)
		}
	}
}

func TestProcessingWarehouseRequiresFinishedBindingAndIsDeterministic(t *testing.T) {
	pool, schema := newCustomerPortalCoreOnlyTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %[1]s.customer_portal_profiles(
	customer_id BIGINT PRIMARY KEY,processing_warehouse_code TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %[1]s.warehouses(
	code TEXT PRIMARY KEY,kind TEXT NOT NULL,customer_id BIGINT NOT NULL,active BOOLEAN NOT NULL DEFAULT true,
	is_default BOOLEAN NOT NULL DEFAULT false,sort_order INTEGER NOT NULL DEFAULT 0
);
INSERT INTO %[1]s.customer_portal_profiles VALUES(77,'');
INSERT INTO %[1]s.warehouses(code,kind,customer_id,is_default,sort_order) VALUES
	('RAW-77','raw',77,true,1),('WIP-77','wip',77,true,2);
`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	resolve := func() (string, error) {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return "", err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		return repo.processingWarehouseForCustomerTx(ctx, tx, 77)
	}
	if _, err := resolve(); err == nil || !strings.Contains(err.Error(), "no active finished warehouse") {
		t.Fatalf("raw-only warehouse error = %v", err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
INSERT INTO %s.warehouses(code,kind,customer_id,is_default,sort_order) VALUES
	('PROCESSING-77','customer_processing',77,false,1),
	('FINISHED-DEFAULT-77','customer_finished',77,true,20),
	('FINISHED-OTHER-77','customer_finished',77,false,10);
`, schema)); err != nil {
		t.Fatal(err)
	}
	if got, err := resolve(); err != nil || got != "FINISHED-DEFAULT-77" {
		t.Fatalf("deterministic default warehouse = %q/%v", got, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_portal_profiles SET processing_warehouse_code='FINISHED-OTHER-77' WHERE customer_id=77`, schema)); err != nil {
		t.Fatal(err)
	}
	if got, err := resolve(); err != nil || got != "FINISHED-OTHER-77" {
		t.Fatalf("explicit profile warehouse = %q/%v", got, err)
	}
	if _, err := pool.Exec(ctx, fmt.Sprintf(`UPDATE %s.customer_portal_profiles SET processing_warehouse_code='RAW-77' WHERE customer_id=77`, schema)); err != nil {
		t.Fatal(err)
	}
	if _, err := resolve(); err == nil || !strings.Contains(err.Error(), "configured warehouse") {
		t.Fatalf("invalid explicit raw warehouse error = %v", err)
	}
}

func TestProcessingAvailabilityExcludesFinishedBatchAlreadyIssuedToAnotherWorkOrder(t *testing.T) {
	pool, schema := newCustomerPortalCoreOnlyTestDB(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`
CREATE TABLE %[1]s.warehouses(
	code TEXT PRIMARY KEY,kind TEXT NOT NULL,customer_id BIGINT NOT NULL DEFAULT 0,active BOOLEAN NOT NULL DEFAULT true
);
CREATE TABLE %[1]s.stock_batches(
	id BIGSERIAL PRIMARY KEY,batch_code TEXT NOT NULL UNIQUE,item_type TEXT NOT NULL,item_id BIGINT NOT NULL,
	spec_g BIGINT NOT NULL DEFAULT 0,remaining_g BIGINT NOT NULL DEFAULT 0,remaining_units BIGINT NOT NULL DEFAULT 0,
	quality_status TEXT NOT NULL DEFAULT 'unchecked'
);
CREATE TABLE %[1]s.stock_ledger_entries(
	id BIGSERIAL PRIMARY KEY,source_batch_code TEXT NOT NULL DEFAULT '',item_type TEXT NOT NULL DEFAULT '',
	item_id BIGINT NOT NULL DEFAULT 0,spec_g BIGINT NOT NULL DEFAULT 0,warehouse TEXT NOT NULL DEFAULT ''
);
CREATE TABLE %[1]s.customer_processing_material_reservations(
	id BIGSERIAL PRIMARY KEY,material_id BIGINT NOT NULL,component_type TEXT NOT NULL DEFAULT 'material',
	component_spec_g BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL DEFAULT 'reserved',
	reserved_g BIGINT NOT NULL DEFAULT 0,consumed_g BIGINT NOT NULL DEFAULT 0,returned_g BIGINT NOT NULL DEFAULT 0,
	reserved_units BIGINT NOT NULL DEFAULT 0,consumed_units BIGINT NOT NULL DEFAULT 0,returned_units BIGINT NOT NULL DEFAULT 0,
	source_warehouse_code TEXT NOT NULL DEFAULT '',material_batch_id BIGINT NOT NULL DEFAULT 0,
	finished_stock_batch_id BIGINT NOT NULL DEFAULT 0
);
INSERT INTO %[1]s.warehouses(code,kind,customer_id) VALUES('wip','wip',0);
INSERT INTO %[1]s.stock_batches(id,batch_code,item_type,item_id,spec_g,remaining_g,remaining_units,quality_status)
VALUES(10,'FP-WIP-A','finished_product',50,1000,400,0,'pass');
INSERT INTO %[1]s.stock_ledger_entries(source_batch_code,item_type,item_id,spec_g,warehouse)
VALUES('FP-WIP-A','finished_product',50,1000,'wip');
INSERT INTO %[1]s.customer_processing_material_reservations(
	id,material_id,component_type,component_spec_g,status,reserved_g,source_warehouse_code,finished_stock_batch_id
)
VALUES(20,50,'finished_product',1000,'reserved',400,'CUSTOMER-77-FINISHED',10);
`, schema)); err != nil {
		t.Fatal(err)
	}
	repo := NewRepository(pool, schema)
	for _, tc := range []struct {
		name string
		lock bool
	}{{name: "preview", lock: false}, {name: "submit_recheck", lock: true}} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			sources, preview, err := repo.processingAvailabilityTx(ctx, tx, 77, 50, "finished_product", 1000, tc.lock)
			if err != nil {
				t.Fatal(err)
			}
			if len(sources) != 1 || sources[0].WarehouseCode != "wip" || sources[0].AvailableG != 0 {
				t.Fatalf("available finished WIP sources = %+v, want bound batch excluded", sources)
			}
			if preview.AvailableG != 0 || preview.ReservedG != 400 {
				t.Fatalf("preview available/reserved = %d/%d, want 0/400", preview.AvailableG, preview.ReservedG)
			}
			if _, _, ok := allocateProcessingSources(sources, 400, 0); ok {
				t.Fatal("second request allocated finished batch already bound to work order A")
			}
		})
	}
}
