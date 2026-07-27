package production

import (
	"context"
	"fmt"
	"math"
	productionapp "orderapp/internal/application/production"
	productiondomain "orderapp/internal/domain/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"

	"github.com/jackc/pgx/v5"
)

func (r Repository) GetWorkOrderWIPCoverage(ctx context.Context, workOrderID int64) (productionapp.ProductionWIPStatus, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return productionapp.ProductionWIPStatus{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var wo productionapp.WorkOrderRow
	var materialSnapshot string
	err = tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_order_no,running_item_id,product_id,product_name,spec_g,
		       planned_g,COALESCE(NULLIF(planned_output_g,0),planned_g),COALESCE(sales_spec_count,0)::float8,COALESCE(order_nos,''),
		       status,COALESCE(operation_template_id,0),COALESCE(material_snapshot,'[]'::jsonb)::text
		FROM %s.work_orders
		WHERE id=$1
	`, r.schema), workOrderID).Scan(
		&wo.ID, &wo.WorkOrderNo, &wo.RunningItemID, &wo.ProductID, &wo.ProductName, &wo.SpecG,
		&wo.PlannedG, &wo.PlannedOutputG, &wo.SalesSpecCount, &wo.OrderNos, &wo.Status, &wo.OperationTemplateID, &materialSnapshot,
	)
	if err == pgx.ErrNoRows {
		return productionapp.ProductionWIPStatus{}, fmt.Errorf("work order not found")
	}
	if err != nil {
		return productionapp.ProductionWIPStatus{}, err
	}
	plan := runningInventoryPlan(wo.SpecG, wo.PlannedOutputG, wo.PlannedG, 1)
	if wo.SalesSpecCount > 0 {
		plan.Units = int64(math.Ceil(wo.SalesSpecCount))
		if wo.SpecG > 0 {
			plan.LooseG = nonnegativeQuantity(wo.PlannedOutputG - plan.Units*wo.SpecG)
		}
	}
	run := ProduceRunRow{
		ID: wo.RunningItemID, ProductID: wo.ProductID, Product: wo.ProductName,
		SpecG: wo.SpecG, NeedG: wo.PlannedOutputG, InputG: wo.PlannedG,
		PlanUnits: plan.Units, PlanLooseG: plan.LooseG, OrderNos: wo.OrderNos,
		OperationTemplateID: wo.OperationTemplateID, MaterialSnapshot: defaultJSONArray(materialSnapshot),
	}
	needs, ok, err := materialSnapshotNeedsTx(run, InvQty{Units: plan.Units, LooseG: plan.LooseG})
	if err != nil {
		return productionapp.ProductionWIPStatus{}, err
	}
	if !ok || len(needs) == 0 {
		needs, err = workOrderReservationNeedsTx(ctx, tx, r.schema, wo.ID)
		if err != nil {
			return productionapp.ProductionWIPStatus{}, err
		}
		if len(needs) == 0 {
			return productionapp.ProductionWIPStatus{
				DataComplete: false, Status: "blocked", BlockingReason: "WIP资料待完善",
				Materials: []productionapp.WIPReservationRow{},
			}, nil
		}
	}
	coverage, err := workOrderWIPCoverageForNeedsTx(ctx, tx, r.schema, workOrderID, needs)
	if err != nil {
		return productionapp.ProductionWIPStatus{}, err
	}
	hasStockEntries, err := schemaColumnExistsTx(ctx, tx, r.schema, "stock_entries", "purpose")
	if err != nil {
		return productionapp.ProductionWIPStatus{}, err
	}
	hasStockEntryItems, err := schemaColumnExistsTx(ctx, tx, r.schema, "stock_entry_items", "material_id")
	if err != nil {
		return productionapp.ProductionWIPStatus{}, err
	}
	status := productionapp.ProductionWIPStatus{
		DataComplete: true, Status: "ok",
		Materials: make([]productionapp.WIPReservationRow, 0, len(coverage)),
	}
	for _, item := range coverage {
		row := productionapp.WIPReservationRow{
			WorkOrderID: wo.ID, WorkOrderNo: wo.WorkOrderNo, RunningItemID: wo.RunningItemID,
			ProductName: wo.ProductName, MaterialID: item.Need.MaterialID,
			MaterialName: item.Need.MaterialName, Unit: item.Need.Unit,
			RequiredG: item.Need.DeductG, RequiredUnits: item.Need.DeductUnits,
			WIPG: item.WIPG, WIPUnits: item.WIPUnits,
			AvailableG: item.AvailableG, AvailableUnits: item.AvailableUnits,
			ShortageG: item.ShortageG, ShortageUnits: item.ShortageUnits,
			InventoryUnit: strings.TrimSpace(item.Need.Unit),
			Status:        "projected",
		}
		if row.InventoryUnit == "" {
			row.InventoryUnit = "g"
		}
		if item.Need.DeductG > 0 {
			row.QuantityBasis = "weight"
			row.RequiredQty = productionInventoryQuantity(item.Need.DeductG, row.InventoryUnit)
			row.AvailableQty = productionInventoryQuantity(item.AvailableG, row.InventoryUnit)
			row.ShortageQty = productionInventoryQuantity(item.ShortageG, row.InventoryUnit)
		} else {
			row.QuantityBasis = "count"
			row.RequiredQty = float64(item.Need.DeductUnits)
			row.AvailableQty = float64(item.AvailableUnits)
			row.ShortageQty = float64(item.ShortageUnits)
		}
		if hasStockEntries && hasStockEntryItems {
			var rememberedG, rememberedUnits int64
			err := tx.QueryRow(ctx, fmt.Sprintf(`
				SELECT si.qty_g,si.qty_units
				FROM %s.stock_entries se
				JOIN %s.stock_entry_items si ON si.stock_entry_id=se.id
				WHERE se.work_order_id=$1
				  AND se.purpose='material_transfer_for_manufacture'
				  AND se.is_return=false
				  AND se.status='submitted'
				  AND si.material_id=$2
				ORDER BY COALESCE(se.submitted_at,se.updated_at) DESC,se.id DESC,si.id DESC
				LIMIT 1
			`, r.schema, r.schema), wo.ID, row.MaterialID).Scan(&rememberedG, &rememberedUnits)
			if err != nil && err != pgx.ErrNoRows {
				return productionapp.ProductionWIPStatus{}, err
			}
			if row.QuantityBasis == "weight" {
				row.RememberedQty = productionInventoryQuantity(rememberedG, row.InventoryUnit)
			} else {
				row.RememberedQty = float64(rememberedUnits)
			}
		}
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			SELECT COALESCE(SUM(reserved_g),0)::bigint,COALESCE(SUM(reserved_units),0)::bigint,
			       COALESCE(SUM(consumed_g),0)::bigint,COALESCE(SUM(consumed_units),0)::bigint,
			       COALESCE(SUM(returned_g),0)::bigint,COALESCE(SUM(returned_units),0)::bigint,
			       COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint
			FROM %s.work_order_material_reservations
			WHERE work_order_id=$1 AND material_id=$2
		`, r.schema), wo.ID, row.MaterialID).Scan(
			&row.ReservedG, &row.ReservedUnits, &row.ConsumedG, &row.ConsumedUnits,
			&row.ReturnedG, &row.ReturnedUnits, &row.RemainingReservedG,
		); err != nil {
			return productionapp.ProductionWIPStatus{}, err
		}
		status.RequiredG += row.RequiredG
		status.RequiredUnits += row.RequiredUnits
		status.ReservedG += row.ReservedG
		status.ConsumedG += row.ConsumedG
		status.RemainingG += row.RemainingReservedG
		status.AvailableG += row.AvailableG
		status.AvailableUnits += row.AvailableUnits
		status.ShortageG += row.ShortageG
		status.ShortageUnits += row.ShortageUnits
		status.Materials = append(status.Materials, row)
	}
	if status.ShortageG > 0 || status.ShortageUnits > 0 {
		status.Status = "blocked"
		if status.ShortageG > 0 {
			status.BlockingReason = fmt.Sprintf("WIP 不足 %dg", status.ShortageG)
		} else {
			status.BlockingReason = fmt.Sprintf("WIP 不足 %d件", status.ShortageUnits)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.ProductionWIPStatus{}, err
	}
	return status, nil
}

