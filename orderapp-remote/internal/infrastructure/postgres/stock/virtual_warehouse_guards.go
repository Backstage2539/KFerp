package stock

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EnsureVirtualWarehouseReferenceGuards runs again after all module schemas exist.
// Writers share-lock referenced virtual warehouses so deletion/rebinding cannot
// race a newly committed stock movement, entitlement, reservation or document.
func EnsureVirtualWarehouseReferenceGuards(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	var ready bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=$1 AND table_name='warehouses' AND column_name='is_virtual')`, schema).Scan(&ready); err != nil || !ready {
		return err
	}
	qualified := pgx.Identifier{schema, "guard_virtual_warehouse_reference"}.Sanitize()
	warehouses := pgx.Identifier{schema, "warehouses"}.Sanitize()
	if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE OR REPLACE FUNCTION %s() RETURNS trigger LANGUAGE plpgsql AS $guard$
DECLARE reference_column TEXT;warehouse_code TEXT;warehouse_active BOOLEAN;warehouse_virtual BOOLEAN;warehouse_customer BIGINT;record_owner BIGINT;record_data JSONB;
BEGIN
 FOREACH reference_column IN ARRAY TG_ARGV LOOP
  record_data:=to_jsonb(NEW);
  warehouse_code:=COALESCE(record_data->>reference_column,'');
  IF warehouse_code='' THEN CONTINUE; END IF;
  SELECT active,is_virtual,customer_id INTO warehouse_active,warehouse_virtual,warehouse_customer FROM %s WHERE code=warehouse_code FOR SHARE;
  IF FOUND AND warehouse_virtual AND NOT warehouse_active THEN
   RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='虚拟仓库已删除，请重新选择仓库';
  END IF;
  IF warehouse_virtual THEN
   record_owner:=NULL;
   IF reference_column LIKE 'source_%%' THEN
    record_owner:=COALESCE(NULLIF((record_data->>'source_owner_customer_id')::bigint,0),NULLIF((record_data->>'source_customer_id')::bigint,0));
   END IF;
   record_owner:=COALESCE(record_owner,NULLIF((record_data->>'owner_customer_id')::bigint,0),NULLIF((record_data->>'customer_id')::bigint,0));
   IF record_owner IS NULL AND record_data ? 'material_id' AND to_regclass(TG_TABLE_SCHEMA||'.materials') IS NOT NULL THEN
    EXECUTE format('SELECT COALESCE((to_jsonb(m)->>''owner_customer_id'')::bigint,0) FROM %%I.materials m WHERE to_jsonb(m)->>''id''=$1',TG_TABLE_SCHEMA) INTO record_owner USING record_data->>'material_id';
   END IF;
   IF record_owner>0 AND record_owner<>warehouse_customer THEN
    RAISE EXCEPTION USING ERRCODE='23514',MESSAGE='虚拟仓库绑定客户与业务货主不一致，请重新选择仓库';
   END IF;
  END IF;
 END LOOP;
 RETURN NEW;
END $guard$`, qualified, warehouses)); err != nil {
		return err
	}
	tables := []struct {
		table               string
		references, updates []string
	}{
		{"finished_inventory", []string{"warehouse"}, []string{"onhand_units", "onhand_loose_g"}},
		{"material_batch_locations", []string{"warehouse"}, []string{"qty_g", "qty_units"}},
		{"stock_ledger_entries", []string{"warehouse"}, nil},
		{"stock_entry_items", []string{"from_warehouse", "to_warehouse"}, []string{"qty_g", "qty_units"}},
		{"material_transfers", []string{"from_warehouse", "to_warehouse"}, nil},
		{"finished_product_transfers", []string{"from_warehouse", "to_warehouse"}, nil},
		{"stock_adjustments", []string{"warehouse"}, nil},
		{"customer_inventory_items", []string{"warehouse"}, []string{"qty_g", "qty_units", "status"}},
		{"customer_stock_entitlements", []string{"warehouse_code"}, []string{"promised_qty", "received_qty", "status"}},
		{"customer_stock_entitlement_receipts", []string{"warehouse_code"}, nil},
		{"customer_direct_ship_request_allocations", []string{"warehouse_code"}, []string{"status", "allocated_qty", "allocated_units", "allocated_g"}},
		{"customer_direct_ship_request_orders", []string{"warehouse_code"}, nil},
		{"order_stock_batch_allocations", []string{"warehouse"}, []string{"status"}},
		{"work_order_material_reservations", []string{"source_warehouse"}, []string{"status", "reserved_g", "reserved_units"}},
		{"customer_processing_material_reservations", []string{"source_warehouse_code"}, []string{"status", "reserved_g", "reserved_units"}},
		{"customer_processing_output_conversions", []string{"warehouse"}, nil},
		{"orders", []string{"delivery_warehouse_code", "source_warehouse"}, []string{"delivery_status", "is_void"}},
		{"work_orders", []string{"target_warehouse"}, []string{"status"}},
		{"production_plan_items", []string{"target_warehouse"}, []string{"status"}},
		{"production_plan_component_sources", []string{"source_warehouse"}, nil},
		{"processing_job_request_items", []string{"target_warehouse"}, []string{"status"}},
		{"customer_processing_production_demands", []string{"target_warehouse"}, []string{"status"}},
		{"work_order_material_reservation_batches", []string{"warehouse"}, []string{"status", "reserved_g", "reserved_units"}},
		{"production_supply_allocations", []string{"warehouse"}, []string{"status", "required_g", "required_units"}},
		{"produce_running_items", []string{"target_warehouse"}, []string{"status"}},
		{"purchase_orders", []string{"target_warehouse"}, []string{"status"}},
		{"purchase_receipts", []string{"target_warehouse"}, nil},
		{"delivery_note_forms", []string{"source_warehouse"}, nil},
		{"stock_adjustment_batch_allocations", []string{"warehouse"}, nil},
		{"customer_portal_profiles", []string{"processing_warehouse_code"}, nil},
	}
	for _, spec := range tables {
		rows, err := pool.Query(ctx, `SELECT column_name FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2`, schema, spec.table)
		if err != nil {
			return err
		}
		columns := map[string]bool{}
		for rows.Next() {
			var column string
			if err := rows.Scan(&column); err != nil {
				rows.Close()
				return err
			}
			columns[column] = true
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		var references, updates, args []string
		for _, column := range spec.references {
			if columns[column] {
				references = append(references, column)
				updates = append(updates, pgx.Identifier{column}.Sanitize())
				args = append(args, "'"+column+"'")
			}
		}
		if len(references) == 0 {
			continue
		}
		for _, column := range append(spec.updates, "customer_id", "owner_customer_id", "source_customer_id", "source_owner_customer_id") {
			if columns[column] {
				updates = append(updates, pgx.Identifier{column}.Sanitize())
			}
		}
		table := pgx.Identifier{schema, spec.table}.Sanitize()
		if _, err := pool.Exec(ctx, `DROP TRIGGER IF EXISTS guard_virtual_warehouse_reference ON `+table); err != nil {
			return err
		}
		if _, err := pool.Exec(ctx, fmt.Sprintf(`CREATE TRIGGER guard_virtual_warehouse_reference BEFORE INSERT OR UPDATE OF %s ON %s FOR EACH ROW EXECUTE FUNCTION %s(%s)`, strings.Join(updates, ","), table, qualified, strings.Join(args, ","))); err != nil {
			return err
		}
	}
	return nil
}
