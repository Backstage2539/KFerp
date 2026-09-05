package production

import (
	"context"
	"fmt"
	"math"
	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
)

type componentSourceIdentity struct {
	itemID, componentID, bomSpecID, bomVariantID, specG int64
	componentType, name, unit                           string
	requiredG, requiredUnits                            int64
	bomVersionID                                        int64
}

func productionPlanItemConsumptionNeeds(item productionapp.ProductionPlanItem) ([]materialConsumptionNeed, error) {
	plan := plannedFinishedInventoryAddition(item.SpecG, item.PlannedOutputG)
	if item.OutputType == "material" {
		outputG, outputUnits := canonicalFromManufacturingQty(item.OutputQty, item.OutputUnit)
		plan = InvQty{Units: outputUnits, LooseG: outputG}
	} else if item.SalesSpecCount > 0 {
		plan.Units = int64(math.Ceil(item.SalesSpecCount))
		if item.SpecG > 0 {
			plan.LooseG = nonnegativeQuantity(item.PlannedOutputG - plan.Units*item.SpecG)
		}
	}
	run := ProduceRunRow{
		Product: item.ProductName, ProductID: item.ProductID, SpecG: item.SpecG,
		NeedG: item.PlannedOutputG, InputG: item.PlannedG,
		PlanUnits: plan.Units, PlanLooseG: plan.LooseG, MaterialSnapshot: item.MaterialSnapshot,
	}
	needs, ok, err := materialSnapshotNeedsTx(run, plan)
	if err != nil || !ok {
		return nil, err
	}
	return aggregateManufacturingConsumptionNeeds(needs), nil
}

func syncProductionPlanComponentSourcesTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, items []productionapp.ProductionPlanItem) error {
	type dependencyQty struct{ g, units int64 }
	dependencies := map[string]dependencyQty{}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT production_plan_item_id,component_type,component_id,component_bom_spec_id,component_spec_g,
		       COALESCE(SUM(required_g),0)::bigint,COALESCE(SUM(required_units),0)::bigint
		FROM %s.production_plan_item_dependencies WHERE production_plan_id=$1
		GROUP BY production_plan_item_id,component_type,component_id,component_bom_spec_id,component_spec_g
	`, schema), planID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var itemID, componentID, bomSpecID, specG int64
		var componentType string
		var qty dependencyQty
		if err := rows.Scan(&itemID, &componentType, &componentID, &bomSpecID, &specG, &qty.g, &qty.units); err != nil {
			rows.Close()
			return err
		}
		dependencies[fmt.Sprintf("%d:%s", itemID, manufacturingReservationIdentityKey(componentType, componentID, bomSpecID, specG))] = qty
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	active := map[string]bool{}
	for _, item := range items {
		needs, err := productionPlanItemConsumptionNeeds(item)
		if err != nil {
			return err
		}
		for _, need := range needs {
			componentType, componentID, componentSpecG := manufacturingNeedIdentity(need)
			bomSpecID, bomVariantID := manufacturingNeedBOMSpecIdentity(need)
			requiredG, requiredUnits := manufacturingNeedCanonicalQuantities(need)
			dependency := dependencies[fmt.Sprintf("%d:%s", item.ID, manufacturingReservationIdentityKey(componentType, componentID, bomSpecID, componentSpecG))]
			requiredG = nonnegativeQuantity(requiredG - dependency.g)
			requiredUnits = nonnegativeQuantity(requiredUnits - dependency.units)
			if requiredG <= 0 && requiredUnits <= 0 {
				continue
			}
			identity := fmt.Sprintf("%d:%s", item.ID, manufacturingReservationIdentityKey(componentType, componentID, bomSpecID, componentSpecG))
			active[identity] = true
			if _, err := tx.Exec(ctx, fmt.Sprintf(`
				INSERT INTO %s.production_plan_component_sources(
					production_plan_id,production_plan_item_id,bom_version_id,component_type,component_id,
					component_bom_spec_id,component_bom_variant_id,component_spec_g,component_name,unit,
					required_g,required_units,created_at,updated_at
				) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,now(),now())
				ON CONFLICT(production_plan_item_id,component_type,component_id,component_bom_spec_id,component_spec_g) DO UPDATE SET
					production_plan_id=excluded.production_plan_id,bom_version_id=excluded.bom_version_id,
					component_bom_variant_id=excluded.component_bom_variant_id,component_name=excluded.component_name,
					unit=excluded.unit,required_g=excluded.required_g,required_units=excluded.required_units,updated_at=now()
			`, schema), planID, item.ID, item.BomVersionID, componentType, componentID, bomSpecID, bomVariantID,
				componentSpecG, need.MaterialName, strings.TrimSpace(need.Unit), requiredG, requiredUnits); err != nil {
				return err
			}
		}
	}
	existing, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,production_plan_item_id,component_type,component_id,component_bom_spec_id,component_spec_g
		FROM %s.production_plan_component_sources WHERE production_plan_id=$1
	`, schema), planID)
	if err != nil {
		return err
	}
	stale := make([]int64, 0)
	for existing.Next() {
		var id, itemID, componentID, bomSpecID, specG int64
		var componentType string
		if err := existing.Scan(&id, &itemID, &componentType, &componentID, &bomSpecID, &specG); err != nil {
			existing.Close()
			return err
		}
		if !active[fmt.Sprintf("%d:%s", itemID, manufacturingReservationIdentityKey(componentType, componentID, bomSpecID, specG))] {
			stale = append(stale, id)
		}
	}
	if err := existing.Err(); err != nil {
		existing.Close()
		return err
	}
	existing.Close()
	if len(stale) > 0 {
		_, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_plan_component_sources WHERE id=ANY($1::bigint[])`, schema), stale)
	}
	return err
}

func componentSourceAvailabilityTx(ctx context.Context, tx pgx.Tx, schema, componentType string, componentID, bomSpecID, specG int64, warehouse string, ownerCustomerID int64, lock bool) (int64, int64, error) {
	componentType = strings.ToLower(strings.TrimSpace(componentType))
	if componentType == "product" || componentType == "finished_product" {
		if lock {
			rows, err := tx.Query(ctx, fmt.Sprintf(`
				SELECT b.id FROM %s.stock_batches b
				LEFT JOIN LATERAL (
					SELECT l.warehouse FROM %s.stock_ledger_entries l
					WHERE l.item_type='finished_product' AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g
					  AND (l.source_batch_code=b.batch_code OR l.source_batch_id=b.batch_code)
					ORDER BY l.id DESC LIMIT 1
				) location ON true
				WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.spec_g=$3
				  AND COALESCE(NULLIF(location.warehouse,''),'finished_goods')=$4 AND COALESCE(b.owner_customer_id,0)=$5
				ORDER BY b.id FOR UPDATE OF b
			`, schema, schema), componentID, bomSpecID, specG, warehouse, ownerCustomerID)
			if err != nil {
				return 0, 0, err
			}
			for rows.Next() {
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return 0, 0, err
			}
			rows.Close()
		}
		var availableG, availableUnits int64
		err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(GREATEST(0,b.remaining_g-COALESCE(bound.reserved_g,0))),0)::bigint,
			       COALESCE(SUM(GREATEST(0,b.remaining_units-COALESCE(bound.reserved_units,0))),0)::bigint
			FROM %s.stock_batches b
			LEFT JOIN LATERAL (
				SELECT l.warehouse FROM %s.stock_ledger_entries l
				WHERE l.item_type='finished_product' AND l.item_id=b.item_id AND l.bom_spec_id=b.bom_spec_id AND l.spec_g=b.spec_g
				  AND (l.source_batch_code=b.batch_code OR l.source_batch_id=b.batch_code)
				ORDER BY l.id DESC LIMIT 1
			) location ON true
			LEFT JOIN LATERAL (
				SELECT COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)),0)::bigint AS reserved_g,
				       COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)),0)::bigint AS reserved_units
				FROM %s.work_order_material_reservation_batches rb
				WHERE rb.stock_batch_id=b.id AND rb.status='reserved'
			) bound ON true
			WHERE b.item_type='finished_product' AND b.item_id=$1 AND b.bom_spec_id=$2 AND b.spec_g=$3
			  AND COALESCE(NULLIF(location.warehouse,''),'finished_goods')=$4 AND COALESCE(b.owner_customer_id,0)=$5
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
			  AND (b.remaining_g>0 OR b.remaining_units>0)
		`, schema, schema, schema), componentID, bomSpecID, specG, warehouse, ownerCustomerID).Scan(&availableG, &availableUnits)
		return availableG, availableUnits, err
	}
	if lock {
		rows, err := tx.Query(ctx, fmt.Sprintf(`
			SELECT b.id FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			WHERE l.material_id=$1 AND l.warehouse=$2 AND COALESCE(b.owner_customer_id,0)=$3
			ORDER BY b.id FOR UPDATE OF b,l
		`, schema, schema), componentID, warehouse, ownerCustomerID)
		if err != nil {
			return 0, 0, err
		}
		for rows.Next() {
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return 0, 0, err
		}
		rows.Close()
	}
	var availableG, availableUnits int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,l.qty_g-COALESCE(bound.reserved_g,0))),0)::bigint,
		       COALESCE(SUM(GREATEST(0,l.qty_units-COALESCE(bound.reserved_units,0))),0)::bigint
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)),0)::bigint AS reserved_g,
			       COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)),0)::bigint AS reserved_units
			FROM %s.work_order_material_reservation_batches rb
			WHERE rb.material_batch_id=b.id AND rb.warehouse=l.warehouse AND rb.status='reserved'
		) bound ON true
		WHERE l.material_id=$1 AND l.warehouse=$2 AND COALESCE(b.owner_customer_id,0)=$3
		  AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		  AND (l.qty_g>0 OR l.qty_units>0)
	`, schema, schema, schema), componentID, warehouse, ownerCustomerID).Scan(&availableG, &availableUnits)
	return availableG, availableUnits, err
}

func componentSourceOptionsTx(ctx context.Context, tx pgx.Tx, schema string, source productionapp.ProductionPlanComponentSource, planCustomerID int64) ([]productionapp.ProductionPlanComponentSourceOption, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT code,name,COALESCE(kind,''),COALESCE(customer_id,0) FROM %s.warehouses WHERE active=true ORDER BY sort_order,code`, schema))
	if err != nil {
		return nil, err
	}
	type warehouseRow struct {
		code, name, kind string
		customerID       int64
	}
	warehouses := make([]warehouseRow, 0)
	for rows.Next() {
		var row warehouseRow
		if err := rows.Scan(&row.code, &row.name, &row.kind, &row.customerID); err != nil {
			rows.Close()
			return nil, err
		}
		if row.customerID > 0 && row.customerID != planCustomerID {
			continue
		}
		warehouses = append(warehouses, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	out := make([]productionapp.ProductionPlanComponentSourceOption, 0)
	for _, warehouse := range warehouses {
		owners := []int64{warehouse.customerID}
		if warehouse.customerID == 0 && planCustomerID > 0 && strings.EqualFold(strings.TrimSpace(warehouse.kind), "wip") {
			owners = []int64{0, planCustomerID}
		}
		for _, ownerCustomerID := range owners {
			availableG, availableUnits, err := componentSourceAvailabilityTx(ctx, tx, schema, source.ComponentType, source.ComponentID, source.ComponentBOMSpecID, source.ComponentSpecG, warehouse.code, ownerCustomerID, false)
			if err != nil {
				return nil, err
			}
			ownerName := "工厂"
			if ownerCustomerID > 0 {
				_ = tx.QueryRow(ctx, fmt.Sprintf(`SELECT name FROM %s.customers WHERE id=$1`, schema), ownerCustomerID).Scan(&ownerName)
			}
			out = append(out, productionapp.ProductionPlanComponentSourceOption{
				Warehouse: warehouse.code, WarehouseName: warehouse.name, OwnerCustomerID: ownerCustomerID,
				OwnerName: ownerName, AvailableG: availableG, AvailableUnits: availableUnits,
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].AvailableG != out[j].AvailableG {
			return out[i].AvailableG > out[j].AvailableG
		}
		if out[i].AvailableUnits != out[j].AvailableUnits {
			return out[i].AvailableUnits > out[j].AvailableUnits
		}
		if out[i].Warehouse != out[j].Warehouse {
			return out[i].Warehouse < out[j].Warehouse
		}
		return out[i].OwnerCustomerID < out[j].OwnerCustomerID
	})
	return out, nil
}

func loadProductionPlanComponentSourcesTx(ctx context.Context, tx pgx.Tx, schema string, planID int64) ([]productionapp.ProductionPlanComponentSource, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,production_plan_id,production_plan_item_id,bom_version_id,component_type,component_id,
		       component_bom_spec_id,component_bom_variant_id,component_spec_g,component_name,unit,
		       required_g,required_units,source_warehouse,source_owner_customer_id,
		       available_g_snapshot,available_units_snapshot
		FROM %s.production_plan_component_sources WHERE production_plan_id=$1
		ORDER BY production_plan_item_id,id
	`, schema), planID)
	if err != nil {
		return nil, err
	}
	out := make([]productionapp.ProductionPlanComponentSource, 0)
	for rows.Next() {
		var row productionapp.ProductionPlanComponentSource
		if err := rows.Scan(&row.ID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BOMVersionID,
			&row.ComponentType, &row.ComponentID, &row.ComponentBOMSpecID, &row.ComponentBOMVariantID,
			&row.ComponentSpecG, &row.ComponentName, &row.Unit, &row.RequiredG, &row.RequiredUnits,
			&row.SourceWarehouse, &row.SourceOwnerCustomerID, &row.AvailableGSnapshot, &row.AvailableUnitsSnapshot); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	itemCustomerIDs := map[int64]int64{}
	for i := range out {
		out[i].Selected = strings.TrimSpace(out[i].SourceWarehouse) != ""
		planCustomerID, ok := itemCustomerIDs[out[i].ProductionPlanItemID]
		if !ok {
			if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0) FROM %s.production_plan_items WHERE id=$1`, schema), out[i].ProductionPlanItemID).Scan(&planCustomerID); err != nil {
				return nil, err
			}
			itemCustomerIDs[out[i].ProductionPlanItemID] = planCustomerID
		}
		out[i].Options, err = componentSourceOptionsTx(ctx, tx, schema, out[i], planCustomerID)
		if err != nil {
			return nil, err
		}
		if out[i].Selected {
			availableG, availableUnits, err := componentSourceAvailabilityTx(ctx, tx, schema, out[i].ComponentType, out[i].ComponentID,
				out[i].ComponentBOMSpecID, out[i].ComponentSpecG, out[i].SourceWarehouse, out[i].SourceOwnerCustomerID, false)
			if err != nil {
				return nil, err
			}
			out[i].ShortageG = nonnegativeQuantity(out[i].RequiredG - availableG)
			out[i].ShortageUnits = nonnegativeQuantity(out[i].RequiredUnits - availableUnits)
		}
	}
	return out, nil
}