func workOrderReservationNeedsTx(ctx context.Context, tx pgx.Tx, schema string, workOrderID int64) ([]materialConsumptionNeed, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT material_id,COALESCE(NULLIF(MAX(material_name),''),concat('material ',material_id)),
		       COALESCE(NULLIF(MAX(unit),''),'g'),
		       COALESCE(SUM(required_g),0)::bigint,COALESCE(SUM(required_units),0)::bigint
		FROM %s.work_order_material_reservations
		WHERE work_order_id=$1
		GROUP BY material_id
		ORDER BY material_id
	`, schema), workOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	needs := make([]materialConsumptionNeed, 0)
	for rows.Next() {
		var need materialConsumptionNeed
		if err := rows.Scan(&need.MaterialID, &need.MaterialName, &need.Unit, &need.DeductG, &need.DeductUnits); err != nil {
			return nil, err
		}
		if need.MaterialID > 0 && (need.DeductG > 0 || need.DeductUnits > 0) {
			needs = append(needs, need)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return needs, nil
}

func productionInventoryQuantity(qtyG int64, unit string) float64 {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "kg", "千克", "公斤":
		return float64(qtyG) / 1000
	case "lb", "磅":
		return float64(qtyG) / 453.59237
	default:
		return float64(qtyG)
	}
}

func (r Repository) GetWorkOrderStockDocumentDraft(ctx context.Context, workOrderID int64, action string) (*productionapp.StockEntryCommand, error) {
	hasStockEntries := false
	if err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM information_schema.columns
			WHERE table_schema=$1 AND table_name='stock_entries' AND column_name='purpose'
		)
	`, r.schema).Scan(&hasStockEntries); err != nil {
		return nil, err
	}
	if !hasStockEntries {
		return nil, nil
	}
	purpose, isReturn := "", false
	switch strings.TrimSpace(action) {
	case "issue", "supplement":
		purpose = "material_transfer_for_manufacture"
	case "return":
		purpose, isReturn = "material_transfer_for_manufacture", true
	case "consume":
		purpose = "material_consumption_for_manufacture"
	case "finish":
		purpose = "manufacture"
	default:
		return nil, nil
	}
	var draft productionapp.StockEntryCommand
	err := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,entry_no,status,entry_type,purpose,is_return,work_order_id,job_card_id,running_item_id,
		       source_type,source_id,return_source,operator,note
		FROM %s.stock_entries
		WHERE work_order_id=$1 AND purpose=$2 AND is_return=$3 AND status='draft'
		ORDER BY updated_at DESC,id DESC
		LIMIT 1
	`, r.schema), workOrderID, purpose, isReturn).Scan(
		&draft.ID, &draft.EntryNo, &draft.Status, &draft.EntryType, &draft.Purpose, &draft.IsReturn,
		&draft.WorkOrderID, &draft.JobCardID, &draft.RunningItemID, &draft.SourceType, &draft.SourceID,
		&draft.ReturnSource, &draft.Operator, &draft.Note,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT material_id,product_id,item_type,item_name,spec_g,inventory_unit,
		       from_warehouse,to_warehouse,qty_g,qty_units,batch_code,COALESCE(unit_cost,0)::float8
		FROM %s.stock_entry_items
		WHERE stock_entry_id=$1
		ORDER BY id
	`, r.schema), draft.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item productionapp.StockEntryItemCommand
		if err := rows.Scan(
			&item.MaterialID, &item.ProductID, &item.ItemType, &item.ItemName, &item.SpecG, &item.InventoryUnit,
			&item.FromWarehouse, &item.ToWarehouse, &item.QtyG, &item.QtyUnits, &item.BatchCode, &item.UnitCost,
		); err != nil {
			return nil, err
		}
		if item.QtyG > 0 {
			item.QuantityBasis = "weight"
			item.DefaultQty = productionInventoryQuantity(item.QtyG, item.InventoryUnit)
		} else {
			item.QuantityBasis = "count"
			item.DefaultQty = float64(item.QtyUnits)
		}
		draft.Items = append(draft.Items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(draft.Items) == 0 {
		return nil, nil
	}
	return &draft, nil
}

