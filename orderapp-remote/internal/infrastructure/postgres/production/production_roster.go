package production

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	app "orderapp/internal/application/production"
	postgresinfra "orderapp/internal/infrastructure/postgres"

	"github.com/jackc/pgx/v5"
)

func resolveProductionWorkstationOwnerTx(ctx context.Context, tx pgx.Tx, schema string, workstationID int64, workstation, workDate string) (app.WorkstationOwnerResolution, bool, error) {
	weekStart, _, err := app.NormalizeProductionRosterWeek(workDate, nil)
	if err != nil {
		return app.WorkstationOwnerResolution{}, false, err
	}
	var version int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT version FROM %s.production_roster_weeks WHERE week_start=$1::date`, schema), weekStart).Scan(&version)
	if err == pgx.ErrNoRows {
		return app.WorkstationOwnerResolution{}, false, nil
	}
	if err != nil {
		return app.WorkstationOwnerResolution{}, false, err
	}
	if workstationID <= 0 {
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id FROM %s.manufacturing_workstations WHERE status='active' AND TRIM(name)=TRIM($1) ORDER BY id LIMIT 1`, schema), workstation).Scan(&workstationID)
		if err == pgx.ErrNoRows {
			return app.WorkstationOwnerResolution{Unattended: true, Reason: "工位未配置人员"}, true, nil
		}
		if err != nil {
			return app.WorkstationOwnerResolution{}, true, err
		}
	}

	var stationActive bool
	if err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT status='active' FROM %s.manufacturing_workstations WHERE id=$1`, schema), workstationID).Scan(&stationActive); err != nil && err != pgx.ErrNoRows {
		return app.WorkstationOwnerResolution{}, true, err
	}
	if !stationActive {
		return app.WorkstationOwnerResolution{Unattended: true, Reason: "工位不存在或已停用"}, true, nil
	}
	attendance := map[int64]string{}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT employee_id,status FROM %s.production_employee_attendance WHERE work_date=$1::date`, schema), workDate)
	if err != nil {
		return app.WorkstationOwnerResolution{}, true, err
	}
	for rows.Next() {
		var employeeID int64
		var status string
		if err := rows.Scan(&employeeID, &status); err != nil {
			rows.Close()
			return app.WorkstationOwnerResolution{}, true, err
		}
		attendance[employeeID] = status
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return app.WorkstationOwnerResolution{}, true, err
	}
	rows.Close()
	staff := []app.WorkstationStaffCandidate{}
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT s.employee_id,COALESCE(e.name,''),s.staff_role,s.sort_order,COALESCE(e.active,false) AND COALESCE(e.account_type,'internal_employee')<>'channel_customer' FROM %s.manufacturing_workstation_employees s LEFT JOIN %s.company_employees e ON e.id=s.employee_id WHERE s.workstation_id=$1 ORDER BY CASE WHEN s.staff_role='primary' THEN 0 ELSE 1 END,s.sort_order,s.employee_id`, schema, schema), workstationID)
	if err != nil {
		return app.WorkstationOwnerResolution{}, true, err
	}
	for rows.Next() {
		var row app.WorkstationStaffCandidate
		if err := rows.Scan(&row.EmployeeID, &row.EmployeeName, &row.Role, &row.SortOrder, &row.Active); err != nil {
			rows.Close()
			return app.WorkstationOwnerResolution{}, true, err
		}
		staff = append(staff, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return app.WorkstationOwnerResolution{}, true, err
	}
	rows.Close()
	var overrideID int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT employee_id FROM %s.production_workstation_overrides WHERE workstation_id=$1 AND work_date=$2::date`, schema), workstationID, workDate).Scan(&overrideID)
	if err != nil && err != pgx.ErrNoRows {
		return app.WorkstationOwnerResolution{}, true, err
	}
	manual := []app.WorkstationStaffCandidate{}
	if overrideID > 0 {
		var candidate app.WorkstationStaffCandidate
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT id,COALESCE(name,''),active AND COALESCE(account_type,'internal_employee')<>'channel_customer' FROM %s.company_employees WHERE id=$1`, schema), overrideID).Scan(&candidate.EmployeeID, &candidate.EmployeeName, &candidate.Active)
		if err != nil && err != pgx.ErrNoRows {
			return app.WorkstationOwnerResolution{}, true, err
		}
		if err == nil {
			manual = append(manual, candidate)
		}
	}
	return app.ResolveWorkstationOwner(staff, attendance, overrideID, manual...), true, nil
}

func productionWorkDate() string {
	return time.Now().In(time.FixedZone("CST", 8*60*60)).Format("2006-01-02")
}

type rosterQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

func (r Repository) ProductionRosterWeek(ctx context.Context, query app.ProductionRosterQuery) (app.ProductionRosterWeek, error) {
	week, err := r.loadProductionRosterWeek(ctx, r.pool, query.WeekStart, nil, nil)
	if err == nil {
		err = r.attachRosterRecentChanges(ctx, r.pool, &week)
	}
	return week, err
}

func (r Repository) loadProductionRosterWeek(ctx context.Context, q rosterQuerier, weekStart string, suppliedEntries []app.ProductionAttendanceEntry, suppliedOverrides []app.ProductionWorkstationOverride) (app.ProductionRosterWeek, error) {
	_, days, err := app.NormalizeProductionRosterWeek(weekStart, nil)
	if err != nil {
		return app.ProductionRosterWeek{}, err
	}
	week := app.ProductionRosterWeek{WeekStart: days[0], WeekEnd: days[6], Days: days, Entries: []app.ProductionAttendanceEntry{}, Overrides: []app.ProductionWorkstationOverride{}, Employees: []app.ProductionRosterEmployee{}, Assignments: []app.ProductionWorkstationDayAssignment{}, Issues: []app.ProductionRosterIssue{}}
	err = q.QueryRow(ctx, fmt.Sprintf(`SELECT version FROM %s.production_roster_weeks WHERE week_start=$1::date`, r.schema), days[0]).Scan(&week.Version)
	if err != nil && err != pgx.ErrNoRows {
		return week, err
	}

	employeeRows, err := q.Query(ctx, fmt.Sprintf(`SELECT id,COALESCE(name,'') FROM %s.company_employees WHERE active=true AND COALESCE(account_type,'internal_employee')<>'channel_customer' ORDER BY name,id`, r.schema))
	if err != nil {
		return week, err
	}
	for employeeRows.Next() {
		var row app.ProductionRosterEmployee
		if err := employeeRows.Scan(&row.ID, &row.Name); err != nil {
			employeeRows.Close()
			return week, err
		}
		week.Employees = append(week.Employees, row)
	}
	if err := employeeRows.Err(); err != nil {
		employeeRows.Close()
		return week, err
	}
	employeeRows.Close()

	attendance := map[string]map[int64]string{}
	for _, day := range days {
		attendance[day] = map[int64]string{}
	}
	if suppliedEntries != nil {
		week.Entries = append(week.Entries, suppliedEntries...)
		for _, entry := range suppliedEntries {
			if attendance[entry.WorkDate] != nil {
				attendance[entry.WorkDate][entry.EmployeeID] = entry.Status
			}
		}
	} else {
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT employee_id,to_char(work_date,'YYYY-MM-DD'),status FROM %s.production_employee_attendance WHERE work_date BETWEEN $1::date AND $2::date ORDER BY employee_id,work_date`, r.schema), days[0], days[6])
		if err != nil {
			return week, err
		}
		for rows.Next() {
			var entry app.ProductionAttendanceEntry
			if err := rows.Scan(&entry.EmployeeID, &entry.WorkDate, &entry.Status); err != nil {
				rows.Close()
				return week, err
			}
			week.Entries = append(week.Entries, entry)
			attendance[entry.WorkDate][entry.EmployeeID] = entry.Status
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return week, err
		}
		rows.Close()
	}

	overrideByKey := map[string]app.ProductionWorkstationOverride{}
	if suppliedOverrides != nil {
		week.Overrides = append(week.Overrides, suppliedOverrides...)
		for _, row := range suppliedOverrides {
			overrideByKey[fmt.Sprintf("%d:%s", row.WorkstationID, row.WorkDate)] = row
		}
	} else {
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT workstation_id,to_char(work_date,'YYYY-MM-DD'),employee_id,COALESCE(reason,'') FROM %s.production_workstation_overrides WHERE work_date BETWEEN $1::date AND $2::date ORDER BY workstation_id,work_date`, r.schema), days[0], days[6])
		if err != nil {
			return week, err
		}
		for rows.Next() {
			var row app.ProductionWorkstationOverride
			if err := rows.Scan(&row.WorkstationID, &row.WorkDate, &row.EmployeeID, &row.Reason); err != nil {
				rows.Close()
				return week, err
			}
			week.Overrides = append(week.Overrides, row)
			overrideByKey[fmt.Sprintf("%d:%s", row.WorkstationID, row.WorkDate)] = row
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return week, err
		}
		rows.Close()
	}

	type workstationRow struct {
		id   int64
		name string
	}
	stations := []workstationRow{}
	stationRows, err := q.Query(ctx, fmt.Sprintf(`SELECT id,name FROM %s.manufacturing_workstations WHERE status='active' ORDER BY name,id`, r.schema))
	if err != nil {
		return week, err
	}
	for stationRows.Next() {
		var row workstationRow
		if err := stationRows.Scan(&row.id, &row.name); err != nil {
			stationRows.Close()
			return week, err
		}
		stations = append(stations, row)
	}
	if err := stationRows.Err(); err != nil {
		stationRows.Close()
		return week, err
	}
	stationRows.Close()
	staffByStation := map[int64][]app.WorkstationStaffCandidate{}
	staffRows, err := q.Query(ctx, fmt.Sprintf(`SELECT s.workstation_id,s.employee_id,COALESCE(e.name,''),s.staff_role,s.sort_order,COALESCE(e.active,false) AND COALESCE(e.account_type,'internal_employee')<>'channel_customer' FROM %s.manufacturing_workstation_employees s LEFT JOIN %s.company_employees e ON e.id=s.employee_id ORDER BY s.workstation_id,CASE WHEN s.staff_role='primary' THEN 0 ELSE 1 END,s.sort_order,s.employee_id`, r.schema, r.schema))
	if err != nil {
		return week, err
	}
	for staffRows.Next() {
		var stationID int64
		var row app.WorkstationStaffCandidate
		if err := staffRows.Scan(&stationID, &row.EmployeeID, &row.EmployeeName, &row.Role, &row.SortOrder, &row.Active); err != nil {
			staffRows.Close()
			return week, err
		}
		staffByStation[stationID] = append(staffByStation[stationID], row)
	}
	if err := staffRows.Err(); err != nil {
		staffRows.Close()
		return week, err
	}
	staffRows.Close()

	for _, station := range stations {
		for _, day := range days {
			override := overrideByKey[fmt.Sprintf("%d:%s", station.id, day)]
			manual := make([]app.WorkstationStaffCandidate, 0, len(week.Employees))
			roles := map[int64]app.WorkstationStaffCandidate{}
			for _, p := range staffByStation[station.id] {
				roles[p.EmployeeID] = p
			}
			candidates := []app.WorkstationStaffCandidate{}
			for _, p := range week.Employees {
				candidate := app.WorkstationStaffCandidate{EmployeeID: p.ID, EmployeeName: p.Name, Role: "other", Active: true}
				if role, ok := roles[p.ID]; ok {
					candidate.Role = role.Role
					candidate.SortOrder = role.SortOrder
				}
				manual = append(manual, candidate)
				if attendance[day][p.ID] == "working" {
					candidates = append(candidates, candidate)
				}
			}
			resolved := app.ResolveWorkstationOwner(staffByStation[station.id], attendance[day], override.EmployeeID, manual...)
			assignment := app.ProductionWorkstationDayAssignment{WorkstationID: station.id, Workstation: station.name, WorkDate: day, EmployeeID: resolved.EmployeeID, EmployeeName: resolved.EmployeeName, Source: resolved.Source, Unattended: resolved.Unattended, OverrideInvalid: resolved.OverrideInvalid, Reason: resolved.Reason}
			if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT count(*) FILTER (WHERE jc.status IN ('pending','ready')),count(*) FILTER (WHERE jc.status IN ('running','paused')),COALESCE(array_agg(jc.id ORDER BY jc.id) FILTER (WHERE jc.status IN ('running','paused') AND $4::bigint>0 AND COALESCE(jc.assigned_employee_id,0)>0 AND jc.assigned_employee_id<>$4::bigint), '{}'::bigint[]) FROM %s.job_cards jc JOIN %s.work_orders wo ON wo.id=jc.work_order_id WHERE wo.status NOT IN ('completed','cancelled') AND jc.status NOT IN ('completed','cancelled') AND (jc.workstation_id=$1 OR (jc.workstation_id=0 AND TRIM(jc.workstation)=TRIM($2))) AND (wo.created_at AT TIME ZONE 'Asia/Shanghai')::date <= $3::date AND (jc.started_at IS NULL OR (jc.started_at AT TIME ZONE 'Asia/Shanghai')::date <= $3::date) AND (jc.planned_start_at IS NULL OR (jc.planned_start_at AT TIME ZONE 'Asia/Shanghai')::date <= $3::date)`, r.schema, r.schema), station.id, station.name, day, assignment.EmployeeID).Scan(&assignment.TaskCount, &assignment.RunningTaskCount, &assignment.HandoverTaskIDs); err != nil {
				return week, err
			}
			assignment.HandoverCount = len(assignment.HandoverTaskIDs)
			assignment.Candidates = candidates
			if assignment.Unattended {
				code := "unattended"
				if assignment.OverrideInvalid {
					code = "invalid_override"
				}
				week.Issues = append(week.Issues, app.ProductionRosterIssue{Code: code, WorkDate: day, WorkstationID: station.id, Message: station.name + " · " + day + " · " + assignment.Reason})
			}
			week.Assignments = append(week.Assignments, assignment)
		}
	}
	week.AffectedStationCount = len(stations)
	if err := q.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(DISTINCT jc.id) FROM %s.job_cards jc JOIN %s.work_orders wo ON wo.id=jc.work_order_id LEFT JOIN %s.manufacturing_workstations ws ON ws.id=jc.workstation_id OR (jc.workstation_id=0 AND TRIM(ws.name)=TRIM(jc.workstation)) WHERE wo.status NOT IN ('completed','cancelled') AND jc.status NOT IN ('completed','cancelled') AND ws.status='active' AND (jc.planned_start_at IS NULL OR (jc.planned_start_at AT TIME ZONE 'Asia/Shanghai')::date <= $1::date)`, r.schema, r.schema, r.schema), days[6]).Scan(&week.AffectedTaskCount); err != nil {
		return week, err
	}
	return week, nil
}

func (r Repository) SaveProductionRoster(ctx context.Context, cmd app.SaveProductionRosterCommand, preview bool) (app.ProductionRosterWeek, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.ProductionRosterWeek{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":production_roster:"+cmd.WeekStart); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	payloadHash := scheduleHash(struct {
		Week        string
		Version     int64
		Entries     []app.ProductionAttendanceEntry
		Overrides   []app.ProductionWorkstationOverride
		Replacement *app.ProductionRosterReplacement
		Fingerprint string
	}{cmd.WeekStart, cmd.ExpectedVersion, cmd.Entries, cmd.Overrides, cmd.Replacement, cmd.ExpectedPreviewFingerprint})
	if !preview && strings.TrimSpace(cmd.RequestID) != "" {
		var hash string
		var data []byte
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT payload_hash,response_json FROM %s.production_roster_requests WHERE actor=$1 AND request_id=$2`, r.schema), cmd.Operator, cmd.RequestID).Scan(&hash, &data)
		if err == nil {
			if hash != payloadHash {
				return app.ProductionRosterWeek{}, fmt.Errorf("同一请求编号不能用于不同排班")
			}
			var out app.ProductionRosterWeek
			if err := json.Unmarshal(data, &out); err != nil {
				return out, err
			}
			out.Replayed = true
			return out, nil
		}
		if err != pgx.ErrNoRows {
			return app.ProductionRosterWeek{}, err
		}
	}
	var currentVersion int64
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT version FROM %s.production_roster_weeks WHERE week_start=$1::date FOR UPDATE`, r.schema), cmd.WeekStart).Scan(&currentVersion)
	if err == pgx.ErrNoRows {
		currentVersion = 0
	} else if err != nil {
		return app.ProductionRosterWeek{}, err
	}
	if currentVersion != cmd.ExpectedVersion {
		return app.ProductionRosterWeek{}, &app.ScheduleError{Code: "version_conflict", Message: "本周排班已被他人修改，请重新读取"}
	}
	before, err := r.loadProductionRosterWeek(ctx, tx, cmd.WeekStart, nil, nil)
	if err != nil {
		return app.ProductionRosterWeek{}, err
	}
	if err = r.attachRosterRecentChanges(ctx, tx, &before); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	cmd, released := app.NormalizeRosterLeaveOverrides(before, cmd)
	if cmd.Replacement != nil {
		draft, loadErr := r.loadProductionRosterWeek(ctx, tx, cmd.WeekStart, cmd.Entries, cmd.Overrides)
		if loadErr != nil {
			return app.ProductionRosterWeek{}, loadErr
		}
		cmd, err = app.ApplyRosterReplacement(before, draft, cmd)
		if err != nil {
			return app.ProductionRosterWeek{}, err
		}
		var extra []app.ProductionWorkstationOverride
		cmd, extra = app.NormalizeRosterLeaveOverrides(before, cmd)
		for _, o := range extra {
			found := false
			for _, old := range released {
				if old.WorkstationID == o.WorkstationID && old.WorkDate == o.WorkDate {
					found = true
					break
				}
			}
			if !found {
				released = append(released, o)
			}
		}
	}
	ids := map[int64]bool{}
	for _, entry := range cmd.Entries {
		ids[entry.EmployeeID] = true
	}
	for _, override := range cmd.Overrides {
		ids[override.EmployeeID] = true
	}
	for id := range ids {
		var active bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT active AND COALESCE(account_type,'internal_employee')<>'channel_customer' FROM %s.company_employees WHERE id=$1`, r.schema), id).Scan(&active)
		if err != nil || !active {
			return app.ProductionRosterWeek{}, fmt.Errorf("排班人员已停用或不属于内部员工")
		}
	}
	stationIDs := map[int64]bool{}
	for _, override := range cmd.Overrides {
		stationIDs[override.WorkstationID] = true
	}
	for id := range stationIDs {
		var active bool
		err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT status='active' FROM %s.manufacturing_workstations WHERE id=$1`, r.schema), id).Scan(&active)
		if err != nil || !active {
			return app.ProductionRosterWeek{}, fmt.Errorf("临时调整的工位不存在或已停用")
		}
	}
	previewResult, err := r.loadProductionRosterWeek(ctx, tx, cmd.WeekStart, cmd.Entries, cmd.Overrides)
	if err != nil {
		return app.ProductionRosterWeek{}, err
	}
	for _, row := range previewResult.Assignments {
		if row.OverrideInvalid {
			return app.ProductionRosterWeek{}, fmt.Errorf("%s %s 的临时负责人当天未上班或已停用", row.WorkDate, row.Workstation)
		}
	}
	previewResult.ReleasedOverrides = released
	app.DescribeRosterChanges(before, &previewResult)
	previewResult.RecentChanges = before.RecentChanges
	if err = r.countRosterAffectedTasks(ctx, tx, &previewResult); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	previewResult.PreviewFingerprint = scheduleHash(struct {
		Version     int64
		Entries     []app.ProductionAttendanceEntry
		Overrides   []app.ProductionWorkstationOverride
		Assignments []app.ProductionWorkstationDayAssignment
		Changes     []app.ProductionRosterAssignmentChange
	}{currentVersion, previewResult.Entries, previewResult.Overrides, previewResult.Assignments, previewResult.Changes})
	if !preview && cmd.ExpectedPreviewFingerprint != "" && cmd.ExpectedPreviewFingerprint != previewResult.PreviewFingerprint {
		return app.ProductionRosterWeek{}, &app.ScheduleError{Code: "preview_changed", Message: "出勤、工位人员或任务范围已变化，请重新核对后保存"}
	}
	previewResult.Version = currentVersion + 1
	if preview {
		previewResult.Saved = false
		return previewResult, tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_employee_attendance WHERE work_date BETWEEN $1::date AND ($1::date+6)`, r.schema), cmd.WeekStart); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	for _, entry := range cmd.Entries {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_employee_attendance(work_date,employee_id,status,updated_by) VALUES($1::date,$2,$3,$4)`, r.schema), entry.WorkDate, entry.EmployeeID, entry.Status, cmd.Operator); err != nil {
			return app.ProductionRosterWeek{}, err
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`DELETE FROM %s.production_workstation_overrides WHERE work_date BETWEEN $1::date AND ($1::date+6)`, r.schema), cmd.WeekStart); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	for _, row := range cmd.Overrides {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_workstation_overrides(work_date,workstation_id,employee_id,reason,updated_by) VALUES($1::date,$2,$3,$4,$5)`, r.schema), row.WorkDate, row.WorkstationID, row.EmployeeID, row.Reason, cmd.Operator); err != nil {
			return app.ProductionRosterWeek{}, err
		}
	}
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_roster_weeks(week_start,version,updated_by,updated_at) VALUES($1::date,$2,$3,now()) ON CONFLICT(week_start) DO UPDATE SET version=excluded.version,updated_by=excluded.updated_by,updated_at=now()`, r.schema), cmd.WeekStart, currentVersion+1, cmd.Operator); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	previewResult.Saved = true
	previewResult.RecentChanges = previewResult.Changes
	data, _ := json.Marshal(previewResult)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_roster_requests(actor,request_id,payload_hash,response_json) VALUES($1,$2,$3,$4)`, r.schema), cmd.Operator, cmd.RequestID, payloadHash, data); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_roster", nil, "save", postgresinfra.StrPtr("week"), nil, postgresinfra.StrPtr(cmd.WeekStart), postgresinfra.AuditMeta{"version": currentVersion + 1, "entry_count": len(cmd.Entries), "override_count": len(cmd.Overrides), "affected_workstation_count": previewResult.AffectedStationCount, "affected_task_count": previewResult.AffectedTaskCount, "attendance_changes": previewResult.AttendanceChanges, "assignment_changes": previewResult.Changes, "released_overrides": previewResult.ReleasedOverrides, "replacement": cmd.Replacement}); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.ProductionRosterWeek{}, err
	}
	return previewResult, nil
}

