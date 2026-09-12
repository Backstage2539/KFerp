package manufacturing

import (
	"context"
	"testing"
)

func TestOperationStaffConfiguration(t *testing.T) {
	svc := NewService(&fakeRepo{})
	tests := []struct {
		name  string
		cmd   SaveManufacturingOperationCommand
		valid bool
	}{
		{"active needs eligible staff", SaveManufacturingOperationCommand{Name: "包装", Status: "active"}, false},
		{"inactive can wait for configuration", SaveManufacturingOperationCommand{Name: "包装", Status: "inactive"}, true},
		{"one lead", SaveManufacturingOperationCommand{Name: "包装", Status: "active", EligibleEmployeeIDs: []int64{1, 2, 3}, DefaultEmployeeID: 1}, true},
		{"collaborators retired", SaveManufacturingOperationCommand{Name: "包装", Status: "active", EligibleEmployeeIDs: []int64{1, 2}, DefaultEmployeeID: 1, DefaultCollaboratorIDs: []int64{2}}, false},
		{"lead must be eligible", SaveManufacturingOperationCommand{Name: "包装", EligibleEmployeeIDs: []int64{2}, DefaultEmployeeID: 1}, false},
		{"lead cannot collaborate twice", SaveManufacturingOperationCommand{Name: "包装", EligibleEmployeeIDs: []int64{1, 2}, DefaultEmployeeID: 1, DefaultCollaboratorIDs: []int64{1}}, false},
		{"collaborators must be eligible", SaveManufacturingOperationCommand{Name: "包装", EligibleEmployeeIDs: []int64{1, 2}, DefaultEmployeeID: 1, DefaultCollaboratorIDs: []int64{3}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.SaveManufacturingOperation(context.Background(), tt.cmd)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v err=%v", tt.valid, err)
			}
		})
	}
}
