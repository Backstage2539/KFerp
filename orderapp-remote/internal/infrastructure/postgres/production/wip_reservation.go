package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	productiondomain "orderapp/internal/domain/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"

	"github.com/jackc/pgx/v5"
)

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