func (r Repository) ResolveProductionWorkstationAssignments(ctx context.Context, date string) ([]app.ProductionWorkstationDayAssignment, error) {
	week, _, err := app.NormalizeProductionRosterWeek(date, nil)
	if err != nil {
		return nil, err
	}
	result, err := r.ProductionRosterWeek(ctx, app.ProductionRosterQuery{WeekStart: week})
	if err != nil {
		return nil, err
	}
	out := []app.ProductionWorkstationDayAssignment{}
	for _, row := range result.Assignments {
		if row.WorkDate == date {
			out = append(out, row)
		}
	}
	return out, nil
}

func (r Repository) ProductionTodayRoster(ctx context.Context, date string, employeeID int64) (app.ProductionTodayRoster, error) {
	weekStart, _, err := app.NormalizeProductionRosterWeek(date, nil)
	if err != nil {
		return app.ProductionTodayRoster{}, err
	}
	week, err := r.ProductionRosterWeek(ctx, app.ProductionRosterQuery{WeekStart: weekStart})
	if err != nil {
		return app.ProductionTodayRoster{}, err
	}
	out := app.ProductionTodayRoster{Date: date, RosterVersion: week.Version, EmployeeID: employeeID, Attendance: "unplanned", Assignments: []app.ProductionWorkstationDayAssignment{}}
	if employeeID > 0 {
		_ = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT COALESCE(name,'') FROM %s.company_employees WHERE id=$1`, r.schema), employeeID).Scan(&out.EmployeeName)
		_ = r.pool.QueryRow(ctx, fmt.Sprintf(`SELECT status FROM %s.production_employee_attendance WHERE employee_id=$1 AND work_date=$2::date`, r.schema), employeeID, date).Scan(&out.Attendance)
	}
	for _, row := range week.Assignments {
		if row.WorkDate != date {
			continue
		}
		if employeeID <= 0 || row.EmployeeID == employeeID {
			out.Assignments = append(out.Assignments, row)
		}
	}
	return out, nil
}

func (r Repository) HandoverWorkstation(ctx context.Context, cmd app.HandoverWorkstationCommand) (app.HandoverWorkstationResult, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return app.HandoverWorkstationResult{}, err
	}
	defer tx.Rollback(ctx)
	weekStart, _, err := app.NormalizeProductionRosterWeek(cmd.WorkDate, nil)
	if err != nil {
		return app.HandoverWorkstationResult{}, err
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, r.schema+":production_roster:"+weekStart); err != nil {
		return app.HandoverWorkstationResult{}, err
	}

	var existing []byte
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT jsonb_build_object('workstation_id',workstation_id,'work_date',to_char(work_date,'YYYY-MM-DD'),'employee_id',new_employee_id,'job_card_ids',job_card_ids_json) FROM %s.production_workstation_handovers WHERE operator=$1 AND request_id=$2`, r.schema), cmd.Operator, cmd.RequestID).Scan(&existing)
	if err == nil {
		var out app.HandoverWorkstationResult
		if err := json.Unmarshal(existing, &out); err != nil {
			return out, err
		}
		if out.WorkstationID != cmd.WorkstationID || out.WorkDate != cmd.WorkDate || out.EmployeeID != cmd.EmployeeID {
			return out, fmt.Errorf("同一请求编号不能用于不同交接")
		}
		out.Replayed = true
		return out, nil
	}
	if err != pgx.ErrNoRows {
		return app.HandoverWorkstationResult{}, err
	}
	var currentVersion int64
	if err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT version FROM %s.production_roster_weeks WHERE week_start=$1::date FOR UPDATE`, r.schema), weekStart).Scan(&currentVersion); err != nil {
		if err == pgx.ErrNoRows {
			return app.HandoverWorkstationResult{}, &app.ScheduleError{Code: "version_conflict", Message: "本周排班已变化，请重新读取后再交接"}
		}
		return app.HandoverWorkstationResult{}, err
	}
	if currentVersion != cmd.ExpectedVersion {
		return app.HandoverWorkstationResult{}, &app.ScheduleError{Code: "version_conflict", Message: "本周排班已变化，请重新读取后再交接"}
	}
	assignments, err := r.loadProductionRosterWeek(ctx, tx, cmd.WorkDate, nil, nil)
	if err != nil {
		return app.HandoverWorkstationResult{}, err
	}
	var assignment app.ProductionWorkstationDayAssignment
	for _, row := range assignments.Assignments {
		if row.WorkDate == cmd.WorkDate && row.WorkstationID == cmd.WorkstationID {
			assignment = row
			break
		}
	}
	if assignment.EmployeeID != cmd.EmployeeID {
		return app.HandoverWorkstationResult{}, fmt.Errorf("接班人必须是当天该工位负责人")
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT jc.id,COALESCE(jc.assigned_employee_id,0),COALESCE(jc.assigned_to,'') FROM %s.job_cards jc JOIN %s.work_orders wo ON wo.id=jc.work_order_id WHERE wo.status NOT IN ('completed','cancelled') AND jc.status IN ('running','paused') AND (jc.workstation_id=$1 OR (jc.workstation_id=0 AND TRIM(jc.workstation)=TRIM($2))) FOR UPDATE OF jc`, r.schema, r.schema), cmd.WorkstationID, assignment.Workstation)
	if err != nil {
		return app.HandoverWorkstationResult{}, err
	}
	ids := []int64{}
	previous := int64(0)
	previousAssignments := []map[string]any{}
	for rows.Next() {
		var id, old int64
		var oldName string
		if err := rows.Scan(&id, &old, &oldName); err != nil {
			rows.Close()
			return app.HandoverWorkstationResult{}, err
		}
		ids = append(ids, id)
		previousAssignments = append(previousAssignments, map[string]any{"job_card_id": id, "employee_id": old, "employee_name": oldName})
		if previous == 0 {
			previous = old
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return app.HandoverWorkstationResult{}, err
	}
	rows.Close()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) > 0 {
		if _, err = tx.Exec(ctx, fmt.Sprintf(`UPDATE %s.job_cards SET assigned_employee_id=$2,assigned_to=$3,schedule_version=schedule_version+1 WHERE id=ANY($1)`, r.schema), ids, cmd.EmployeeID, assignment.EmployeeName); err != nil {
			return app.HandoverWorkstationResult{}, err
		}
	}
	jobJSON, _ := json.Marshal(ids)
	previousJSON, _ := json.Marshal(previousAssignments)
	if _, err = tx.Exec(ctx, fmt.Sprintf(`INSERT INTO %s.production_workstation_handovers(work_date,workstation_id,previous_employee_id,new_employee_id,job_card_ids_json,previous_assignments_json,request_id,operator) VALUES($1::date,$2,$3,$4,$5,$6,$7,$8)`, r.schema), cmd.WorkDate, cmd.WorkstationID, previous, cmd.EmployeeID, jobJSON, previousJSON, cmd.RequestID, cmd.Operator); err != nil {
		return app.HandoverWorkstationResult{}, err
	}
	if err := postgresinfra.AuditInsertTx(ctx, tx, r.schema, cmd.Operator, "production_workstation_handover", &cmd.WorkstationID, "handover", postgresinfra.StrPtr("employee_id"), postgresinfra.StrPtr(fmt.Sprint(previous)), postgresinfra.StrPtr(fmt.Sprint(cmd.EmployeeID)), postgresinfra.AuditMeta{"work_date": cmd.WorkDate, "job_card_ids": ids, "previous_assignments": previousAssignments}); err != nil {
		return app.HandoverWorkstationResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return app.HandoverWorkstationResult{}, err
	}
	return app.HandoverWorkstationResult{WorkstationID: cmd.WorkstationID, WorkDate: cmd.WorkDate, EmployeeID: cmd.EmployeeID, EmployeeName: assignment.EmployeeName, JobCardIDs: ids}, nil
}

