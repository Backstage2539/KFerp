package production

import (
	productionapp "orderapp/internal/application/production"
	"os"
	"strings"
	"testing"
)

func TestManufacturingPhase3ScheduleSchemaCreatesCapacityAndScheduleFields(t *testing.T) {
	src, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %s.work_center_capacity_calendar",
		"work_center TEXT NOT NULL DEFAULT ''",
		"work_date DATE NOT NULL",
		"shift_code TEXT NOT NULL DEFAULT ''",
		"available_minutes INT NOT NULL DEFAULT 0",
		"downtime_minutes INT NOT NULL DEFAULT 0",
		"work_center_capacity_calendar_uq",
		"planned_start_at TIMESTAMPTZ",
		"planned_end_at TIMESTAMPTZ",
		"shift_code TEXT NOT NULL DEFAULT ''",
		"assigned_to TEXT NOT NULL DEFAULT ''",
		"priority INT NOT NULL DEFAULT 0",
		"scheduling_note TEXT NOT NULL DEFAULT ''",
		"work_center TEXT NOT NULL DEFAULT ''",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("phase3 schedule schema missing %q", want)
		}
	}
}

func TestScheduleConflictsFromRowsAggregatesJobCardLoadAgainstCapacity(t *testing.T) {
	conflicts := scheduleConflictsFromRows(nil, []productionapp.JobCardRow{
		{ID: 1, WorkCenter: "印刷线", PlannedStartAt: "2026-06-13 09:00", PlannedEndAt: "2026-06-13 14:00", ShiftCode: "早班"},
		{ID: 2, WorkCenter: "印刷线", PlannedStartAt: "2026-06-13 14:00", PlannedEndAt: "2026-06-13 18:00", ShiftCode: "早班"},
	}, []productionapp.CapacityCalendarRow{
		{WorkCenter: "印刷线", WorkDate: "2026-06-13", ShiftCode: "早班", AvailableMinutes: 480, DowntimeMinutes: 30},
	})
	if len(conflicts) != 1 {
		t.Fatalf("conflicts len=%d want 1: %+v", len(conflicts), conflicts)
	}
	got := conflicts[0]
	if got.WorkCenter != "印刷线" || got.WorkDate != "2026-06-13" || got.ShiftCode != "早班" || got.LoadMinutes != 540 || got.CapacityMinutes != 450 {
		t.Fatalf("conflict = %+v", got)
	}
}
