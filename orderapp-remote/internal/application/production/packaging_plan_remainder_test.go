package production

import "testing"

func TestValidatePlanRemainderSelection(t *testing.T) {
	sources := []PlanRemainderSource{{SourceID: "issued:11", AvailableG: 500}, {SourceID: "issued:12", AvailableG: 250}}
	tests := []struct {
		name    string
		cmd     PlanRemainderCommand
		wantErr bool
	}{
		{"zero remains allowed", PlanRemainderCommand{PlanID: 1, ProductID: 2, ExpectedVersion: "v1", RequestID: "r1", TargetWarehouse: "wip", Sources: []PlanRemainderSelection{{SourceID: "issued:11", QuantityG: 0}, {SourceID: "issued:12", QuantityG: 0}}}, false},
		{"valid partial", PlanRemainderCommand{PlanID: 1, ProductID: 2, ExpectedVersion: "v1", RequestID: "r1", TargetWarehouse: "wip", Sources: []PlanRemainderSelection{{SourceID: "issued:11", QuantityG: 300}, {SourceID: "issued:12", QuantityG: 0}}}, false},
		{"missing", PlanRemainderCommand{PlanID: 1, ProductID: 2, ExpectedVersion: "v1", RequestID: "r1", TargetWarehouse: "wip", Sources: []PlanRemainderSelection{{SourceID: "issued:11", QuantityG: 1}}}, true},
		{"duplicate", PlanRemainderCommand{PlanID: 1, ProductID: 2, ExpectedVersion: "v1", RequestID: "r1", TargetWarehouse: "wip", Sources: []PlanRemainderSelection{{SourceID: "issued:11", QuantityG: 200}, {SourceID: "issued:11", QuantityG: 100}}}, true},
		{"over available", PlanRemainderCommand{PlanID: 1, ProductID: 2, ExpectedVersion: "v1", RequestID: "r1", TargetWarehouse: "wip", Sources: []PlanRemainderSelection{{SourceID: "issued:11", QuantityG: 501}}}, true},
		{"unknown component", PlanRemainderCommand{PlanID: 1, ProductID: 2, ExpectedVersion: "v1", RequestID: "r1", TargetWarehouse: "wip", Sources: []PlanRemainderSelection{{SourceID: "issued:13", QuantityG: 1}}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePlanRemainderSelection(tt.cmd, sources)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
