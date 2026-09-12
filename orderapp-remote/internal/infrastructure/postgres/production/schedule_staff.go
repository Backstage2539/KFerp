package production

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v5"
	manufacturingapp "orderapp/internal/application/manufacturing"
	app "orderapp/internal/application/production"
	infra "orderapp/internal/infrastructure/postgres"
	manufacturingrepo "orderapp/internal/infrastructure/postgres/manufacturing"
	"sort"
	"strings"
)

// The common query supplies IDs and frozen product context to every scheduling surface.
func scheduleTaskSQL(schema string) string {
	return fmt.Sprintf(`
 SELECT (to_jsonb(jc) - 'metrics_json' - 'parameter_schema_json') || jsonb_build_object(
 'metrics_json',COALESCE(jc.metrics_json,'{}'::jsonb)::text,'parameter_schema_json',COALESCE(jc.parameter_schema_json,'{}'::jsonb)::text,
 'work_order_no',wo.work_order_no,'product_name',wo.product_name,'product_id',wo.product_id,'spec_g',wo.spec_g,
 'order_nos',wo.order_nos,'planned_g',wo.planned_g,'planned_output_g',COALESCE(NULLIF(wo.planned_output_g,0),wo.planned_g),
 'production_plan_id',wo.production_plan_id,'production_plan_no',COALESCE(pp.plan_no,''),'work_order_status',wo.status,
 'inventory_unit',COALESCE(wo.inventory_unit,''),'planned_units',wo.sales_spec_count::bigint,
 'planned_output_inventory_qty',COALESCE(wo.planned_inventory_qty,0),
 'planned_start_at',COALESCE(to_char(jc.planned_start_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),''),
 'planned_end_at',COALESCE(to_char(jc.planned_end_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM-DD HH24:MI'),''),
 'collaborator_employee_ids','[]'::jsonb,
 'collaborators','[]'::jsonb,
 'eligible_employees',COALESCE((SELECT jsonb_agg(jsonb_build_object('id',e.id,'name',e.name,'active',e.active) ORDER BY e.name,e.id) FROM %[1]s.manufacturing_operation_employees oe JOIN %[1]s.company_employees e ON e.id=oe.employee_id WHERE oe.operation_id=jc.operation_id AND e.active=true),'[]'::jsonb),
 'default_employee_id',COALESCE((SELECT oe.employee_id FROM %[1]s.manufacturing_operation_employees oe JOIN %[1]s.company_employees e ON e.id=oe.employee_id AND e.active=true WHERE oe.operation_id=jc.operation_id AND oe.default_role='lead'),0),
 'default_collaborator_ids','[]'::jsonb,
 'batch_index',(SELECT count(*) FROM %[1]s.job_cards b WHERE b.work_order_id=jc.work_order_id AND b.sequence_no=jc.sequence_no AND b.id<=jc.id),
 'batch_count',(SELECT count(*) FROM %[1]s.job_cards b WHERE b.work_order_id=jc.work_order_id AND b.sequence_no=jc.sequence_no)
 )
 FROM %[1]s.job_cards jc JOIN %[1]s.work_orders wo ON wo.id=jc.work_order_id
 LEFT JOIN %[1]s.production_plans pp ON pp.id=wo.production_plan_id
 `, schema)
}

type scheduleQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadScheduleTasks(ctx context.Context, q scheduleQuerier, schema, suffix string, args ...any) ([]app.ScheduleTask, error) {
	rows, err := q.Query(ctx, scheduleTaskSQL(schema)+suffix, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []app.ScheduleTask{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		var row app.ScheduleTask
		if err := json.Unmarshal(data, &row); err != nil {
			return nil, err
		}
		row.StaffingReady = row.DefaultEmployeeID > 0
		row.ArrangementState = scheduleArrangementState(row)
		out = append(out, row)
	}
	return out, rows.Err()
}
func scheduleArrangementState(row app.ScheduleTask) string {
	if row.Status == "completed" || row.Status == "cancelled" || row.WorkOrderStatus == "completed" || row.WorkOrderStatus == "cancelled" {
		return "history"
	}
	if row.Status == "running" || row.Status == "in_progress" || row.Status == "paused" {
		return "running"
	}
	if row.AssignedEmployeeID > 0 && row.PlannedStartAt != "" && row.PlannedEndAt != "" && (row.Workstation != "" || row.WorkCenter != "") {
		return "scheduled"
	}
	return "pending"
}
func (r Repository) ScheduleOptions(ctx context.Context) (app.ScheduleOptions, error) {
	repo := manufacturingrepo.NewRepository(r.pool, r.schema)
	ops, err := repo.ListManufacturingOperations(ctx)
	if err != nil {
		return app.ScheduleOptions{}, err
	}
	caps, err := repo.ListManufacturingWorkstationCapacities(ctx, manufacturingapp.WorkstationCapacityQuery{Status: "active"})
	if err != nil {
		return app.ScheduleOptions{}, err
	}
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`SELECT id,COALESCE(name,''),active FROM %s.company_employees WHERE active=true ORDER BY name,id`, r.schema))
	if err != nil {
		return app.ScheduleOptions{}, err
	}
	defer rows.Close()
	staff := []app.ScheduleEmployee{}
	for rows.Next() {
		var e app.ScheduleEmployee
		if err := rows.Scan(&e.ID, &e.Name, &e.Active); err != nil {
			return app.ScheduleOptions{}, err
		}
		staff = append(staff, e)
	}
	return app.ScheduleOptions{Operations: ops, Capacities: caps, Employees: staff}, rows.Err()
}
func scheduleHash(value any) string {
	data, _ := json.Marshal(value)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (r Repository) ScheduleBatch(ctx context.Context, cmd app.ScheduleBatchCommand, preview bool) (app.ScheduleBatchResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ScheduleBatchResult{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":production_schedule"); err != nil {
		return app.ScheduleBatchResult{}, err
	}
	payloadHash := scheduleHash(cmd.Items)
	if !preview && cmd.RequestID != "" {
		var hash string
		var data []byte
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT payload_hash,response_json FROM %s.production_schedule_requests WHERE actor=$1 AND request_id=$2`, r.schema), cmd.Operator, cmd.RequestID).Scan(&hash, &data)
		if err == nil {
			if hash != payloadHash {
				return app.ScheduleBatchResult{}, fmt.Errorf("请求编号已用于另一批安排，请重新预览")
			}
			var result app.ScheduleBatchResult
			if err = json.Unmarshal(data, &result); err != nil {
				return result, err
			}
			result.Replayed = true
			return result, nil
		}
		if err != pgx.ErrNoRows {
			return app.ScheduleBatchResult{}, err
		}
	}
	items := append([]app.ScheduleTaskPatch{}, cmd.Items...)
	sort.Slice(items, func(i, j int) bool { return items[i].JobCardID < items[j].JobCardID })
	changed := []app.ScheduleTask{}
	originals := map[int64]app.ScheduleTask{}
	ids := []int64{}
	for _, patch := range items {
		rows, err := loadScheduleTasks(ctx, tx, r.schema, " WHERE jc.id=$1 AND jc.work_order_id=$2 FOR UPDATE OF jc,wo", patch.JobCardID, patch.WorkOrderID)
		if err != nil {
			return app.ScheduleBatchResult{}, err
		}
		if len(rows) != 1 {
			return app.ScheduleBatchResult{}, fmt.Errorf("工序任务不存在，请刷新")
		}
		original := rows[0]
		if patch.AssignedEmployeeID == nil && patch.AssignedTo != nil && strings.TrimSpace(*patch.AssignedTo) != "" {
			matches := []int64{}
			for _, e := range original.EligibleEmployees {
				if e.Name == strings.TrimSpace(*patch.AssignedTo) {
					matches = append(matches, e.ID)
				}
			}
			if len(matches) != 1 {
				return app.ScheduleBatchResult{}, fmt.Errorf("历史姓名无法明确关联可执行员工，请通过人员选择器重新选择")
			}
			patch.AssignedEmployeeID = &matches[0]
		}
		next, err := app.MergeSchedulePatch(original, patch)
		if err != nil {
			return app.ScheduleBatchResult{}, err
		}
		if patch.WorkCenter != nil && *patch.WorkCenter != "" && *patch.WorkCenter != original.Workstation && *patch.WorkCenter != original.WorkCenter {
			return app.ScheduleBatchResult{}, fmt.Errorf("排程沿用已确认工位；未分配任务请选择适配产能档")
		}
		if patch.WorkstationCapacityID != nil && *patch.WorkstationCapacityID > 0 {
			if err := fillMissingScheduleWorkstation(ctx, tx, r.schema, &next, *patch.WorkstationCapacityID); err != nil {
				return app.ScheduleBatchResult{}, err
			}
		}
		personnelChanged := patch.AssignedEmployeeID != nil || patch.CollaboratorEmployeeIDs != nil
		if personnelChanged || scheduleArrangementState(original) != "running" {
			if err := validateScheduleStaff(ctx, tx, r.schema, &next); err != nil {
				return app.ScheduleBatchResult{}, err
			}
		}
		if patch.Claim && original.AssignedTo != "" {
			return app.ScheduleBatchResult{}, fmt.Errorf("任务已有负责人，请刷新后查看")
		}
		changed = append(changed, next)
		ids = append(ids, next.ID)
		originals[next.ID] = original
	}
	others, err := loadScheduleTasks(ctx, tx, r.schema, ` WHERE jc.id<>ALL($1) AND jc.status NOT IN ('completed','cancelled') AND wo.status NOT IN ('completed','cancelled') AND jc.planned_start_at IS NOT NULL AND jc.planned_end_at IS NOT NULL`, ids)
	if err != nil {
		return app.ScheduleBatchResult{}, err
	}
	conflicts := app.StaffScheduleConflicts(changed, append(others, changed...))
	result := app.ScheduleBatchResult{Rows: changed, Conflicts: conflicts}
	result.PreviewToken = scheduleHash(struct {
		Items     []app.ScheduleTaskPatch
		Rows      []app.ScheduleTask
		Conflicts []app.StaffScheduleConflict
	}{items, changed, conflicts})
	if preview {
		return result, nil
	}
	if len(conflicts) > 0 && cmd.PreviewToken != result.PreviewToken {
		return result, &app.ScheduleError{Code: "confirmation_required", Message: "人员安排存在时间重叠或冲突已变化，请重新预览并确认", Conflicts: conflicts}
	}
	for i := range changed {
		row := &changed[i]
		old := originals[row.ID]
		_, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET assigned_employee_id=$2,assigned_to=$3,assigned_employee_name=$3,
   planned_start_at=NULLIF($4,'')::timestamp AT TIME ZONE 'Asia/Shanghai',planned_end_at=NULLIF($5,'')::timestamp AT TIME ZONE 'Asia/Shanghai',
   shift_code=$6,priority=$7,scheduling_note=$8,workstation_id=$9,workstation=$10,work_center=$11,
   workstation_capacity_id=$12,workstation_capacity_name=$13,schedule_version=schedule_version+1 WHERE id=$1`, r.schema), row.ID, row.AssignedEmployeeID, row.AssignedTo, row.PlannedStartAt, row.PlannedEndAt, row.ShiftCode, row.Priority, row.SchedulingNote, row.WorkstationID, row.Workstation, row.WorkCenter, row.WorkstationCapacityID, row.WorkstationCapacityName)
		if err != nil {
			return result, err
		}
		// Legacy collaborator rows are intentionally left untouched as history.
		before, _ := json.Marshal(old)
		after, _ := json.Marshal(row)
		if err = infra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "job_card", &row.ID, "schedule", infra.StrPtr("assignment"), infra.StrPtr(string(before)), infra.StrPtr(string(after)), infra.AuditMeta{"work_order_id": row.WorkOrderID, "request_id": cmd.RequestID, "overlap_confirmed": len(conflicts) > 0}); err != nil {
			return result, err
		}
		row.ScheduleVersion++
		row.ArrangementState = scheduleArrangementState(*row)
	}
	for _, row := range changed {
		summary, err := operationSummaryJSONForWorkOrderTx(ctx, tx, r.schema, row.WorkOrderID)
		if err != nil {
			return result, err
		}
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.work_orders SET operation_summary_json=$2::jsonb WHERE id=$1`, r.schema), row.WorkOrderID, summary); err != nil {
			return result, err
		}
	}
	result.Rows = changed
	result.Saved = true
	if cmd.RequestID != "" {
		data, _ := json.Marshal(result)
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_schedule_requests(actor,request_id,payload_hash,response_json) VALUES($1,$2,$3,$4)`, r.schema), cmd.Operator, cmd.RequestID, payloadHash, data); err != nil {
			return result, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func validateScheduleStaff(ctx context.Context, tx pgx.Tx, schema string, row *app.ScheduleTask) error {
	if row.OperationID <= 0 {
		return fmt.Errorf("历史任务尚未关联工序，请先补齐工序信息")
	}
	ids := []int64{row.AssignedEmployeeID}
	row.Collaborators = []app.ScheduleEmployee{}
	row.CollaboratorEmployeeIDs = []int64{}
	for _, id := range ids {
		if id <= 0 {
			return fmt.Errorf("请选择工序负责人")
		}
		var e app.ScheduleEmployee
		err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT e.id,e.name,e.active FROM %[1]s.company_employees e JOIN %[1]s.manufacturing_operation_employees oe ON oe.employee_id=e.id JOIN %[1]s.manufacturing_operations o ON o.id=oe.operation_id WHERE e.id=$1 AND oe.operation_id=$2 AND e.active=true AND o.status='active' FOR SHARE OF e,o`, schema), id, row.OperationID).Scan(&e.ID, &e.Name, &e.Active)
		if err == pgx.ErrNoRows {
			return fmt.Errorf("所选员工已停用或不属于本工序的可执行员工，请先补齐工序人员配置")
		}
		if err != nil {
			return err
		}
		row.AssignedTo = e.Name
	}
	return nil
}
func fillMissingScheduleWorkstation(ctx context.Context, tx pgx.Tx, schema string, row *app.ScheduleTask, capacityID int64) error {
	if row.WorkstationID > 0 || row.WorkstationCapacityID > 0 {
		if row.WorkstationCapacityID == capacityID {
			return nil
		}
		return fmt.Errorf("排程沿用生产计划已确认的工位和批次")
	}
	var stationID int64
	var station, name, unit string
	var qty float64
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT c.workstation_id,w.name,c.name,c.batch_size_qty,c.batch_size_unit FROM %[1]s.manufacturing_workstation_capacities c JOIN %[1]s.manufacturing_workstations w ON w.id=c.workstation_id JOIN %[1]s.manufacturing_workstation_operations o ON o.workstation_id=w.id AND o.operation_id=$2 WHERE c.id=$1 AND c.status='active' AND w.status='active' FOR SHARE OF c,w`, schema), capacityID, row.OperationID).Scan(&stationID, &station, &name, &qty, &unit)
	if err == pgx.ErrNoRows {
		return fmt.Errorf("请选择适用于该工序的启用工位产能档")
	}
	if err != nil {
		return err
	}
	taskQty := row.PlannedInputQty
	taskUnit := row.InventoryUnit
	if row.PlannedOutputInventoryQty > 0 && row.PlannedG > 0 {
		taskQty = row.PlannedInputQty / float64(row.PlannedG) * row.PlannedOutputInventoryQty
	} else if taskUnit == "kg" {
		taskQty /= 1000
	} else if taskUnit == "" {
		taskUnit = "g"
	}
	if taskQty <= 0 || qty <= 0 || unit != taskUnit || taskQty > qty {
		return fmt.Errorf("当前批次与所选产能档的单位或容量不匹配，需重新规划批次")
	}
	row.WorkstationID = stationID
	row.Workstation = station
	row.WorkCenter = station
	row.WorkstationCapacityID = capacityID
	row.WorkstationCapacityName = name
	return nil
}
