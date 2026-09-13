package production

import "testing"

func staffPtr[T any](v T) *T { return &v }
func TestSchedulePatchPreservesUnrelatedFields(t *testing.T) {
	row := ScheduleTask{JobCardRow: JobCardRow{ID: 1, WorkOrderID: 2, Status: "pending", AssignedEmployeeID: 3, PlannedStartAt: "2026-09-12 09:00", PlannedEndAt: "2026-09-12 10:00", ShiftCode: "早班", Workstation: "包装台", SchedulingNote: "保留", ScheduleVersion: 4}}
	got, err := MergeSchedulePatch(row, ScheduleTaskPatch{JobCardID: 1, WorkOrderID: 2, ExpectedVersion: staffPtr[int64](4), AssignedEmployeeID: staffPtr[int64](5)})
	if err != nil {
		t.Fatal(err)
	}
	if got.PlannedStartAt != row.PlannedStartAt || got.ShiftCode != row.ShiftCode || got.Workstation != row.Workstation || got.SchedulingNote != "保留" || got.AssignedEmployeeID != 5 {
		t.Fatalf("unexpected merged patch: %+v", got)
	}
	if _, err = MergeSchedulePatch(row, ScheduleTaskPatch{ExpectedVersion: staffPtr[int64](3)}); err == nil {
		t.Fatal("stale version accepted")
	}
}
func TestSchedulePatchRejectsHistoricalAndInvalidTimes(t *testing.T) {
	for _, status := range []string{"completed", "cancelled"} {
		if _, err := MergeSchedulePatch(ScheduleTask{JobCardRow: JobCardRow{Status: status}}, ScheduleTaskPatch{}); err == nil {
			t.Fatalf("editable %s", status)
		}
	}
	row := ScheduleTask{JobCardRow: JobCardRow{Status: "pending", PlannedStartAt: "2026-09-12 10:00", PlannedEndAt: "2026-09-12 11:00"}}
	if _, err := MergeSchedulePatch(row, ScheduleTaskPatch{PlannedEndAt: staffPtr("2026-09-12 09:00")}); err == nil {
		t.Fatal("end before start accepted")
	}
}
func TestEmployeeOverlapUsesResponsibleEmployeeOnlyAndBoundary(t *testing.T) {
	a := ScheduleTask{JobCardRow: JobCardRow{ID: 1, WorkOrderNo: "WO-A", AssignedEmployeeID: 1, CollaboratorEmployeeIDs: []int64{2}, PlannedStartAt: "2026-09-12 09:00", PlannedEndAt: "2026-09-12 11:00"}}
	b := ScheduleTask{JobCardRow: JobCardRow{ID: 2, WorkOrderNo: "WO-B", AssignedEmployeeID: 2, PlannedStartAt: "2026-09-12 10:00", PlannedEndAt: "2026-09-12 12:00"}}
	if conflicts := StaffScheduleConflicts([]ScheduleTask{a}, []ScheduleTask{b}); len(conflicts) != 0 {
		t.Fatalf("retired collaborator must not create active conflict %+v", conflicts)
	}
	a.AssignedEmployeeID = 2
	if conflicts := StaffScheduleConflicts([]ScheduleTask{a}, []ScheduleTask{b}); len(conflicts) != 1 || conflicts[0].EmployeeID != 2 {
		t.Fatalf("missing responsible employee overlap %+v", conflicts)
	}
	b.PlannedStartAt = a.PlannedEndAt
	if conflicts := StaffScheduleConflicts([]ScheduleTask{a}, []ScheduleTask{b}); len(conflicts) != 0 {
		t.Fatalf("touching boundaries overlap %+v", conflicts)
	}
}

func TestSchedulePatchRejectsRetiredCollaboratorAssignments(t *testing.T) {
	row := ScheduleTask{JobCardRow: JobCardRow{ID: 1, WorkOrderID: 2, Status: "pending", AssignedEmployeeID: 3}}
	ids := []int64{4}
	if _, err := MergeSchedulePatch(row, ScheduleTaskPatch{CollaboratorEmployeeIDs: &ids}); err == nil || err.Error() != "协作人员功能已停用，请只设置负责人" {
		t.Fatalf("retired collaborators accepted: %v", err)
	}
	empty := []int64{}
	if _, err := MergeSchedulePatch(row, ScheduleTaskPatch{CollaboratorEmployeeIDs: &empty}); err != nil {
		t.Fatalf("empty compatibility field rejected: %v", err)
	}
}
