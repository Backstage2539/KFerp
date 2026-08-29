package productspecmigration

import (
	"context"
	"errors"
	"fmt"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/jackc/pgx/v5"
)

// GuardDefaultProductBOMSwitchTx must run in the same transaction that changes
// production_bom_output_bindings. It applies while the current product uses
// BOM-spec identity. Stable specs may survive a version/default-BOM change;
// specs absent from the candidate version (all specs for a single-output BOM)
// must have no dependent mutable business state before the switch.
func GuardDefaultProductBOMSwitchTx(
	ctx context.Context,
	tx pgx.Tx,
	schema string,
	productID int64,
	candidateBOMID int64,
	candidateBOMVersionID int64,
) error {
	if productID <= 0 {
		return productspecmigrationapp.ErrProductRequired
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, productID); err != nil {
		return err
	}
	hasMigration, err := tableHasColumnsTx(ctx, tx, schema, "product_bom_spec_migrations", "product_id", "state")
	if err != nil || !hasMigration {
		return err
	}
	var state string
	var storedIdentityMode string
	var legacyCatalogProduct bool
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT state,
		       COALESCE(to_jsonb(product_bom_spec_migrations)->>'spec_identity_mode',''),
		       COALESCE((to_jsonb(product_bom_spec_migrations)->>'legacy_catalog_product')::boolean,true)
		FROM %s.product_bom_spec_migrations WHERE product_id=$1
		FOR UPDATE
	`, schema), productID).Scan(&state, &storedIdentityMode, &legacyCatalogProduct)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !productspecmigrationapp.IsBOMSpecAuthoritativeWithMode(storedIdentityMode, productspecmigrationapp.MigrationState(state), legacyCatalogProduct) {
		return nil
	}

	var currentBOMID, currentBOMVersionID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT bom_id,bom_version_id
		FROM %s.production_bom_output_bindings
		WHERE output_type='product' AND output_id=$1 AND is_default=true
		FOR UPDATE
	`, schema), productID).Scan(&currentBOMID, &currentBOMVersionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	currentSpecIDs, err := productionBOMVersionSpecIDsTx(ctx, tx, schema, currentBOMVersionID)
	if err != nil {
		return err
	}
	candidateSpecIDs, err := productionBOMVersionSpecIDsTx(ctx, tx, schema, candidateBOMVersionID)
	if err != nil {
		return err
	}
	candidateSet := make(map[int64]struct{}, len(candidateSpecIDs))
	for _, id := range candidateSpecIDs {
		candidateSet[id] = struct{}{}
	}
	removedSpecIDs := make([]int64, 0, len(currentSpecIDs))
	for _, id := range currentSpecIDs {
		if _, retained := candidateSet[id]; !retained {
			removedSpecIDs = append(removedSpecIDs, id)
		}
	}
	if len(removedSpecIDs) == 0 {
		return nil
	}

	counts, err := countOldBOMGroupActivityTx(ctx, tx, schema, productID, removedSpecIDs)
	if err != nil {
		return err
	}
	blockers := make([]productspecmigrationapp.Blocker, 0, 6)
	add := func(code string, count int64, message string) {
		if count > 0 {
			blockers = append(blockers, productspecmigrationapp.Blocker{Code: code, Count: count, Message: message})
		}
	}
	add("old_bom_stock", counts.stock, "旧默认 BOM 规格组仍有库存")
	add("old_bom_reservations", counts.reservations, "旧默认 BOM 规格组仍有有效预留")
	add("old_bom_unfinished_orders", counts.orders, "旧默认 BOM 规格组仍有未完成订单")
	add("old_bom_unfinished_plans", counts.plans, "旧默认 BOM 规格组仍有未完成生产计划")
	add("old_bom_unfinished_work_orders", counts.workOrders, "旧默认 BOM 规格组仍有未完成工单")
	add("old_bom_unfinished_fulfillment", counts.fulfillment, "旧默认 BOM 规格组仍有未完成履约")
	if len(blockers) == 0 {
		return nil
	}
	return &productspecmigrationapp.DefaultBOMSwitchBlockedError{
		ProductID:             productID,
		CurrentBOMID:          currentBOMID,
		CurrentBOMVersionID:   currentBOMVersionID,
		CandidateBOMID:        candidateBOMID,
		CandidateBOMVersionID: candidateBOMVersionID,
		Blockers:              blockers,
	}
}

