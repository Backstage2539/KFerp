package productspecmigration

import (
	"context"
	"fmt"

	productspecmigrationapp "orderapp/internal/application/productspecmigration"

	"github.com/jackc/pgx/v5"
)

func (r Repository) assessReadinessTx(ctx context.Context, tx pgx.Tx, productID int64) (productspecmigrationapp.Readiness, error) {
	var counts readinessCounts
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::bigint FROM %s.products
		WHERE active=true AND %s
	`, r.schema, legacyChildCandidatePredicate("products")), productID).Scan(&counts.ActiveSpecs); err != nil {
		return productspecmigrationapp.Readiness{}, err
	}

	hasSpecs, err := relationExistsTx(ctx, tx, r.schema, "production_bom_specs")
	if err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	hasVariants, err := relationExistsTx(ctx, tx, r.schema, "production_bom_version_variants")
	if err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	if hasSpecs && hasVariants {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(DISTINCT child.id)::bigint
			FROM %s.products child
			JOIN %s.legacy_child_sku_bom_spec_mappings mapping
			  ON mapping.legacy_child_product_id=child.id AND mapping.parent_product_id=$1
			JOIN %s.production_bom_specs spec
			  ON spec.id=mapping.bom_spec_id AND spec.bom_id=mapping.bom_id
			JOIN %s.production_bom_output_bindings binding
			  ON binding.output_type='product' AND binding.output_id=$1 AND binding.is_default=true
			 AND binding.bom_id=mapping.bom_id
			JOIN %s.production_bom_versions version
			  ON version.id=binding.bom_version_id AND version.bom_id=binding.bom_id AND version.status='published'
			JOIN %s.production_bom_version_variants variant
			  ON variant.version_id=version.id AND variant.bom_spec_id=spec.id
			WHERE child.active=true AND %s
		`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, legacyChildCandidatePredicate("child")), productID).Scan(&counts.PublishedSpecs); err != nil {
			return productspecmigrationapp.Readiness{}, err
		}
		if counts.ActiveSpecs == 0 {
			var hasBindings bool
			hasBindings, err = relationExistsTx(ctx, tx, r.schema, "production_bom_output_bindings")
			if err != nil {
				return productspecmigrationapp.Readiness{}, err
			}
			if hasBindings {
				if err := tx.QueryRow(ctx, fmt.Sprintf(`
					SELECT COUNT(DISTINCT variant.bom_spec_id)::bigint
					FROM %[1]s.production_bom_output_bindings binding
					JOIN %[1]s.production_bom_versions version
					  ON version.id=binding.bom_version_id
					 AND version.bom_id=binding.bom_id
					 AND version.status='published'
					JOIN %[1]s.production_bom_specs spec
					  ON spec.bom_id=binding.bom_id
					JOIN %[1]s.production_bom_version_variants variant
					  ON variant.version_id=version.id
					 AND variant.bom_spec_id=spec.id
					WHERE binding.output_type='product'
					  AND binding.output_id=$1
					  AND binding.is_default=true
				`, r.schema), productID).Scan(&counts.ActiveSpecs); err != nil {
					return productspecmigrationapp.Readiness{}, err
				}
				counts.PublishedSpecs = counts.ActiveSpecs
			}
		}
	}
	counts.InvalidSpecTemplateProvenance, counts.InactiveMainInputMaterial, err = r.defaultPublishedBOMAuthorityReadinessTx(ctx, tx, productID)
	if err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::bigint
		FROM (
			SELECT lower(mapping.legacy_spec_key)
			FROM %s.legacy_child_sku_bom_spec_mappings mapping
			JOIN %s.products child ON child.id=mapping.legacy_child_product_id
			WHERE mapping.parent_product_id=$1 AND child.active=true AND %s
			GROUP BY lower(mapping.legacy_spec_key)
			HAVING COUNT(DISTINCT lower(btrim(mapping.legacy_sales_unit)))>1
		) ambiguous
	`, r.schema, r.schema, legacyChildCandidatePredicate("child")), productID).Scan(&counts.AmbiguousLegacySpecs); err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	if hasSpecs && hasVariants {
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint
			FROM %s.legacy_child_sku_bom_spec_mappings mapping
			JOIN %s.products child
			  ON child.id=mapping.legacy_child_product_id
			JOIN %s.production_bom_output_bindings binding
			  ON binding.output_type='product'
			 AND binding.output_id=mapping.parent_product_id
			 AND binding.bom_id=mapping.bom_id
			 AND binding.is_default=true
			JOIN %s.production_bom_versions version
			  ON version.id=binding.bom_version_id
			 AND version.bom_id=binding.bom_id
			 AND version.status='published'
			JOIN %s.production_bom_specs spec
			  ON spec.id=mapping.bom_spec_id
			 AND spec.bom_id=binding.bom_id
			JOIN %s.production_bom_version_variants variant
			  ON variant.version_id=version.id
			 AND variant.bom_spec_id=spec.id
			WHERE mapping.parent_product_id=$1
			  AND child.active=true
			  AND %s
			  AND (
			    btrim(COALESCE(mapping.legacy_sales_unit,''))=''
			    OR btrim(COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit,''))=''
			    OR lower(btrim(mapping.legacy_sales_unit))<>
			       lower(btrim(COALESCE(NULLIF(variant.inventory_unit,''),spec.inventory_unit,'')))
			  )
		`, r.schema, r.schema, r.schema, r.schema, r.schema, r.schema, legacyChildCandidatePredicate("child")), productID).Scan(&counts.LegacyUnitMismatches); err != nil {
			return productspecmigrationapp.Readiness{}, err
		}
	}

	stockCount, err := r.legacyStockCountTx(ctx, tx, productID)
	if err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	counts.LegacyStock = stockCount
	reservationCount, err := r.legacyReservationCountTx(ctx, tx, productID)
	if err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	counts.Reservations = reservationCount
	if err := r.countUnfinishedOrdersTx(ctx, tx, productID, &counts.UnfinishedOrders); err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	if err := r.countUnfinishedPlansTx(ctx, tx, productID, &counts.UnfinishedPlans); err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	if err := r.countUnfinishedWorkOrdersTx(ctx, tx, productID, &counts.UnfinishedWorkOrders); err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	fulfillment, err := r.unfinishedFulfillmentCountTx(ctx, tx, productID)
	if err != nil {
		return productspecmigrationapp.Readiness{}, err
	}
	counts.UnfinishedFulfillment = fulfillment
	return finalizeReadiness(counts), nil
}

