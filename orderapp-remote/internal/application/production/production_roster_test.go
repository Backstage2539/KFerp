package production

import (
	"context"
	"testing"
	"time"
)

func TestResolveWorkstationOwnerUsesOverrideThenPrimaryThenOrderedBackup(t *testing.T) {
	staff := []WorkstationStaffCandidate{
		{EmployeeID: 11, EmployeeName: "主负责人", Role: "primary", SortOrder: 0, Active: true},
		{EmployeeID: 12, EmployeeName: "替补甲", Role: "backup", SortOrder: 1, Active: true},
		{EmployeeID: 13, EmployeeName: "替补乙", Role: "backup", SortOrder: 2, Active: true},
	}
	attendance := map[int64]string{11: "off", 12: "working", 13: "working"}

	auto := ResolveWorkstationOwner(staff, attendance, 0)
	if auto.EmployeeID != 12 || auto.Source != "backup" || auto.Unattended {
		t.Fatalf("ordered backup was not selected: %+v", auto)
	}

	override := ResolveWorkstationOwner(staff, attendance, 13)
	if override.EmployeeID != 13 || override.Source != "override" {
		t.Fatalf("valid override was not selected: %+v", override)
	}

	invalid := ResolveWorkstationOwner(staff, attendance, 99)
	if !invalid.Unattended || !invalid.OverrideInvalid || invalid.EmployeeID != 0 {
		t.Fatalf("invalid override silently fell back: %+v", invalid)
	}
}

func TestResolveWorkstationOwnerTreatsUnplannedAsAbsent(t *testing.T) {
	staff := []WorkstationStaffCandidate{{EmployeeID: 21, EmployeeName: "负责人", Role: "primary", Active: true}}
	for _, attendance := range []map[int64]string{{}, {21: "unplanned"}, {21: "off"}} {
		resolved := ResolveWorkstationOwner(staff, attendance, 0)
		if !resolved.Unattended || resolved.EmployeeID != 0 {
			t.Fatalf("unplanned/off employee was assigned: %+v", resolved)
		}
	}
}

func TestNormalizeProductionRosterWeekUsesBeijingMonday(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	week, days, err := NormalizeProductionRosterWeek("2026-09-17", loc)
	if err != nil {
		t.Fatal(err)
	}
	if week != "2026-09-14" || len(days) != 7 || days[6] != "2026-09-20" {
		t.Fatalf("week=%s days=%v", week, days)
	}
}

func TestValidateProductionRosterEntriesRejectsDuplicateAndInvalidState(t *testing.T) {
	entries := []ProductionAttendanceEntry{
		{EmployeeID: 1, WorkDate: "2026-09-14", Status: "working"},
		{EmployeeID: 1, WorkDate: "2026-09-14", Status: "off"},
	}
	if err := ValidateProductionRosterEntries("2026-09-14", entries); err == nil {
		t.Fatal("duplicate attendance was accepted")
	}
	if err := ValidateProductionRosterEntries("2026-09-14", []ProductionAttendanceEntry{{EmployeeID: 1, WorkDate: "2026-09-14", Status: "unknown"}}); err == nil {
		t.Fatal("invalid state was accepted")
	}
}

func TestSaveProductionRosterRejectsOverrideOutsideWeek(t *testing.T) {
	service := NewService(&fakeRepo{})
	_, err := service.SaveProductionRoster(context.Background(), SaveProductionRosterCommand{
		WeekStart: "2026-09-14", Operator: "排班员",
		Overrides: []ProductionWorkstationOverride{{WorkstationID: 1, EmployeeID: 2, WorkDate: "2026-09-21"}},
	}, true)
	if err == nil || err.Error() != "工位临时负责人无效" {
		t.Fatalf("outside-week override err=%v", err)
	}
}

func TestRosterManualOwnerCanBeWorkingEmployeeOutsideStationRoster(t *testing.T) {
	staff := []WorkstationStaffCandidate{{EmployeeID: 1, EmployeeName: "主负责人", Role: "primary", Active: true}}
	people := []WorkstationStaffCandidate{{EmployeeID: 3, EmployeeName: "临时接替", Active: true}}
	owner := ResolveWorkstationOwner(staff, map[int64]string{1: "working", 3: "working"}, 3, people...)
	if owner.EmployeeID != 3 || owner.Source != "override" {
		t.Fatalf("manual owner: %+v", owner)
	}
	owner = ResolveWorkstationOwner(staff, map[int64]string{1: "off", 3: "working"}, 0, people...)
	if !owner.Unattended {
		t.Fatal("non-station employee was automatically assigned")
	}
	owner = ResolveWorkstationOwner(staff, map[int64]string{3: "off"}, 3, people...)
	if !owner.OverrideInvalid {
		t.Fatal("off employee manually assigned")
	}
}

func TestRosterLeaveClearsOverrideButPreservesExternalInvalidity(t *testing.T) {
	before := ProductionRosterWeek{Employees: []ProductionRosterEmployee{{ID: 1, Name: "A"}}, Entries: []ProductionAttendanceEntry{{EmployeeID: 1, WorkDate: "2026-09-14", Status: "working"}}}
	cmd := SaveProductionRosterCommand{Entries: []ProductionAttendanceEntry{{EmployeeID: 1, WorkDate: "2026-09-14", Status: "off"}}, Overrides: []ProductionWorkstationOverride{{WorkstationID: 9, WorkDate: "2026-09-14", EmployeeID: 1}}}
	normalized, released := NormalizeRosterLeaveOverrides(before, cmd)
	if len(normalized.Overrides) != 0 || len(released) != 1 {
		t.Fatalf("leave did not release override: %+v %+v", normalized, released)
	}
	before.Employees = nil
	normalized, released = NormalizeRosterLeaveOverrides(before, cmd)
	if len(normalized.Overrides) != 1 || len(released) != 0 {
		t.Fatal("disabled employee override silently removed")
	}
}
