package manufacturing

import "testing"

func TestValidateWorkstationStaffRequiresPrimaryAndUniqueOrderedBackups(t *testing.T) {
	if err := validateWorkstationStaff(SaveManufacturingWorkstationCommand{Status: "active"}); err == nil {
		t.Fatal("active workstation without a primary owner was accepted")
	}
	if err := validateWorkstationStaff(SaveManufacturingWorkstationCommand{Status: "active", PrimaryEmployeeID: 7, BackupEmployeeIDs: []int64{8, 7}}); err == nil {
		t.Fatal("primary repeated as backup was accepted")
	}
	if err := validateWorkstationStaff(SaveManufacturingWorkstationCommand{Status: "active", PrimaryEmployeeID: 7, BackupEmployeeIDs: []int64{8, 9}}); err != nil {
		t.Fatalf("valid workstation staff rejected: %v", err)
	}
	if err := validateWorkstationStaff(SaveManufacturingWorkstationCommand{Status: "inactive"}); err != nil {
		t.Fatalf("inactive workstation should allow incomplete staff: %v", err)
	}
}
