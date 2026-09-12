package production

import (
	"context"
	"fmt"
	app "orderapp/internal/application/production"
	"strings"
)

func (r Repository) scheduleTaskBoard(ctx context.Context, q app.ScheduleBoardQuery) (app.ScheduleBoardResult, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.Limit < 1 || q.Limit > 100 {
		q.Limit = 20
	}
	args := []any{}
	where := []string{"1=1"}
	add := func(sql string, v any) { args = append(args, v); where = append(where, fmt.Sprintf(sql, len(args))) }
	history := `(jc.status IN ('completed','cancelled') OR wo.status IN ('completed','cancelled'))`
	running := `jc.status IN ('running','paused','in_progress')`
	scheduled := `(jc.assigned_employee_id>0 AND jc.planned_start_at IS NOT NULL AND jc.planned_end_at IS NOT NULL AND COALESCE(NULLIF(jc.workstation,''),jc.work_center,'')<>'')`
	switch q.Scope {
	case "task":
		if q.JobCardID <= 0 && q.WorkOrderID <= 0 {
			return app.ScheduleBoardResult{}, fmt.Errorf("请选择指定任务")
		}
	case "history":
		where = append(where, history)
	case "running":
		where = append(where, "NOT "+history, running)
	case "scheduled":
		where = append(where, "NOT "+history, "NOT ("+running+")", scheduled)
	default:
		where = append(where, "NOT "+history, "NOT ("+running+")", "NOT "+scheduled)
	}
	if q.JobCardID > 0 {
		add("jc.id=$%d", q.JobCardID)
	}
	if q.WorkOrderID > 0 {
		add("wo.id=$%d", q.WorkOrderID)
	}
	if q.WorkCenter != "" {
		add("COALESCE(NULLIF(jc.workstation,''),jc.work_center)=$%d", q.WorkCenter)
	}
	if q.OperationID > 0 {
		add("jc.operation_id=$%d", q.OperationID)
	}
	if q.EmployeeID > 0 {
		args = append(args, q.EmployeeID)
		n := len(args)
		where = append(where, fmt.Sprintf(`(jc.assigned_employee_id=$%d OR EXISTS(SELECT 1 FROM %s.job_card_collaborators c WHERE c.job_card_id=jc.id AND c.employee_id=$%d))`, n, r.schema, n))
	}
	if q.Search != "" {
		add("concat_ws(' ',wo.work_order_no,wo.product_name,wo.order_nos,pp.plan_no,jc.operation) ILIKE $%d", "%"+q.Search+"%")
	}
	// Unscheduled demand stays visible until it has an actual time. Dated rows intersect the range.
	if q.Scope != "task" {
		add("(jc.planned_start_at IS NULL OR jc.planned_start_at < ($%d::date + 1)::timestamp AT TIME ZONE 'Asia/Shanghai')", q.To)
		add("(jc.planned_end_at IS NULL OR jc.planned_end_at > $%d::date::timestamp AT TIME ZONE 'Asia/Shanghai')", q.From)
	}
	base := fmt.Sprintf(` FROM %[1]s.job_cards jc JOIN %[1]s.work_orders wo ON wo.id=jc.work_order_id LEFT JOIN %[1]s.production_plans pp ON pp.id=wo.production_plan_id WHERE `, r.schema) + strings.Join(where, " AND ")
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT count(*)"+base, args...).Scan(&total); err != nil {
		return app.ScheduleBoardResult{}, err
	}
	pages := (total + q.Limit - 1) / q.Limit
	if pages < 1 {
		pages = 1
	}
	if q.Page > pages {
		q.Page = pages
	}
	args = append(args, q.Limit, (q.Page-1)*q.Limit)
	suffix := " WHERE " + strings.Join(where, " AND ") + fmt.Sprintf(" ORDER BY jc.planned_start_at NULLS FIRST,jc.priority DESC,wo.id DESC,jc.sequence_no,jc.id LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	tasks, err := loadScheduleTasks(ctx, r.pool, r.schema, suffix, args...)
	if err != nil {
		return app.ScheduleBoardResult{}, err
	}
	load, err := r.scheduleWorkstationLoad(ctx, q.From, q.To)
	if err != nil {
		return app.ScheduleBoardResult{}, err
	}
	return app.ScheduleBoardResult{Rows: tasks, Total: total, Page: q.Page, Limit: q.Limit, TotalPages: pages, Load: load, WorkOrders: []app.WorkOrderRow{}, JobCards: []app.JobCardRow{}, Capacity: []app.CapacityCalendarRow{}, Conflicts: []app.ScheduleConflict{}}, nil
}

func (r Repository) scheduleWorkstationLoad(ctx context.Context, from, to string) ([]app.ScheduleLoad, error) {
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
 WITH days AS (SELECT generate_series($1::date,$2::date,'1 day')::date AS day), loads AS (
 SELECT COALESCE(NULLIF(jc.workstation,''),jc.work_center) AS center,d.day,
 sum(EXTRACT(epoch FROM LEAST(jc.planned_end_at,(d.day+1)::timestamp AT TIME ZONE 'Asia/Shanghai')-GREATEST(jc.planned_start_at,d.day::timestamp AT TIME ZONE 'Asia/Shanghai'))/60)::int AS minutes
 FROM %[1]s.job_cards jc JOIN %[1]s.work_orders wo ON wo.id=jc.work_order_id JOIN days d ON jc.planned_start_at<(d.day+1)::timestamp AT TIME ZONE 'Asia/Shanghai' AND jc.planned_end_at>d.day::timestamp AT TIME ZONE 'Asia/Shanghai'
 WHERE jc.status NOT IN ('completed','cancelled') AND wo.status NOT IN ('completed','cancelled')
 GROUP BY center,d.day)
 SELECT l.center,to_char(l.day,'YYYY-MM-DD'),l.minutes,c.minutes
 FROM loads l LEFT JOIN (SELECT work_center,work_date,sum(GREATEST(available_minutes-downtime_minutes,0))::int AS minutes FROM %[1]s.work_center_capacity_calendar GROUP BY work_center,work_date) c ON c.work_center=l.center AND c.work_date=l.day
 ORDER BY l.day,l.center`, r.schema), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.ScheduleLoad{}
	for rows.Next() {
		var row app.ScheduleLoad
		if err := rows.Scan(&row.WorkCenter, &row.WorkDate, &row.LoadMinutes, &row.AvailableMinutes); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (r Repository) attachJobCardStaff(ctx context.Context, cards []app.JobCardRow) error {
	if len(cards) == 0 {
		return nil
	}
	ids := []int64{}
	index := map[int64]int{}
	for i := range cards {
		ids = append(ids, cards[i].ID)
		index[cards[i].ID] = i
	}
	rows, err := loadScheduleTasks(ctx, r.pool, r.schema, " WHERE jc.id=ANY($1)", ids)
	if err != nil {
		return err
	}
	for _, row := range rows {
		card := &cards[index[row.ID]]
		card.AssignedEmployeeID = row.AssignedEmployeeID
		card.CollaboratorEmployeeIDs = row.CollaboratorEmployeeIDs
		card.Collaborators = row.Collaborators
		card.ScheduleVersion = row.ScheduleVersion
		card.PlannedStartAt = row.PlannedStartAt
		card.PlannedEndAt = row.PlannedEndAt
	}
	return nil
}