func (r Repository) ListWIPReservations(ctx context.Context, query productionapp.WIPReservationQuery) (productionapp.WIPReservationResult, error) {
	where := []string{"1=1"}
	args := []any{}
	if query.Status != "" {
		args = append(args, query.Status)
		where = append(where, fmt.Sprintf("res.status=$%d", len(args)))
	}
	if query.WorkOrderNo != "" {
		args = append(args, query.WorkOrderNo)
		where = append(where, fmt.Sprintf("wo.work_order_no=$%d", len(args)))
	}
	if query.MaterialID > 0 {
		args = append(args, query.MaterialID)
		where = append(where, fmt.Sprintf("res.material_id=$%d", len(args)))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT res.id,res.work_order_id,COALESCE(wo.work_order_no,''),res.running_item_id,
		       COALESCE(wo.product_name,''),res.material_id,res.material_name,res.unit,
		       res.required_g,res.required_units,res.reserved_g,res.reserved_units,
		       res.consumed_g,res.consumed_units,res.returned_g,res.returned_units,
		       GREATEST(0,res.reserved_g-res.consumed_g-res.returned_g)::bigint AS remaining_reserved_g,
		       res.status,
		       COALESCE(wip.wip_g,0)::bigint AS wip_g,
		       GREATEST(0,COALESCE(wip.wip_g,0)-COALESCE(open_res.open_reserved_g,0))::bigint AS available_g,
		       to_char(res.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.work_order_material_reservations res
		LEFT JOIN %s.work_orders wo ON wo.id=res.work_order_id
		LEFT JOIN LATERAL (
			SELECT SUM(l.qty_g)::bigint AS wip_g
			FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			WHERE l.material_id=res.material_id
			  AND l.warehouse='wip'
			  AND l.qty_g > 0
			  AND b.status='active'
			  AND b.remaining_g > 0
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		) wip ON true
		LEFT JOIN LATERAL (
			SELECT SUM(GREATEST(0,r2.reserved_g-r2.consumed_g-r2.returned_g))::bigint AS open_reserved_g
			FROM %s.work_order_material_reservations r2
			WHERE r2.material_id=res.material_id AND r2.status='reserved'
		) open_res ON true
		WHERE %s
		ORDER BY res.updated_at DESC,res.id DESC
		LIMIT $%d
	`, r.schema, r.schema, r.schema, r.schema, r.schema, strings.Join(where, " AND "), limitArg), args...)
	if err != nil {
		return productionapp.WIPReservationResult{}, err
	}
	defer rows.Close()
	out := make([]productionapp.WIPReservationRow, 0)
	var totalReservedG, totalConsumedG, totalRemainingG int64
	for rows.Next() {
		var row productionapp.WIPReservationRow
		if err := rows.Scan(
			&row.ID, &row.WorkOrderID, &row.WorkOrderNo, &row.RunningItemID,
			&row.ProductName, &row.MaterialID, &row.MaterialName, &row.Unit,
			&row.RequiredG, &row.RequiredUnits, &row.ReservedG, &row.ReservedUnits,
			&row.ConsumedG, &row.ConsumedUnits, &row.ReturnedG, &row.ReturnedUnits,
			&row.RemainingReservedG, &row.Status, &row.WIPG, &row.AvailableG, &row.UpdatedAt,
		); err != nil {
			return productionapp.WIPReservationResult{}, err
		}
		totalReservedG += row.ReservedG
		totalConsumedG += row.ConsumedG
		totalRemainingG += row.RemainingReservedG
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return productionapp.WIPReservationResult{}, err
	}
	return productionapp.WIPReservationResult{Rows: out, TotalReservedG: totalReservedG, TotalConsumedG: totalConsumedG, TotalRemainingG: totalRemainingG}, nil
}

func (r Repository) AdjustWIPReservation(ctx context.Context, cmd productionapp.WIPReservationAdjustCommand) (productionapp.WIPReservationRow, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	defer tx.Rollback(ctx)

	var current productionapp.WIPReservationRow
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_order_id,running_item_id,material_id,material_name,unit,reserved_g,reserved_units,consumed_g,consumed_units,returned_g,returned_units,status
		FROM %s.work_order_material_reservations
		WHERE id=$1
		FOR UPDATE
	`, r.schema), cmd.ReservationID).Scan(
		&current.ID, &current.WorkOrderID, &current.RunningItemID, &current.MaterialID, &current.MaterialName, &current.Unit,
		&current.ReservedG, &current.ReservedUnits, &current.ConsumedG, &current.ConsumedUnits, &current.ReturnedG, &current.ReturnedUnits, &current.Status,
	); err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	if current.Status != "reserved" {
		return productionapp.WIPReservationRow{}, fmt.Errorf("only reserved WIP reservations can be adjusted")
	}
	if cmd.ReservedUnits < current.ConsumedUnits+current.ReturnedUnits {
		return productionapp.WIPReservationRow{}, fmt.Errorf("reserved_units cannot be less than consumed and returned quantity")
	}
	wipG, err := materialWarehouseGTx(ctx, tx, r.schema, current.MaterialID, "wip")
	if err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	otherReservedG, err := reservedWIPGForMaterialExceptReservationTx(ctx, tx, r.schema, current.MaterialID, current.ID)
	if err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	adjusted, err := productiondomain.ValidateWIPReservationAdjustment(productiondomain.WIPReservationAdjustment{
		Current: productiondomain.WIPReservationQuantity{
			ReservedG: current.ReservedG,
			ConsumedG: current.ConsumedG,
			ReturnedG: current.ReturnedG,
		},
		TargetReservedG: cmd.ReservedG,
		WIPG:            wipG,
		OtherReservedG:  otherReservedG,
	})
	if err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	reservedUnits := cmd.ReservedUnits
	if reservedUnits == 0 && current.ReservedUnits > 0 {
		reservedUnits = current.ReservedUnits
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations
		SET reserved_g=$2,reserved_units=$3,updated_at=now()
		WHERE id=$1
	`, r.schema), current.ID, adjusted.ReservedG, reservedUnits); err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	oldValue := fmt.Sprintf("%d", current.ReservedG)
	newValue := fmt.Sprintf("%d", adjusted.ReservedG)
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "wip_reservation", &current.ID, "adjust", postgresinfra.StrPtr("reserved_g"), postgresinfra.StrPtr(oldValue), postgresinfra.StrPtr(newValue), postgresinfra.AuditMeta{"note": cmd.Note, "running_item_id": current.RunningItemID, "material_id": current.MaterialID}); err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	row, err := r.getWIPReservationRowTx(ctx, tx, current.ID)
	if err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	return row, nil
}

func (r Repository) ReleaseWIPReservations(ctx context.Context, cmd productionapp.WIPReservationReleaseCommand) (productionapp.WIPReservationReleaseResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return productionapp.WIPReservationReleaseResult{}, err
	}
	defer tx.Rollback(ctx)

	where := []string{"res.status='reserved'"}
	args := []any{}
	if cmd.RunningItemID > 0 {
		args = append(args, cmd.RunningItemID)
		where = append(where, fmt.Sprintf("res.running_item_id=$%d", len(args)))
	}
	if cmd.WorkOrderNo != "" {
		args = append(args, cmd.WorkOrderNo)
		where = append(where, fmt.Sprintf("EXISTS (SELECT 1 FROM %s.work_orders wo WHERE wo.id=res.work_order_id AND wo.work_order_no=$%d)", r.schema, len(args)))
	}
	whereSQL := strings.Join(where, " AND ")
	var result productionapp.WIPReservationReleaseResult
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COUNT(*)::bigint,
		       COALESCE(SUM(GREATEST(0,res.reserved_g-res.consumed_g-res.returned_g)),0)::bigint,
		       COALESCE(SUM(GREATEST(0,res.reserved_units-res.consumed_units-res.returned_units)),0)::bigint
		FROM %s.work_order_material_reservations res
		WHERE %s
	`, r.schema, whereSQL), args...).Scan(&result.ReleasedCount, &result.ReleasedG, &result.ReleasedUnits); err != nil {
		return productionapp.WIPReservationReleaseResult{}, err
	}
	if _, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_order_material_reservations res
		SET status='released',
		    returned_g=GREATEST(0,reserved_g-consumed_g),
		    returned_units=GREATEST(0,reserved_units-consumed_units),
		    updated_at=now()
		WHERE %s
	`, r.schema, whereSQL), args...); err != nil {
		return productionapp.WIPReservationReleaseResult{}, err
	}
	var entityID *int64
	if cmd.RunningItemID > 0 {
		entityID = &cmd.RunningItemID
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "wip_reservation", entityID, "release", postgresinfra.StrPtr("work_order_no"), nil, postgresinfra.StrPtr(cmd.WorkOrderNo), postgresinfra.AuditMeta{"note": cmd.Note, "released_count": result.ReleasedCount, "released_g": result.ReleasedG}); err != nil {
		return productionapp.WIPReservationReleaseResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.WIPReservationReleaseResult{}, err
	}
	return result, nil
}

func reservedWIPGForMaterialExceptReservationTx(ctx context.Context, tx pgx.Tx, schema string, materialID, reservationID int64) (int64, error) {
	var reservedG int64
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT COALESCE(SUM(GREATEST(0,reserved_g-consumed_g-returned_g)),0)::bigint
		FROM %s.work_order_material_reservations
		WHERE material_id=$1 AND status='reserved' AND id<>$2
	`, schema), materialID, reservationID).Scan(&reservedG)
	return reservedG, err
}