func warehouseCustomerID(ctx context.Context, tx pgx.Tx, schema, warehouse string) (int64, error) {
	var customerID int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(customer_id,0) FROM %s.warehouses WHERE code=$1 AND active=true`, schema), warehouse).Scan(&customerID)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("source warehouse not found or inactive: %s", warehouse)
	}
	return customerID, err
}

func validateComponentSourceOwner(planCustomerID, warehouseCustomerID, ownerCustomerID int64) (int64, error) {
	if warehouseCustomerID > 0 {
		if planCustomerID <= 0 || warehouseCustomerID != planCustomerID {
			return 0, fmt.Errorf("source warehouse belongs to another customer")
		}
		if ownerCustomerID > 0 && ownerCustomerID != warehouseCustomerID {
			return 0, fmt.Errorf("source owner does not match customer warehouse")
		}
		return warehouseCustomerID, nil
	}
	if ownerCustomerID > 0 && ownerCustomerID != planCustomerID {
		return 0, fmt.Errorf("source owner is outside production plan customer scope")
	}
	return ownerCustomerID, nil
}

func (r Repository) UpdateProductionPlanItemComponentSources(ctx context.Context, cmd productionapp.UpdateProductionPlanItemComponentSourcesCommand) ([]productionapp.ProductionPlanComponentSource, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status string
	var planCustomerID int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT plan.status,COALESCE(item.customer_id,0)
		FROM %s.production_plans plan
		JOIN %s.production_plan_items item ON item.production_plan_id=plan.id
		WHERE plan.id=$1 AND item.id=$2 FOR UPDATE OF plan,item
	`, r.schema, r.schema), cmd.ProductionPlanID, cmd.ProductionPlanItemID).Scan(&status, &planCustomerID); err != nil {
		return nil, err
	}
	if status != "draft" {
		return nil, fmt.Errorf("仅草稿生产计划可选择来源仓库")
	}
	for _, source := range cmd.Sources {
		warehouse := strings.TrimSpace(source.SourceWarehouse)
		warehouseOwner, err := warehouseCustomerID(ctx, tx, r.schema, warehouse)
		if err != nil {
			return nil, err
		}
		ownerCustomerID, err := validateComponentSourceOwner(planCustomerID, warehouseOwner, source.SourceOwnerCustomerID)
		if err != nil {
			return nil, err
		}
		availableG, availableUnits, err := componentSourceAvailabilityTx(ctx, tx, r.schema, source.ComponentType, source.ComponentID,
			source.ComponentBOMSpecID, source.ComponentSpecG, warehouse, ownerCustomerID, false)
		if err != nil {
			return nil, err
		}
		tag, err := tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.production_plan_component_sources SET source_warehouse=$6,source_owner_customer_id=$7,
			       available_g_snapshot=$8,available_units_snapshot=$9,selected_at=now(),selected_by=$10,updated_at=now()
			WHERE production_plan_id=$1 AND production_plan_item_id=$2 AND component_type=$3 AND component_id=$4
			  AND component_bom_spec_id=$5 AND component_spec_g=$11
		`, r.schema), cmd.ProductionPlanID, cmd.ProductionPlanItemID, source.ComponentType, source.ComponentID,
			source.ComponentBOMSpecID, warehouse, ownerCustomerID, availableG, availableUnits, cmd.Operator, source.ComponentSpecG)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, fmt.Errorf("component source does not belong to production plan item")
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_plan_component_source", nil,
			"select_source_warehouse", postgresinfra.StrPtr("source_warehouse"), nil, postgresinfra.StrPtr(warehouse),
			postgresinfra.AuditMeta{"production_plan_id": cmd.ProductionPlanID, "production_plan_item_id": cmd.ProductionPlanItemID,
				"component_type": source.ComponentType, "component_id": source.ComponentID, "owner_customer_id": ownerCustomerID}); err != nil {
			return nil, err
		}
	}
	rows, err := loadProductionPlanComponentSourcesTx(ctx, tx, r.schema, cmd.ProductionPlanID)
	if err != nil {
		return nil, err
	}
	filtered := make([]productionapp.ProductionPlanComponentSource, 0)
	for _, row := range rows {
		if row.ProductionPlanItemID == cmd.ProductionPlanItemID {
			filtered = append(filtered, row)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return filtered, nil
}

func validateProductionPlanComponentSourcesAtSubmitTx(ctx context.Context, tx pgx.Tx, schema string, planID int64, items []productionapp.ProductionPlanItem) error {
	if err := syncProductionPlanComponentSourcesTx(ctx, tx, schema, planID, items); err != nil {
		return err
	}
	rows, err := loadProductionPlanComponentSourcesTx(ctx, tx, schema, planID)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	for _, row := range rows {
		if !row.Selected {
			return fmt.Errorf("生产计划组件「%s」必须选择来源仓库", row.ComponentName)
		}
		availableG, availableUnits, err := componentSourceAvailabilityTx(ctx, tx, schema, row.ComponentType, row.ComponentID,
			row.ComponentBOMSpecID, row.ComponentSpecG, row.SourceWarehouse, row.SourceOwnerCustomerID, true)
		if err != nil {
			return err
		}
		if availableG < row.RequiredG || availableUnits < row.RequiredUnits {
			return fmt.Errorf("所选来源仓库存不足：%s / %s / 货主%d，缺少 %dg/%d units", row.ComponentName, row.SourceWarehouse,
				row.SourceOwnerCustomerID, nonnegativeQuantity(row.RequiredG-availableG), nonnegativeQuantity(row.RequiredUnits-availableUnits))
		}
	}
	return nil
}

func productionPlanComponentSourceForIdentityTx(ctx context.Context, tx pgx.Tx, schema string, planItemID int64, componentType string, componentID, bomSpecID, specG int64) (productionapp.ProductionPlanComponentSource, bool, error) {
	var row productionapp.ProductionPlanComponentSource
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,production_plan_id,production_plan_item_id,bom_version_id,component_type,component_id,
		       component_bom_spec_id,component_bom_variant_id,component_spec_g,component_name,unit,
		       required_g,required_units,source_warehouse,source_owner_customer_id,available_g_snapshot,available_units_snapshot
		FROM %s.production_plan_component_sources
		WHERE production_plan_item_id=$1 AND component_type=$2 AND component_id=$3 AND component_bom_spec_id=$4 AND component_spec_g=$5
	`, schema), planItemID, componentType, componentID, bomSpecID, specG).Scan(
		&row.ID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BOMVersionID, &row.ComponentType, &row.ComponentID,
		&row.ComponentBOMSpecID, &row.ComponentBOMVariantID, &row.ComponentSpecG, &row.ComponentName, &row.Unit,
		&row.RequiredG, &row.RequiredUnits, &row.SourceWarehouse, &row.SourceOwnerCustomerID,
		&row.AvailableGSnapshot, &row.AvailableUnitsSnapshot,
	)
	if err == pgx.ErrNoRows {
		return productionapp.ProductionPlanComponentSource{}, false, nil
	}
	return row, err == nil, err
}

