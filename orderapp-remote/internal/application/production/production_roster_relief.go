package production

import (
	"fmt"
	"strings"
)

type ProductionRosterReplacement struct {
	WorkDate       string  `json:"work_date"`
	FromEmployeeID int64   `json:"from_employee_id"`
	ToEmployeeID   int64   `json:"to_employee_id"`
	WorkstationIDs []int64 `json:"workstation_ids"`
	MarkFromOff    bool    `json:"mark_from_off"`
}
type ProductionRosterAssignmentChange struct {
	WorkstationID    int64   `json:"workstation_id"`
	Workstation      string  `json:"workstation"`
	WorkDate         string  `json:"work_date"`
	FromEmployeeID   int64   `json:"from_employee_id"`
	FromEmployeeName string  `json:"from_employee_name"`
	ToEmployeeID     int64   `json:"to_employee_id"`
	ToEmployeeName   string  `json:"to_employee_name"`
	Source           string  `json:"source"`
	Reason           string  `json:"reason"`
	TaskCount        int     `json:"task_count"`
	HandoverCount    int     `json:"handover_count"`
	TaskIDs          []int64 `json:"task_ids"`
	HandoverTaskIDs  []int64 `json:"handover_task_ids"`
}
type ProductionAttendanceChange struct {
	EmployeeID   int64  `json:"employee_id"`
	EmployeeName string `json:"employee_name"`
	WorkDate     string `json:"work_date"`
	FromStatus   string `json:"from_status"`
	ToStatus     string `json:"to_status"`
}

func rosterCell(id int64, day string) string { return fmt.Sprintf("%d:%s", id, day) }
func attendanceByCell(entries []ProductionAttendanceEntry) map[string]string {
	out := map[string]string{}
	for _, e := range entries {
		out[rosterCell(e.EmployeeID, e.WorkDate)] = e.Status
	}
	return out
}
func workstationAbsenceReason(staff []WorkstationStaffCandidate, attendance map[int64]string) string {
	states := map[string][]string{"primary": {}, "backup": {}}
	for _, p := range staff {
		state := "未排班"
		if !p.Active {
			state = "已停用"
		} else if attendance[p.EmployeeID] == "off" {
			state = "休息"
		}
		states[p.Role] = append(states[p.Role], state)
	}
	parts := []string{}
	for _, role := range []string{"primary", "backup"} {
		label := "主负责人"
		if role == "backup" {
			label = "替补"
		}
		if len(states[role]) == 0 {
			parts = append(parts, "未配置"+label)
			continue
		}
		unique := []string{}
		for _, state := range states[role] {
			found := false
			for _, s := range unique {
				found = found || state == s
			}
			if !found {
				unique = append(unique, state)
			}
		}
		parts = append(parts, label+strings.Join(unique, "、"))
	}
	return strings.Join(parts, "，")
}

// An explicit attendance edit releases that employee's daily override. External
// deactivation is intentionally left visible for the scheduler to resolve.
func NormalizeRosterLeaveOverrides(before ProductionRosterWeek, cmd SaveProductionRosterCommand) (SaveProductionRosterCommand, []ProductionWorkstationOverride) {
	prior := attendanceByCell(before.Entries)
	next := attendanceByCell(cmd.Entries)
	active := map[int64]string{}
	for _, e := range before.Employees {
		active[e.ID] = e.Name
	}
	kept := make([]ProductionWorkstationOverride, 0, len(cmd.Overrides))
	released := []ProductionWorkstationOverride{}
	// Include already-normalized removals so a subsequent draft edit retains the leave explanation.
	submitted := map[string]bool{}
	candidates := append([]ProductionWorkstationOverride(nil), cmd.Overrides...)
	for _, o := range cmd.Overrides {
		submitted[rosterCell(o.WorkstationID, o.WorkDate)] = true
	}
	for _, o := range before.Overrides {
		if !submitted[rosterCell(o.WorkstationID, o.WorkDate)] {
			candidates = append(candidates, o)
		}
	}
	for _, o := range candidates {
		k := rosterCell(o.EmployeeID, o.WorkDate)
		status := next[k]
		if status == "" {
			status = "unplanned"
		}
		if active[o.EmployeeID] != "" && prior[k] == "working" && status != "working" {
			label := "未排班"
			if status == "off" {
				label = "休息"
			}
			o.Reason = active[o.EmployeeID] + "当天改为" + label + "，恢复自动安排"
			released = append(released, o)
		} else if submitted[rosterCell(o.WorkstationID, o.WorkDate)] {
			kept = append(kept, o)
		}
	}
	cmd.Overrides = kept
	return cmd, released
}