// SetProductIdentityModeForDefaultBOMTx persists the business identity selected
// by the candidate default BOM after GuardDefaultProductBOMSwitchTx succeeds.
func SetProductIdentityModeForDefaultBOMTx(ctx context.Context, tx pgx.Tx, schema string, productID, candidateBOMID int64) (string, error) {
	var specificationMode string
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(NULLIF(specification_mode,''),'single')
		FROM %s.production_boms
		WHERE id=$1 AND output_type='product' AND output_product_id=$2
		FOR SHARE
	`, schema), candidateBOMID, productID).Scan(&specificationMode); err != nil {
		return "", err
	}
	identityMode := productspecmigrationapp.SpecIdentityModeProduct
	if specificationMode == "spec_group" {
		identityMode = productspecmigrationapp.SpecIdentityModeBOMSpec
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		INSERT INTO %s.product_bom_spec_migrations(product_id,state,legacy_catalog_product,spec_identity_mode,updated_at)
		VALUES($1,'legacy',true,$2,now())
		ON CONFLICT(product_id) DO UPDATE SET spec_identity_mode=excluded.spec_identity_mode,updated_at=now()
	`, schema), productID, identityMode); err != nil {
		return "", err
	}
	return identityMode, nil
}

func productionBOMVersionSpecIDsTx(ctx context.Context, tx pgx.Tx, schema string, versionID int64) ([]int64, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT DISTINCT bom_spec_id
		FROM %s.production_bom_version_variants
		WHERE version_id=$1 AND bom_spec_id>0
		ORDER BY bom_spec_id
	`, schema), versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	specIDs := make([]int64, 0)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		specIDs = append(specIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return specIDs, nil
}

type oldBOMGroupActivityCounts struct {
	stock        int64
	reservations int64
	orders       int64
	plans        int64
	workOrders   int64
	fulfillment  int64
}

func countOldBOMGroupActivityTx(ctx context.Context, tx pgx.Tx, schema string, productID int64, specIDs []int64) (oldBOMGroupActivityCounts, error) {
	var result oldBOMGroupActivityCounts
	queries := []struct {
		table   string
		columns []string
		query   string
		target  *int64
	}{
		{
			table: "finished_inventory", columns: []string{"product_id", "bom_spec_id", "onhand_units", "onhand_loose_g"}, target: &result.stock,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.finished_inventory WHERE product_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND (COALESCE(onhand_units,0)>0 OR COALESCE(onhand_loose_g,0)>0)`,
		},
		{
			table: "stock_batches", columns: []string{"item_type", "item_id", "bom_spec_id", "remaining_g", "remaining_units"}, target: &result.stock,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.stock_batches WHERE item_type IN ('product','finished_product') AND item_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND (COALESCE(remaining_g,0)>0 OR COALESCE(remaining_units,0)>0)`,
		},
		{
			table: "customer_inventory_items", columns: []string{"item_type", "item_id", "bom_spec_id", "qty_g", "qty_units"}, target: &result.stock,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.customer_inventory_items WHERE item_type IN ('product','finished_product') AND item_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND (COALESCE(qty_g,0)>0 OR COALESCE(qty_units,0)>0)`,
		},
		{
			table: "work_order_material_reservations", columns: []string{"component_type", "component_id", "bom_spec_id", "status"}, target: &result.reservations,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.work_order_material_reservations WHERE component_type IN ('product','finished_product') AND component_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND lower(COALESCE(status,'')) NOT IN ('consumed','released','cancelled','returned','completed')`,
		},
		{
			table: "customer_processing_material_reservations", columns: []string{"component_type", "component_product_id", "bom_spec_id", "status"}, target: &result.reservations,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.customer_processing_material_reservations WHERE component_type IN ('product','finished_product') AND component_product_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND lower(COALESCE(status,'')) NOT IN ('consumed','released','cancelled','returned','completed')`,
		},
		{
			table: "production_plan_items", columns: []string{"production_plan_id", "product_id", "bom_spec_id"}, target: &result.plans,
			query: `SELECT COUNT(DISTINCT plan.id)::bigint FROM %[1]s.production_plan_items item JOIN %[1]s.production_plans plan ON plan.id=item.production_plan_id WHERE item.product_id=$1 AND item.bom_spec_id=ANY($2::bigint[]) AND lower(COALESCE(plan.status,'')) NOT IN ('completed','cancelled','closed')`,
		},
		{
			table: "work_orders", columns: []string{"product_id", "bom_spec_id", "status"}, target: &result.workOrders,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.work_orders WHERE product_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','closed')`,
		},
		{
			table: "processing_job_request_items", columns: []string{"product_id", "bom_spec_id", "status"}, target: &result.fulfillment,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.processing_job_request_items WHERE product_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','closed')`,
		},
		{
			table: "customer_processing_production_demands", columns: []string{"product_id", "bom_spec_id", "status"}, target: &result.fulfillment,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.customer_processing_production_demands WHERE product_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','closed','done')`,
		},
		{
			table: "customer_processing_work_orders", columns: []string{"product_id", "bom_spec_id", "status"}, target: &result.fulfillment,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.customer_processing_work_orders WHERE product_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','closed','done')`,
		},
		{
			table: "customer_processing_packaging_jobs", columns: []string{"product_id", "bom_spec_id", "status"}, target: &result.fulfillment,
			query: `SELECT COUNT(*)::bigint FROM %[1]s.customer_processing_packaging_jobs WHERE product_id=$1 AND bom_spec_id=ANY($2::bigint[]) AND lower(COALESCE(status,'')) NOT IN ('completed','cancelled','closed','done')`,
		},
	}
	for _, candidate := range queries {
		ok, err := tableHasColumnsTx(ctx, tx, schema, candidate.table, candidate.columns...)
		if err != nil {
			return oldBOMGroupActivityCounts{}, err
		}
		if !ok {
			continue
		}
		var count int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(candidate.query, schema), productID, specIDs).Scan(&count); err != nil {
			return oldBOMGroupActivityCounts{}, err
		}
		*candidate.target += count
	}
	if err := countOldBOMOrderActivityTx(ctx, tx, schema, productID, specIDs, &result); err != nil {
		return oldBOMGroupActivityCounts{}, err
	}
	if err := countOldBOMDirectShipActivityTx(ctx, tx, schema, productID, specIDs, &result.fulfillment); err != nil {
		return oldBOMGroupActivityCounts{}, err
	}
	return result, nil
}