func (r Repository) getWIPReservationRowTx(ctx context.Context, tx pgx.Tx, id int64) (productionapp.WIPReservationRow, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT res.id,res.work_order_id,COALESCE(wo.work_order_no,''),res.running_item_id,
		       COALESCE(wo.product_name,''),res.material_id,res.material_name,res.unit,
		       res.required_g,res.required_units,res.reserved_g,res.reserved_units,
		       res.consumed_g,res.consumed_units,res.returned_g,res.returned_units,
		       GREATEST(0,res.reserved_g-res.consumed_g-res.returned_g)::bigint,
		       res.status,
		       COALESCE(wip.wip_g,0)::bigint,
		       GREATEST(0,COALESCE(wip.wip_g,0)-COALESCE(open_res.open_reserved_g,0))::bigint,
		       to_char(res.updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.work_order_material_reservations res
		LEFT JOIN %s.work_orders wo ON wo.id=res.work_order_id
		LEFT JOIN LATERAL (
			SELECT SUM(l.qty_g)::bigint AS wip_g
			FROM %s.material_batch_locations l
			JOIN %s.material_batches b ON b.id=l.material_batch_id
			WHERE l.material_id=res.material_id
			  AND l.warehouse='wip'
			  AND l.qty_g > 0
			  AND b.status='active'
			  AND b.remaining_g > 0
			  AND COALESCE(b.quality_status,'unchecked') NOT IN ('hold','reject')
		) wip ON true
		LEFT JOIN LATERAL (
			SELECT SUM(GREATEST(0,r2.reserved_g-r2.consumed_g-r2.returned_g))::bigint AS open_reserved_g
			FROM %s.work_order_material_reservations r2
			WHERE r2.material_id=res.material_id AND r2.status='reserved'
		) open_res ON true
		WHERE res.id=$1
	`, r.schema, r.schema, r.schema, r.schema, r.schema), id)
	if err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		return productionapp.WIPReservationRow{}, pgx.ErrNoRows
	}
	var row productionapp.WIPReservationRow
	err = rows.Scan(
		&row.ID, &row.WorkOrderID, &row.WorkOrderNo, &row.RunningItemID,
		&row.ProductName, &row.MaterialID, &row.MaterialName, &row.Unit,
		&row.RequiredG, &row.RequiredUnits, &row.ReservedG, &row.ReservedUnits,
		&row.ConsumedG, &row.ConsumedUnits, &row.ReturnedG, &row.ReturnedUnits,
		&row.RemainingReservedG, &row.Status, &row.WIPG, &row.AvailableG, &row.UpdatedAt,
	)
	if err != nil {
		return productionapp.WIPReservationRow{}, err
	}
	return row, rows.Err()
}
