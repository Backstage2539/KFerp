package production

import (
	"context"
	"fmt"
	productionapp "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (r Repository) SaveScheduleAssignment(ctx context.Context, cmd productionapp.ScheduleAssignmentCommand) (productionapp.ScheduleAssignmentResult, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.ScheduleAssignmentResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, fmt.Sprintf(`
		UPDATE %s.work_orders
		SET planned_start_at=NULLIF($2,'')::timestamptz,
		    planned_end_at=NULLIF($3,'')::timestamptz,
		    shift_code=$4,
		    assigned_to=$5,
		    priority=$6,
		    scheduling_note=$7,
		    work_center=$8
		WHERE id=$1
	`, r.schema), cmd.WorkOrderID, cmd.PlannedStartAt, cmd.PlannedEndAt, cmd.ShiftCode, cmd.AssignedTo, cmd.Priority, cmd.Note, cmd.WorkCenter)
	if err != nil {
		return productionapp.ScheduleAssignmentResult{}, err
	}
	if tag.RowsAffected() == 0 {
		return productionapp.ScheduleAssignmentResult{}, fmt.Errorf("work order not found")
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "work_order", &cmd.WorkOrderID, "schedule", postgresinfra.StrPtr("planned_start_at"), nil, postgresinfra.StrPtr(cmd.PlannedStartAt), postgresinfra.AuditMeta{"planned_end_at": cmd.PlannedEndAt, "shift_code": cmd.ShiftCode, "assigned_to": cmd.AssignedTo, "priority": cmd.Priority, "work_center": cmd.WorkCenter}); err != nil {
		return productionapp.ScheduleAssignmentResult{}, err
	}

	var card productionapp.JobCardRow
	if cmd.JobCardID > 0 {
		tag, err = tx.Exec(ctx, fmt.Sprintf(`
			UPDATE %s.job_cards
			SET planned_start_at=NULLIF($3,'')::timestamptz,
			    planned_end_at=NULLIF($4,'')::timestamptz,
			    shift_code=$5,
			    assigned_to=$6,
			    priority=$7,
			    scheduling_note=$8,
			    work_center=$9,
			    workstation=COALESCE(NULLIF($9,''), workstation)
			WHERE id=$1 AND work_order_id=$2
		`, r.schema), cmd.JobCardID, cmd.WorkOrderID, cmd.PlannedStartAt, cmd.PlannedEndAt, cmd.ShiftCode, cmd.AssignedTo, cmd.Priority, cmd.Note, cmd.WorkCenter)
		if err != nil {
			return productionapp.ScheduleAssignmentResult{}, err
		}
		if tag.RowsAffected() == 0 {
			return productionapp.ScheduleAssignmentResult{}, fmt.Errorf("job card not found")
		}
		if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "job_card", &cmd.JobCardID, "schedule", postgresinfra.StrPtr("planned_start_at"), nil, postgresinfra.StrPtr(cmd.PlannedStartAt), postgresinfra.AuditMeta{"work_order_id": cmd.WorkOrderID, "planned_end_at": cmd.PlannedEndAt, "shift_code": cmd.ShiftCode, "assigned_to": cmd.AssignedTo, "priority": cmd.Priority, "work_center": cmd.WorkCenter}); err != nil {
			return productionapp.ScheduleAssignmentResult{}, err
		}
		card, err = loadScheduledJobCardTx(ctx, tx, r.schema, cmd.JobCardID)
		if err != nil {
			return productionapp.ScheduleAssignmentResult{}, err
		}
	}
	wo, err := loadScheduledWorkOrderTx(ctx, tx, r.schema, cmd.WorkOrderID)
	if err != nil {
		return productionapp.ScheduleAssignmentResult{}, err
	}
	conflicts, err := scheduleConflictsTx(ctx, tx, r.schema, scheduleDateFromTimestamp(cmd.PlannedStartAt), scheduleDateFromTimestamp(cmd.PlannedStartAt), cmd.WorkCenter)
	if err != nil {
		return productionapp.ScheduleAssignmentResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.ScheduleAssignmentResult{}, err
	}
	return productionapp.ScheduleAssignmentResult{WorkOrder: wo, JobCard: card, Conflicts: conflicts}, nil
}

func (r Repository) SaveCapacityCalendar(ctx context.Context, cmd productionapp.CapacityCalendarCommand) (productionapp.CapacityCalendarRow, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return productionapp.CapacityCalendarRow{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var id int64
	if err := tx.QueryRow(ctx, fmt.Sprintf(`
		INSERT INTO %s.work_center_capacity_calendar(work_center,work_date,shift_code,available_minutes,downtime_minutes,note,updated_by,updated_at)
		VALUES($1,$2::date,$3,$4,$5,$6,$7,now())
		ON CONFLICT (work_center, work_date, shift_code) DO UPDATE SET
			available_minutes=excluded.available_minutes,
			downtime_minutes=excluded.downtime_minutes,
			note=excluded.note,
			updated_by=excluded.updated_by,
			updated_at=now()
		RETURNING id
	`, r.schema), cmd.WorkCenter, cmd.WorkDate, cmd.ShiftCode, cmd.AvailableMinutes, cmd.DowntimeMinutes, cmd.Note, cmd.Operator).Scan(&id); err != nil {
		return productionapp.CapacityCalendarRow{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "work_center_capacity_calendar", &id, "upsert", postgresinfra.StrPtr("available_minutes"), nil, postgresinfra.StrPtr(fmt.Sprintf("%d", cmd.AvailableMinutes)), postgresinfra.AuditMeta{"work_center": cmd.WorkCenter, "work_date": cmd.WorkDate, "shift_code": cmd.ShiftCode, "downtime_minutes": cmd.DowntimeMinutes}); err != nil {
		return productionapp.CapacityCalendarRow{}, err
	}
	row, err := loadCapacityCalendarRowTx(ctx, tx, r.schema, id)
	if err != nil {
		return productionapp.CapacityCalendarRow{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return productionapp.CapacityCalendarRow{}, err
	}
	return row, nil
}

func (r Repository) ScheduleBoard(ctx context.Context, query productionapp.ScheduleBoardQuery) (productionapp.ScheduleBoardResult, error) {
	workOrders, err := r.listScheduledWorkOrders(ctx, query)
	if err != nil {
		return productionapp.ScheduleBoardResult{}, err
	}
	jobCards, err := r.listScheduledJobCards(ctx, query)
	if err != nil {
		return productionapp.ScheduleBoardResult{}, err
	}
	capacity, err := r.listCapacityCalendar(ctx, query)
	if err != nil {
		return productionapp.ScheduleBoardResult{}, err
	}
	conflicts := scheduleConflictsFromRows(workOrders, jobCards, capacity)
	return productionapp.ScheduleBoardResult{WorkOrders: workOrders, JobCards: jobCards, Capacity: capacity, Conflicts: conflicts}, nil
}

func (r Repository) listScheduledWorkOrders(ctx context.Context, query productionapp.ScheduleBoardQuery) ([]productionapp.WorkOrderRow, error) {
	args := []any{query.From, query.To}
	where := `(wo.planned_start_at IS NULL OR (wo.planned_start_at::date >= $1::date AND wo.planned_start_at::date <= $2::date))`
	if query.WorkCenter != "" {
		args = append(args, query.WorkCenter)
		where += fmt.Sprintf(" AND COALESCE(wo.work_center,'')=$%d", len(args))
	}
	if query.Status != "" {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND wo.status=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,product_id,product_name,spec_g,planned_g,COALESCE(NULLIF(planned_output_g,0),planned_g),status,
		       COALESCE(actual_cost,0)::float8,to_char(created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(planned_start_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(planned_end_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(shift_code,''),COALESCE(assigned_to,''),COALESCE(priority,0),COALESCE(scheduling_note,''),COALESCE(work_center,'')
		FROM %s.work_orders wo
		WHERE %s
		ORDER BY COALESCE(wo.planned_start_at, wo.created_at), wo.priority DESC, wo.id DESC
		LIMIT $%d
	`, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.WorkOrderRow, 0)
	for rows.Next() {
		var row productionapp.WorkOrderRow
		if err := rows.Scan(&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.PlannedOutputG, &row.Status, &row.ActualCost, &row.CreatedAt, &row.CompletedAt, &row.PlannedStartAt, &row.PlannedEndAt, &row.ShiftCode, &row.AssignedTo, &row.Priority, &row.SchedulingNote, &row.WorkCenter); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listScheduledJobCards(ctx context.Context, query productionapp.ScheduleBoardQuery) ([]productionapp.JobCardRow, error) {
	args := []any{query.From, query.To}
	where := `(jc.planned_start_at IS NULL OR (jc.planned_start_at::date >= $1::date AND jc.planned_start_at::date <= $2::date))`
	if query.WorkCenter != "" {
		args = append(args, query.WorkCenter)
		where += fmt.Sprintf(" AND COALESCE(NULLIF(jc.work_center,''), jc.workstation, '')=$%d", len(args))
	}
	if query.Status != "" {
		args = append(args, query.Status)
		where += fmt.Sprintf(" AND jc.status=$%d", len(args))
	}
	args = append(args, query.Limit)
	limitArg := len(args)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,work_order_id,sequence_no,operation,workstation,status,
		       COALESCE(to_char(started_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(paused_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(resumed_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),operator,
		       COALESCE(planned_input_qty,0)::float8,COALESCE(actual_input_qty,0)::float8,COALESCE(actual_output_qty,0)::float8,COALESCE(actual_loss_qty,0)::float8,COALESCE(actual_loss_rate,0)::float8,
		       COALESCE(records_loss,false),COALESCE(loss_reason,''),COALESCE(exception_reason,''),COALESCE(metrics_json,'{}'::jsonb)::text,COALESCE(parameter_schema_json,'{}'::jsonb)::text,
		       COALESCE(to_char(planned_start_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(planned_end_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(shift_code,''),COALESCE(assigned_to,''),COALESCE(priority,0),COALESCE(scheduling_note,''),COALESCE(work_center,'')
		FROM %s.job_cards jc
		WHERE %s
		ORDER BY COALESCE(jc.planned_start_at, jc.started_at), jc.priority DESC, jc.work_order_id DESC, jc.sequence_no, jc.id
		LIMIT $%d
	`, r.schema, where, limitArg), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.JobCardRow, 0)
	for rows.Next() {
		var row productionapp.JobCardRow
		if err := rows.Scan(&row.ID, &row.WorkOrderID, &row.SequenceNo, &row.Operation, &row.Workstation, &row.Status, &row.StartedAt, &row.PausedAt, &row.ResumedAt, &row.CompletedAt, &row.Operator, &row.PlannedInputQty, &row.ActualInputQty, &row.ActualOutputQty, &row.ActualLossQty, &row.ActualLossRate, &row.RecordsLoss, &row.LossReason, &row.ExceptionReason, &row.MetricsJSON, &row.ParameterSchemaJSON, &row.PlannedStartAt, &row.PlannedEndAt, &row.ShiftCode, &row.AssignedTo, &row.Priority, &row.SchedulingNote, &row.WorkCenter); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) listCapacityCalendar(ctx context.Context, query productionapp.ScheduleBoardQuery) ([]productionapp.CapacityCalendarRow, error) {
	args := []any{query.From, query.To}
	where := "work_date >= $1::date AND work_date <= $2::date"
	if query.WorkCenter != "" {
		args = append(args, query.WorkCenter)
		where += fmt.Sprintf(" AND work_center=$%d", len(args))
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id,work_center,to_char(work_date,'YYYY-MM-DD'),shift_code,available_minutes,downtime_minutes,note,to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.work_center_capacity_calendar
		WHERE %s
		ORDER BY work_date, work_center, shift_code, id
	`, r.schema, where), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.CapacityCalendarRow, 0)
	for rows.Next() {
		var row productionapp.CapacityCalendarRow
		if err := rows.Scan(&row.ID, &row.WorkCenter, &row.WorkDate, &row.ShiftCode, &row.AvailableMinutes, &row.DowntimeMinutes, &row.Note, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func loadCapacityCalendarRowTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.CapacityCalendarRow, error) {
	var row productionapp.CapacityCalendarRow
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_center,to_char(work_date,'YYYY-MM-DD'),shift_code,available_minutes,downtime_minutes,note,to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.work_center_capacity_calendar
		WHERE id=$1
	`, schema), id).Scan(&row.ID, &row.WorkCenter, &row.WorkDate, &row.ShiftCode, &row.AvailableMinutes, &row.DowntimeMinutes, &row.Note, &row.UpdatedAt)
	return row, err
}

func loadScheduledWorkOrderTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.WorkOrderRow, error) {
	var row productionapp.WorkOrderRow
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_order_no,running_item_id,production_plan_id,production_plan_item_id,batch_id,product_id,product_name,spec_g,planned_g,COALESCE(NULLIF(planned_output_g,0),planned_g),status,
		       COALESCE(actual_cost,0)::float8,to_char(created_at,'YYYY-MM-DD HH24:MI'),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(planned_start_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(planned_end_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(shift_code,''),COALESCE(assigned_to,''),COALESCE(priority,0),COALESCE(scheduling_note,''),COALESCE(work_center,'')
		FROM %s.work_orders
		WHERE id=$1
	`, schema), id).Scan(&row.ID, &row.WorkOrderNo, &row.RunningItemID, &row.ProductionPlanID, &row.ProductionPlanItemID, &row.BatchID, &row.ProductID, &row.ProductName, &row.SpecG, &row.PlannedG, &row.PlannedOutputG, &row.Status, &row.ActualCost, &row.CreatedAt, &row.CompletedAt, &row.PlannedStartAt, &row.PlannedEndAt, &row.ShiftCode, &row.AssignedTo, &row.Priority, &row.SchedulingNote, &row.WorkCenter)
	if err == pgx.ErrNoRows {
		return productionapp.WorkOrderRow{}, fmt.Errorf("work order not found")
	}
	return row, err
}

func loadScheduledJobCardTx(ctx context.Context, tx pgx.Tx, schema string, id int64) (productionapp.JobCardRow, error) {
	var row productionapp.JobCardRow
	err := tx.QueryRow(ctx, fmt.Sprintf(`
		SELECT id,work_order_id,sequence_no,operation,workstation,status,
		       COALESCE(to_char(started_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(paused_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(resumed_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(completed_at,'YYYY-MM-DD HH24:MI'),''),operator,
		       COALESCE(planned_input_qty,0)::float8,COALESCE(actual_input_qty,0)::float8,COALESCE(actual_output_qty,0)::float8,COALESCE(actual_loss_qty,0)::float8,COALESCE(actual_loss_rate,0)::float8,
		       COALESCE(records_loss,false),COALESCE(loss_reason,''),COALESCE(exception_reason,''),COALESCE(metrics_json,'{}'::jsonb)::text,COALESCE(parameter_schema_json,'{}'::jsonb)::text,
		       COALESCE(to_char(planned_start_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(to_char(planned_end_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(shift_code,''),COALESCE(assigned_to,''),COALESCE(priority,0),COALESCE(scheduling_note,''),COALESCE(work_center,'')
		FROM %s.job_cards
		WHERE id=$1
	`, schema), id).Scan(&row.ID, &row.WorkOrderID, &row.SequenceNo, &row.Operation, &row.Workstation, &row.Status, &row.StartedAt, &row.PausedAt, &row.ResumedAt, &row.CompletedAt, &row.Operator, &row.PlannedInputQty, &row.ActualInputQty, &row.ActualOutputQty, &row.ActualLossQty, &row.ActualLossRate, &row.RecordsLoss, &row.LossReason, &row.ExceptionReason, &row.MetricsJSON, &row.ParameterSchemaJSON, &row.PlannedStartAt, &row.PlannedEndAt, &row.ShiftCode, &row.AssignedTo, &row.Priority, &row.SchedulingNote, &row.WorkCenter)
	if err == pgx.ErrNoRows {
		return productionapp.JobCardRow{}, fmt.Errorf("job card not found")
	}
	return row, err
}

func scheduleConflictsTx(ctx context.Context, tx pgx.Tx, schema string, from string, to string, workCenter string) ([]productionapp.ScheduleConflict, error) {
	if from == "" || to == "" {
		return nil, nil
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,work_center,to_char(work_date,'YYYY-MM-DD'),shift_code,available_minutes,downtime_minutes,note,to_char(updated_at,'YYYY-MM-DD HH24:MI')
		FROM %s.work_center_capacity_calendar
		WHERE work_date >= $1::date AND work_date <= $2::date AND ($3='' OR work_center=$3)
	`, schema), from, to, workCenter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	capacity := make([]productionapp.CapacityCalendarRow, 0)
	for rows.Next() {
		var row productionapp.CapacityCalendarRow
		if err := rows.Scan(&row.ID, &row.WorkCenter, &row.WorkDate, &row.ShiftCode, &row.AvailableMinutes, &row.DowntimeMinutes, &row.Note, &row.UpdatedAt); err != nil {
			return nil, err
		}
		capacity = append(capacity, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	jobCards, err := scheduledJobCardLoadTx(ctx, tx, schema, from, to, workCenter)
	if err != nil {
		return nil, err
	}
	workOrders, err := scheduledWorkOrderLoadTx(ctx, tx, schema, from, to, workCenter)
	if err != nil {
		return nil, err
	}
	return scheduleConflictsFromRows(workOrders, jobCards, capacity), nil
}

func scheduledJobCardLoadTx(ctx context.Context, tx pgx.Tx, schema string, from string, to string, workCenter string) ([]productionapp.JobCardRow, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,work_order_id,COALESCE(workstation,''),
		       COALESCE(to_char(planned_start_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(planned_end_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(shift_code,''),COALESCE(work_center,'')
		FROM %s.job_cards
		WHERE planned_start_at IS NOT NULL
		  AND planned_start_at::date >= $1::date
		  AND planned_start_at::date <= $2::date
		  AND ($3='' OR COALESCE(NULLIF(work_center,''), workstation, '')=$3)
	`, schema), from, to, workCenter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.JobCardRow, 0)
	for rows.Next() {
		var row productionapp.JobCardRow
		if err := rows.Scan(&row.ID, &row.WorkOrderID, &row.Workstation, &row.PlannedStartAt, &row.PlannedEndAt, &row.ShiftCode, &row.WorkCenter); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func scheduledWorkOrderLoadTx(ctx context.Context, tx pgx.Tx, schema string, from string, to string, workCenter string) ([]productionapp.WorkOrderRow, error) {
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT id,COALESCE(work_order_no,''),COALESCE(to_char(planned_start_at,'YYYY-MM-DD HH24:MI'),''),
		       COALESCE(to_char(planned_end_at,'YYYY-MM-DD HH24:MI'),''),COALESCE(shift_code,''),COALESCE(work_center,'')
		FROM %s.work_orders
		WHERE planned_start_at IS NOT NULL
		  AND planned_start_at::date >= $1::date
		  AND planned_start_at::date <= $2::date
		  AND ($3='' OR COALESCE(work_center,'')=$3)
	`, schema), from, to, workCenter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]productionapp.WorkOrderRow, 0)
	for rows.Next() {
		var row productionapp.WorkOrderRow
		if err := rows.Scan(&row.ID, &row.WorkOrderNo, &row.PlannedStartAt, &row.PlannedEndAt, &row.ShiftCode, &row.WorkCenter); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func scheduleConflictsFromRows(workOrders []productionapp.WorkOrderRow, jobCards []productionapp.JobCardRow, capacity []productionapp.CapacityCalendarRow) []productionapp.ScheduleConflict {
	available := map[string]int{}
	for _, row := range capacity {
		key := scheduleBucketKey(row.WorkCenter, row.WorkDate, row.ShiftCode)
		available[key] = row.AvailableMinutes - row.DowntimeMinutes
	}
	load := map[string]int{}
	for _, row := range jobCards {
		if row.PlannedStartAt == "" || row.PlannedEndAt == "" {
			continue
		}
		center := strings.TrimSpace(row.WorkCenter)
		if center == "" {
			center = strings.TrimSpace(row.Workstation)
		}
		key := scheduleBucketKey(center, scheduleDateFromTimestamp(row.PlannedStartAt), row.ShiftCode)
		load[key] += scheduleDurationMinutes(row.PlannedStartAt, row.PlannedEndAt)
	}
	if len(jobCards) == 0 {
		for _, row := range workOrders {
			if row.PlannedStartAt == "" || row.PlannedEndAt == "" {
				continue
			}
			key := scheduleBucketKey(row.WorkCenter, scheduleDateFromTimestamp(row.PlannedStartAt), row.ShiftCode)
			load[key] += scheduleDurationMinutes(row.PlannedStartAt, row.PlannedEndAt)
		}
	}
	out := make([]productionapp.ScheduleConflict, 0)
	for key, minutes := range load {
		capacityMinutes, ok := available[key]
		if !ok || capacityMinutes <= 0 || minutes <= capacityMinutes {
			continue
		}
		center, date, shift := splitScheduleBucketKey(key)
		out = append(out, productionapp.ScheduleConflict{
			Severity:        "warning",
			WorkCenter:      center,
			WorkDate:        date,
			ShiftCode:       shift,
			LoadMinutes:     minutes,
			CapacityMinutes: capacityMinutes,
			Message:         fmt.Sprintf("%s %s %s 负载 %d 分钟超过可用产能 %d 分钟", date, center, shift, minutes, capacityMinutes),
		})
	}
	return out
}

func scheduleBucketKey(center string, date string, shift string) string {
	return strings.TrimSpace(center) + "\x00" + strings.TrimSpace(date) + "\x00" + strings.TrimSpace(shift)
}

func splitScheduleBucketKey(key string) (string, string, string) {
	parts := strings.Split(key, "\x00")
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}

func scheduleDateFromTimestamp(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 10 {
		return value[:10]
	}
	return ""
}

func scheduleDurationMinutes(start string, end string) int {
	startAt, err := time.Parse("2006-01-02 15:04", strings.TrimSpace(start))
	if err != nil {
		return 0
	}
	endAt, err := time.Parse("2006-01-02 15:04", strings.TrimSpace(end))
	if err != nil || !endAt.After(startAt) {
		return 0
	}
	return int(endAt.Sub(startAt).Minutes())
}