// defaultPublishedBOMAuthorityReadinessTx verifies that the current product
// specification authority came from a specification-template version that was
// actually published and that its selected main input is still active. The row
// locks are held through cutover so a concurrent template, binding, BOM-version
// or material change cannot invalidate the readiness decision after it is made.
func (r Repository) defaultPublishedBOMAuthorityReadinessTx(ctx context.Context, tx pgx.Tx, productID int64) (int64, int64, error) {
	invalidProvenance := int64(1)
	inactiveMainInput := int64(1)
	requiredRelations := []struct {
		table   string
		columns []string
	}{
		{table: "production_bom_output_bindings", columns: []string{"output_type", "output_id", "bom_id", "bom_version_id", "is_default"}},
		{table: "production_bom_versions", columns: []string{"id", "bom_id", "status", "published_at", "source_spec_template_version_id", "main_input_material_id"}},
		{table: "production_bom_spec_template_versions", columns: []string{"id", "status", "published_at"}},
		{table: "materials", columns: []string{"id", "deprecated_at"}},
	}
	for _, required := range requiredRelations {
		hasColumns, err := tableHasColumnsTx(ctx, tx, r.schema, required.table, required.columns...)
		if err != nil {
			return 0, 0, err
		}
		if !hasColumns {
			return invalidProvenance, inactiveMainInput, nil
		}
	}

	var templateVersionID, mainInputMaterialID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(version.source_spec_template_version_id,0),
		       COALESCE(version.main_input_material_id,0)
		FROM %[1]s.production_bom_output_bindings binding
		JOIN %[1]s.production_bom_versions version
		  ON version.id=binding.bom_version_id
		 AND version.bom_id=binding.bom_id
		 AND version.status='published'
		 AND version.published_at IS NOT NULL
		WHERE binding.output_type='product'
		  AND binding.output_id=$1
		  AND binding.is_default=true
		FOR SHARE OF binding,version
	`, r.schema), productID).Scan(&templateVersionID, &mainInputMaterialID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return invalidProvenance, inactiveMainInput, nil
		}
		return 0, 0, err
	}

	if templateVersionID > 0 {
		var valid bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT lower(btrim(status)) IN ('published','archived') AND published_at IS NOT NULL
			FROM %s.production_bom_spec_template_versions
			WHERE id=$1
			FOR SHARE
		`, r.schema), templateVersionID).Scan(&valid)
		if err == nil && valid {
			invalidProvenance = 0
		} else if err != nil && err != pgx.ErrNoRows {
			return 0, 0, err
		}
	}
	if mainInputMaterialID > 0 {
		var active bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT deprecated_at IS NULL
			FROM %s.materials
			WHERE id=$1
			FOR SHARE
		`, r.schema), mainInputMaterialID).Scan(&active)
		if err == nil && active {
			inactiveMainInput = 0
		} else if err != nil && err != pgx.ErrNoRows {
			return 0, 0, err
		}
	}
	return invalidProvenance, inactiveMainInput, nil
}

func (r Repository) legacyStockCountTx(ctx context.Context, tx pgx.Tx, productID int64) (int64, error) {
	var total int64
	has, err := relationExistsTx(ctx, tx, r.schema, "finished_inventory")
	if err != nil {
		return 0, err
	}
	if has {
		var count int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint FROM %s.finished_inventory stock
			WHERE COALESCE(stock.bom_spec_id,0)=0
			  AND (COALESCE(stock.onhand_units,0)>0 OR COALESCE(stock.onhand_loose_g,0)>0)
			  AND %s
		`, r.schema, r.legacyProductPredicate("stock", "product_id", "spec_g")), productID).Scan(&count)
		if err != nil {
			return 0, err
		}
		total += count
	}
	has, err = relationExistsTx(ctx, tx, r.schema, "stock_batches")
	if err != nil {
		return 0, err
	}
	if has {
		var count int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint FROM %s.stock_batches stock
			WHERE stock.item_type='finished_product' AND COALESCE(stock.bom_spec_id,0)=0
			  AND (COALESCE(stock.remaining_g,0)>0 OR COALESCE(stock.remaining_units,0)>0)
			  AND %s
		`, r.schema, r.legacyProductPredicate("stock", "item_id", "spec_g")), productID).Scan(&count)
		if err != nil {
			return 0, err
		}
		total += count
	}
	has, err = relationExistsTx(ctx, tx, r.schema, "customer_inventory_items")
	if err != nil {
		return 0, err
	}
	if has {
		bomIdentityPredicate := ""
		hasBOMSpecID, err := tableHasColumnsTx(ctx, tx, r.schema, "customer_inventory_items", "bom_spec_id")
		if err != nil {
			return 0, err
		}
		if hasBOMSpecID {
			bomIdentityPredicate = "AND COALESCE(stock.bom_spec_id,0)=0"
		}
		var count int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint FROM %s.customer_inventory_items stock
			WHERE stock.item_type IN ('finished_product','product') %s
			  AND (COALESCE(stock.qty_g,0)>0 OR COALESCE(stock.qty_units,0)>0)
			  AND %s
		`, r.schema, bomIdentityPredicate, r.legacyProductPredicate("stock", "item_id", "spec_g")), productID).Scan(&count)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (r Repository) legacyReservationCountTx(ctx context.Context, tx pgx.Tx, productID int64) (int64, error) {
	var total int64
	has, err := relationExistsTx(ctx, tx, r.schema, "work_order_material_reservations")
	if err != nil {
		return 0, err
	}
	if has {
		var count int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint FROM %s.work_order_material_reservations reservation
			WHERE reservation.component_type IN ('product','finished_product')
			  AND reservation.status NOT IN ('consumed','released','cancelled','returned')
			  AND COALESCE(reservation.bom_spec_id,0)=0
			  AND %s
		`, r.schema, r.legacyProductPredicate("reservation", "component_id", "component_spec_g")), productID).Scan(&count)
		if err != nil {
			return 0, err
		}
		total += count
	}
	has, err = relationExistsTx(ctx, tx, r.schema, "customer_processing_material_reservations")
	if err != nil {
		return 0, err
	}
	if has {
		var count int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint FROM %s.customer_processing_material_reservations reservation
			WHERE reservation.component_type IN ('product','finished_product')
			  AND reservation.status NOT IN ('consumed','released','cancelled','returned')
			  AND COALESCE(reservation.bom_spec_id,0)=0
			  AND %s
		`, r.schema, r.legacyProductPredicate("reservation", "component_product_id", "component_spec_g")), productID).Scan(&count)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (r Repository) countUnfinishedOrdersTx(ctx context.Context, tx pgx.Tx, productID int64, target *int64) error {
	has, err := relationExistsTx(ctx, tx, r.schema, "order_items")
	if err != nil || !has {
		return err
	}
	legacyPredicate := fmt.Sprintf(`item.product_id IN (
		SELECT legacy_child_product_id FROM %s.legacy_child_sku_bom_spec_mappings WHERE parent_product_id=$1
	)`, r.schema)
	for _, specColumn := range []string{"unit_bean_g", "spec_g"} {
		hasSpecColumn, err := tableHasColumnsTx(ctx, tx, r.schema, "order_items", specColumn)
		if err != nil {
			return err
		}
		if hasSpecColumn {
			legacyPredicate = r.legacyProductPredicate("item", "product_id", specColumn)
			break
		}
	}
	return tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(DISTINCT orders.id)::bigint
		FROM %s.order_items item
		JOIN %s.orders orders ON orders.id=item.order_id
		LEFT JOIN %s.ship_statuses ship_status ON ship_status.id=orders.ship_status_id
		WHERE COALESCE(item.bom_spec_id,0)=0
		  AND %s
		  AND COALESCE(orders.is_void,false)=false
		  AND lower(COALESCE(ship_status.name,'')) NOT IN ('已发货','shipped','completed','cancelled')
		  AND COALESCE(ship_status.name,'') NOT LIKE '%%已发货%%'
	`, r.schema, r.schema, r.schema, legacyPredicate), productID).Scan(target)
}

func (r Repository) countUnfinishedPlansTx(ctx context.Context, tx pgx.Tx, productID int64, target *int64) error {
	has, err := relationExistsTx(ctx, tx, r.schema, "production_plan_items")
	if err != nil || !has {
		return err
	}
	return tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(DISTINCT plan.id)::bigint
		FROM %s.production_plan_items item
		JOIN %s.production_plans plan ON plan.id=item.production_plan_id
		WHERE COALESCE(item.bom_spec_id,0)=0 AND plan.status NOT IN ('completed','cancelled')
		  AND %s
	`, r.schema, r.schema, r.legacyProductPredicate("item", "product_id", "spec_g")), productID).Scan(target)
}

