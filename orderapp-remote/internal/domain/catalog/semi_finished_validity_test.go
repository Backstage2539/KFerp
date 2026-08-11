package catalog

import (
	"reflect"
	"testing"
)

func TestCheckSemiFinishedPackagingValidity(t *testing.T) {
	specs := []SalesSpecForPackagingCheck{
		{SpecKey: "bag-227g", Active: true},
		{SpecKey: "bag-454g", Active: true},
		{SpecKey: "bag-2500g", Active: true},
		{SpecKey: "bag-5000g", Active: false},
	}
	refs := []PackagingBomRefForCheck{
		{SpecKey: "bag-227g", IsValid: true},
		{SpecKey: "bag-454g", IsValid: true},
	}

	result := CheckSemiFinishedPackagingValidity(specs, refs)
	if result.Valid {
		t.Fatalf("should be invalid when bag-2500g lacks packaging BOM")
	}
	if !reflect.DeepEqual(result.MissingSpecs, []string{"bag-2500g"}) {
		t.Fatalf("missing specs = %v, want [bag-2500g]", result.MissingSpecs)
	}

	refs2 := append(refs, PackagingBomRefForCheck{SpecKey: "bag-2500g", IsValid: true})
	result2 := CheckSemiFinishedPackagingValidity(specs, refs2)
	if !result2.Valid {
		t.Fatalf("should be valid when all active specs have valid packaging BOM, missing=%v", result2.MissingSpecs)
	}

	refs3 := append(refs, PackagingBomRefForCheck{SpecKey: "bag-2500g", IsValid: false})
	result3 := CheckSemiFinishedPackagingValidity(specs, refs3)
	if result3.Valid {
		t.Fatalf("should be invalid when bag-2500g packaging BOM is not valid (unpublished)")
	}
	if !reflect.DeepEqual(result3.MissingSpecs, []string{"bag-2500g"}) {
		t.Fatalf("missing specs = %v, want [bag-2500g]", result3.MissingSpecs)
	}

	result4 := CheckSemiFinishedPackagingValidity(nil, nil)
	if !result4.Valid {
		t.Fatalf("empty specs should be valid")
	}
}
