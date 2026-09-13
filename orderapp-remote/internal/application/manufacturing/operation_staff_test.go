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
		{"active operation no longer configures staff", SaveManufacturingOperationCommand{Name: "包装", Status: "active"}, true},
		{"inactive can wait for configuration", SaveManufacturingOperationCommand{Name: "包装", Status: "inactive"}, true},
		{"legacy eligible staff points to workstation", SaveManufacturingOperationCommand{Name: "包装", Status: "active", EligibleEmployeeIDs: []int64{1, 2, 3}, DefaultEmployeeID: 1}, false},
		{"legacy collaborators point to workstation", SaveManufacturingOperationCommand{Name: "包装", Status: "active", DefaultCollaboratorIDs: []int64{2}}, false},
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