func (r Repository) attachRosterRecentChanges(ctx context.Context, q rosterQuerier, week *app.ProductionRosterWeek) error {
	week.RecentChanges = []app.ProductionRosterAssignmentChange{}
	if week.Version <= 0 {
		return nil
	}
	var raw []byte
	err := q.QueryRow(ctx, fmt.Sprintf(`SELECT response_json FROM %s.production_roster_requests WHERE response_json->>'week_start'=$1 AND response_json->>'version'=$2 ORDER BY created_at DESC,request_id DESC LIMIT 1`, r.schema), week.WeekStart, fmt.Sprint(week.Version)).Scan(&raw)
	if err == pgx.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var prior app.ProductionRosterWeek
	if err = json.Unmarshal(raw, &prior); err != nil {
		return err
	}
	if prior.Changes != nil {
		week.RecentChanges = prior.Changes
	}
	return nil
}

func (r Repository) countRosterAffectedTasks(ctx context.Context, q rosterQuerier, week *app.ProductionRosterWeek) error {
	ids := map[int64]bool{}
	for i := range week.Changes {
		change := &week.Changes[i]
		change.TaskIDs = []int64{}
		change.HandoverTaskIDs = []int64{}
		rows, err := q.Query(ctx, fmt.Sprintf(`SELECT jc.id,jc.status,COALESCE(jc.assigned_employee_id,0) FROM %s.job_cards jc JOIN %s.work_orders wo ON wo.id=jc.work_order_id WHERE wo.status NOT IN ('completed','cancelled') AND jc.status IN ('pending','ready','running','paused') AND (jc.workstation_id=$1 OR (jc.workstation_id=0 AND TRIM(jc.workstation)=TRIM($2))) AND (wo.created_at AT TIME ZONE 'Asia/Shanghai')::date <= $3::date AND (jc.started_at IS NULL OR (jc.started_at AT TIME ZONE 'Asia/Shanghai')::date <= $3::date) AND (jc.planned_start_at IS NULL OR (jc.planned_start_at AT TIME ZONE 'Asia/Shanghai')::date <= $3::date)`, r.schema, r.schema), change.WorkstationID, change.Workstation, change.WorkDate)
		if err != nil {
			return err
		}
		for rows.Next() {
			var id, assigned int64
			var status string
			if err = rows.Scan(&id, &status, &assigned); err != nil {
				rows.Close()
				return err
			}
			if status == "pending" || status == "ready" {
				ids[id] = true
				change.TaskIDs = append(change.TaskIDs, id)
			} else if change.ToEmployeeID > 0 && assigned > 0 && assigned != change.ToEmployeeID {
				change.HandoverTaskIDs = append(change.HandoverTaskIDs, id)
			}
		}
		sort.Slice(change.TaskIDs, func(i, j int) bool { return change.TaskIDs[i] < change.TaskIDs[j] })
		sort.Slice(change.HandoverTaskIDs, func(i, j int) bool { return change.HandoverTaskIDs[i] < change.HandoverTaskIDs[j] })
		change.TaskCount = len(change.TaskIDs)
		change.HandoverCount = len(change.HandoverTaskIDs)
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
	}
	week.AffectedTaskCount = len(ids)
	return nil
}
