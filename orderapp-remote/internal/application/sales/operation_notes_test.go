package sales

import "testing"

func TestNormalizeOrderOperationNotes(t *testing.T) {
	notes := []OrderOperationNote{{OutputType: "material", OutputID: 12, RouteID: 3, StepID: 7, OperationID: 8, OperationName: " 烘焙 ", Note: " 浅烘 "}}
	got, err := NormalizeOrderOperationNotes(notes)
	if err != nil || len(got) != 1 || got[0].Note != "浅烘" || got[0].OperationName != "烘焙" {
		t.Fatalf("unexpected normalization: %+v %v", got, err)
	}
	if _, err = NormalizeOrderOperationNotes(append(notes, notes...)); err == nil {
		t.Fatal("duplicate steps must be rejected")
	}
	notes[0].OutputID = 0
	if _, err = NormalizeOrderOperationNotes(notes); err == nil {
		t.Fatal("missing target must be rejected")
	}
}