func ApplyRosterReplacement(before, draft ProductionRosterWeek, cmd SaveProductionRosterCommand) (SaveProductionRosterCommand, error) {
	r := cmd.Replacement
	if r == nil {
		return cmd, nil
	}
	if r.FromEmployeeID <= 0 || r.ToEmployeeID <= 0 || r.FromEmployeeID == r.ToEmployeeID || len(r.WorkstationIDs) == 0 {
		return cmd, fmt.Errorf("请选择不同的原员工、接替员工及工位")
	}
	names := map[int64]string{}
	for _, p := range draft.Employees {
		names[p.ID] = p.Name
	}
	if names[r.FromEmployeeID] == "" || names[r.ToEmployeeID] == "" {
		return cmd, fmt.Errorf("换人员工已停用或不是内部员工")
	}
	if attendanceByCell(cmd.Entries)[rosterCell(r.ToEmployeeID, r.WorkDate)] != "working" {
		return cmd, fmt.Errorf("接替员工当天必须上班")
	}
	allowed := map[int64]bool{}
	current := map[int64]ProductionWorkstationDayAssignment{}
	for _, a := range draft.Assignments {
		if a.WorkDate == r.WorkDate {
			current[a.WorkstationID] = a
			if a.EmployeeID == r.FromEmployeeID {
				allowed[a.WorkstationID] = true
			}
		}
	}
	for _, a := range before.Assignments {
		if a.WorkDate == r.WorkDate && a.EmployeeID == r.FromEmployeeID {
			allowed[a.WorkstationID] = true
		}
	}
	for _, a := range before.RecentChanges {
		if a.WorkDate == r.WorkDate && a.FromEmployeeID == r.FromEmployeeID && current[a.WorkstationID].EmployeeID == a.ToEmployeeID {
			allowed[a.WorkstationID] = true
		}
	}
	selected := map[int64]bool{}
	for _, id := range r.WorkstationIDs {
		_, exists := current[id]
		if !exists || !allowed[id] || selected[id] {
			return cmd, &ScheduleError{Code: "version_conflict", Message: "原员工的工位范围已变化，请重新核对"}
		}
		selected[id] = true
	}
	overrides := make([]ProductionWorkstationOverride, 0, len(cmd.Overrides)+len(selected))
	for _, o := range cmd.Overrides {
		if o.WorkDate != r.WorkDate || !selected[o.WorkstationID] {
			overrides = append(overrides, o)
		}
	}
	for _, id := range r.WorkstationIDs {
		overrides = append(overrides, ProductionWorkstationOverride{WorkstationID: id, WorkDate: r.WorkDate, EmployeeID: r.ToEmployeeID, Reason: "整日换人：" + names[r.FromEmployeeID] + " → " + names[r.ToEmployeeID]})
	}
	cmd.Overrides = overrides
	cmd.Entries = append([]ProductionAttendanceEntry(nil), cmd.Entries...)
	if r.MarkFromOff {
		found := false
		for i := range cmd.Entries {
			e := &cmd.Entries[i]
			if e.EmployeeID == r.FromEmployeeID && e.WorkDate == r.WorkDate {
				e.Status = "off"
				found = true
			}
		}
		if !found {
			cmd.Entries = append(cmd.Entries, ProductionAttendanceEntry{EmployeeID: r.FromEmployeeID, WorkDate: r.WorkDate, Status: "off"})
		}
	}
	return cmd, nil
}

func DescribeRosterChanges(before ProductionRosterWeek, after *ProductionRosterWeek) {
	prior := map[string]ProductionWorkstationDayAssignment{}
	for _, a := range before.Assignments {
		prior[rosterCell(a.WorkstationID, a.WorkDate)] = a
	}
	oldReasons := map[string]string{}
	for _, o := range before.Overrides {
		oldReasons[rosterCell(o.WorkstationID, o.WorkDate)] = o.Reason
	}
	overrideReasons := map[string]string{}
	for _, o := range after.Overrides {
		if oldReasons[rosterCell(o.WorkstationID, o.WorkDate)] != o.Reason {
			overrideReasons[rosterCell(o.WorkstationID, o.WorkDate)] = o.Reason
		}
	}
	for _, o := range after.ReleasedOverrides {
		overrideReasons[rosterCell(o.WorkstationID, o.WorkDate)] = o.Reason
	}
	after.Changes = []ProductionRosterAssignmentChange{}
	stations := map[int64]bool{}
	for _, a := range after.Assignments {
		k := rosterCell(a.WorkstationID, a.WorkDate)
		old := prior[k]
		reason := overrideReasons[k]
		if old.EmployeeID == a.EmployeeID && old.Source == a.Source && old.OverrideInvalid == a.OverrideInvalid && reason == "" {
			continue
		}
		if reason == "" {
			reason = "按当天出勤自动匹配"
		}
		after.Changes = append(after.Changes, ProductionRosterAssignmentChange{WorkstationID: a.WorkstationID, Workstation: a.Workstation, WorkDate: a.WorkDate, FromEmployeeID: old.EmployeeID, FromEmployeeName: old.EmployeeName, ToEmployeeID: a.EmployeeID, ToEmployeeName: a.EmployeeName, Source: a.Source, Reason: reason, TaskCount: a.TaskCount, HandoverCount: a.HandoverCount})
		stations[a.WorkstationID] = true
	}
	after.AffectedStationCount = len(stations)
	after.AttendanceChanges = []ProductionAttendanceChange{}
	oldEntries := attendanceByCell(before.Entries)
	newEntries := attendanceByCell(after.Entries)
	for _, employee := range after.Employees {
		for _, day := range after.Days {
			k := rosterCell(employee.ID, day)
			from, to := oldEntries[k], newEntries[k]
			if from == "" {
				from = "unplanned"
			}
			if to == "" {
				to = "unplanned"
			}
			if from != to {
				after.AttendanceChanges = append(after.AttendanceChanges, ProductionAttendanceChange{EmployeeID: employee.ID, EmployeeName: employee.Name, WorkDate: day, FromStatus: from, ToStatus: to})
			}
		}
	}
}