func countOldBOMOrderActivityTx(ctx context.Context, tx pgx.Tx, schema string, productID int64, specIDs []int64, result *oldBOMGroupActivityCounts) error {
	baseColumns := []string{"id", "ship_status_id", "is_void"}
	itemsOK, err := tableHasColumnsTx(ctx, tx, schema, "order_items", "order_id", "product_id", "bom_spec_id")
	if err != nil || !itemsOK {
		return err
	}
	ordersOK, err := tableHasColumnsTx(ctx, tx, schema, "orders", baseColumns...)
	if err != nil || !ordersOK {
		return err
	}
	statusesOK, err := tableHasColumnsTx(ctx, tx, schema, "ship_statuses", "id", "name")
	if err != nil {
		return err
	}
	statusJoin := ""
	statusWhere := ""
	if statusesOK {
		statusJoin = fmt.Sprintf(" LEFT JOIN %s.ship_statuses ship_status ON ship_status.id=orders.ship_status_id", schema)
		statusWhere = " AND lower(COALESCE(ship_status.name,'')) NOT IN ('已发货','shipped','completed','cancelled','closed') AND COALESCE(ship_status.name,'') NOT LIKE '%已发货%'"
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(DISTINCT orders.id)::bigint
		FROM %s.order_items item
		JOIN %s.orders orders ON orders.id=item.order_id%s
		WHERE item.product_id=$1 AND item.bom_spec_id=ANY($2::bigint[])
		  AND COALESCE(orders.is_void,false)=false%s
	`, schema, schema, statusJoin, statusWhere), productID, specIDs).Scan(&result.orders); err != nil {
		return err
	}

	allocOK, err := tableHasColumnsTx(ctx, tx, schema, "order_stock_batch_allocations", "order_id", "product_id", "bom_spec_id", "allocated_g", "allocated_units")
	if err != nil {
		return err
	}
	if allocOK {
		var count int64
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint
			FROM %s.order_stock_batch_allocations allocation
			JOIN %s.orders orders ON orders.id=allocation.order_id%s
			WHERE allocation.product_id=$1 AND allocation.bom_spec_id=ANY($2::bigint[])
			  AND (COALESCE(allocation.allocated_g,0)>0 OR COALESCE(allocation.allocated_units,0)>0)
			  AND COALESCE(orders.is_void,false)=false%s
		`, schema, schema, statusJoin, statusWhere), productID, specIDs).Scan(&count); err != nil {
			return err
		}
		result.reservations += count
	}
	return nil
}

func countOldBOMDirectShipActivityTx(ctx context.Context, tx pgx.Tx, schema string, productID int64, specIDs []int64, target *int64) error {
	itemsOK, err := tableHasColumnsTx(ctx, tx, schema, "customer_direct_ship_request_items", "request_id", "product_id", "bom_spec_id")
	if err != nil || !itemsOK {
		return err
	}
	requestsOK, err := tableHasColumnsTx(ctx, tx, schema, "customer_direct_ship_requests", "id", "status")
	if err != nil || !requestsOK {
		return err
	}
	var count int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(DISTINCT request.id)::bigint
		FROM %s.customer_direct_ship_request_items item
		JOIN %s.customer_direct_ship_requests request ON request.id=item.request_id
		WHERE item.product_id=$1 AND item.bom_spec_id=ANY($2::bigint[])
		  AND lower(COALESCE(request.status,'')) NOT IN ('completed','cancelled','closed','shipped')
	`, schema, schema), productID, specIDs).Scan(&count); err != nil {
		return err
	}
	*target += count
	return nil
}

func tableHasColumnsTx(ctx context.Context, tx pgx.Tx, schema, table string, columns ...string) (bool, error) {
	var count int
	err := tx.QueryRow(ctx, `
		SELECT COUNT(DISTINCT column_name)::int
		FROM information_schema.columns
		WHERE table_schema=$1 AND table_name=$2 AND column_name=ANY($3)
	`, schema, table, columns).Scan(&count)
	return count == len(columns), err
}
