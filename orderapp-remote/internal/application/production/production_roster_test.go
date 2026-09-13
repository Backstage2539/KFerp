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