func bindMaterialReservationBatchesTx(ctx context.Context, tx pgx.Tx, schema string, reservationID, workOrderID, materialID int64, warehouse string, ownerCustomerID, reserveG, reserveUnits int64) error {
	if reserveG <= 0 && reserveUnits <= 0 {
		return nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT b.id,b.batch_code,
		       GREATEST(0,l.qty_g-COALESCE(bound.reserved_g,0))::bigint,
		       GREATEST(0,l.qty_units-COALESCE(bound.reserved_units,0))::bigint
		FROM %s.material_batch_locations l
		JOIN %s.material_batches b ON b.id=l.material_batch_id
		LEFT JOIN LATERAL (
			SELECT COALESCE(SUM(GREATEST(0,rb.reserved_g-rb.consumed_g-rb.returned_g)),0)::bigint AS reserved_g,
			       COALESCE(SUM(GREATEST(0,rb.reserved_units-rb.consumed_units-rb.returned_units)),0)::bigint AS reserved_units
			FROM %s.work_order_material_reservation_batches rb
			WHERE rb.material_batch_id=b.id AND rb.warehouse=l.warehouse AND rb.status='reserved' AND rb.reservation_id<>$4
		) bound ON true
		WHERE l.material_id=$1 AND l.warehouse=$2 AND COALESCE(b.owner_customer_id,0)=$3
		  AND b.status='active' AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		ORDER BY b.received_at,b.id FOR UPDATE OF b,l
	`, schema, schema, schema), materialID, warehouse, ownerCustomerID, reservationID)
	if err != nil {
		return err
	}
	type batch struct {
		id, g, units int64
		code         string
	}
	batches := make([]batch, 0)
	for rows.Next() {
		var row batch
		if err := rows.Scan(&row.id, &row.code, &row.g, &row.units); err != nil {
			rows.Close()
			return err
		}
		batches = append(batches, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	remainingG, remainingUnits := reserveG, reserveUnits
	for _, batch := range batches {
		if remainingG <= 0 && remainingUnits <= 0 {
			break
		}
		addG, addUnits := minInt64(remainingG, batch.g), minInt64(remainingUnits, batch.units)
		if addG <= 0 && addUnits <= 0 {
			continue
		}
		if _, err := tx.Exec(ctx, fmt.Sprintf(`
			INSERT INTO %s.work_order_material_reservation_batches(
				reservation_id,work_order_id,material_id,component_type,component_id,material_batch_id,stock_batch_id,
				batch_code,warehouse,owner_customer_id,reserved_g,reserved_units,status,created_at,updated_at
			) VALUES($1,$2,$3,'material',$3,$4,0,$5,$6,$7,$8,$9,'reserved',now(),now())
			ON CONFLICT(reservation_id,component_type,component_id,component_bom_spec_id,component_spec_g,material_batch_id,stock_batch_id) DO UPDATE SET
				reserved_g=excluded.reserved_g,reserved_units=excluded.reserved_units,warehouse=excluded.warehouse,
				owner_customer_id=excluded.owner_customer_id,status='reserved',updated_at=now()
		`, schema), reservationID, workOrderID, materialID, batch.id, batch.code, warehouse, ownerCustomerID, addG, addUnits); err != nil {
			return err
		}
		remainingG -= addG
		remainingUnits -= addUnits
	}
	if remainingG > 0 || remainingUnits > 0 {
		return fmt.Errorf("selected material source became insufficient: material %d / %s / owner %d", materialID, warehouse, ownerCustomerID)
	}
	return nil
}