func (r Repository) countUnfinishedWorkOrdersTx(ctx context.Context, tx pgx.Tx, productID int64, target *int64) error {
	has, err := relationExistsTx(ctx, tx, r.schema, "work_orders")
	if err != nil || !has {
		return err
	}
	return tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::bigint FROM %s.work_orders work_order
		WHERE COALESCE(work_order.bom_spec_id,0)=0 AND work_order.status NOT IN ('completed','cancelled')
		  AND %s
	`, r.schema, r.legacyProductPredicate("work_order", "product_id", "spec_g")), productID).Scan(target)
}

func (r Repository) unfinishedFulfillmentCountTx(ctx context.Context, tx pgx.Tx, productID int64) (int64, error) {
	var total int64
	has, err := relationExistsTx(ctx, tx, r.schema, "customer_direct_ship_request_items")
	if err != nil {
		return 0, err
	}
	if has {
		var count int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(DISTINCT request.id)::bigint
			FROM %s.customer_direct_ship_request_items item
			JOIN %s.customer_direct_ship_requests request ON request.id=item.request_id
			WHERE COALESCE(item.bom_spec_id,0)=0
			  AND request.status NOT IN ('completed','cancelled','shipped')
			  AND %s
		`, r.schema, r.schema, r.legacyProductPredicate("item", "product_id", "spec_g")), productID).Scan(&count)
		if err != nil {
			return 0, err
		}
		total += count
	}
	has, err = relationExistsTx(ctx, tx, r.schema, "processing_job_request_items")
	if err != nil {
		return 0, err
	}
	if has {
		var count int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint FROM %s.processing_job_request_items item
			WHERE COALESCE(item.bom_spec_id,0)=0 AND item.status NOT IN ('completed','cancelled')
			  AND %s
		`, r.schema, r.legacyProductPredicate("item", "product_id", "spec_g")), productID).Scan(&count)
		if err != nil {
			return 0, err
		}
		total += count
	}
	hasDemandColumns, err := tableHasColumnsTx(ctx, tx, r.schema, "customer_processing_production_demands", "product_id", "spec_g", "bom_spec_id", "status")
	if err != nil {
		return 0, err
	}
	if hasDemandColumns {
		var count int64
		err = tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COUNT(*)::bigint FROM %s.customer_processing_production_demands demand
			WHERE COALESCE(demand.bom_spec_id,0)=0
			  AND lower(COALESCE(demand.status,'')) NOT IN ('completed','cancelled','closed','done')
			  AND %s
		`, r.schema, r.legacyProductPredicate("demand", "product_id", "spec_g")), productID).Scan(&count)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func (r Repository) legacyProductPredicate(alias, productColumn, specColumn string) string {
	return fmt.Sprintf(`(
		%s.%s IN (
			SELECT legacy_child_product_id FROM %s.legacy_child_sku_bom_spec_mappings WHERE parent_product_id=$1
		)
		OR (
			%s.%s=$1 AND %s.%s IN (
				SELECT legacy_spec_g FROM %s.legacy_child_sku_bom_spec_mappings
				WHERE parent_product_id=$1 AND legacy_spec_g>0
			)
		)
	)`, alias, productColumn, r.schema, alias, productColumn, alias, specColumn, r.schema)
}
