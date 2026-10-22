package bom

import (
	"reflect"
	"testing"
)

func rolloutFixture(weight float64) TemplateRolloutSpec {
	return TemplateRolloutSpec{Key: "bag", Variant: ProductionBomDraftVariant{SpecKey: "bag", Name: "454g", InventoryUnit: "袋", IsDefault: true, LogisticsWeightG: &weight, ProcessRouteID: 1, Items: []ProductionBomDraftItem{{MaterialID: 7, ComponentType: "material", ConsumeUnit: "kg", QtyPerUnit: .454}}}}
}
func TestTemplateRolloutThreeWayMerge(t *testing.T) {
	old, next, current := rolloutFixture(460), rolloutFixture(470), rolloutFixture(460)
	current.Variant.ProcessRouteID = 9
	current.Variant.BomSpecID = 69
	result := MergeTemplateRollout([]TemplateRolloutSpec{old}, []TemplateRolloutSpec{next}, []TemplateRolloutSpec{current})
	if len(result.Conflicts) != 0 || len(result.Specs) != 1 || *result.Specs[0].Variant.LogisticsWeightG != 470 || result.Specs[0].Variant.ProcessRouteID != 9 || result.Specs[0].Variant.BomSpecID != 69 {
		t.Fatalf("merge: %+v", result)
	}
	current = rolloutFixture(480)
	result = MergeTemplateRollout([]TemplateRolloutSpec{old}, []TemplateRolloutSpec{next}, []TemplateRolloutSpec{current})
	if len(result.Conflicts) == 0 {
		t.Fatal("concurrent weight edits must conflict")
	}
	result = MergeTemplateRollout([]TemplateRolloutSpec{old}, nil, []TemplateRolloutSpec{current})
	if len(result.Conflicts) == 0 {
		t.Fatal("deleting local modification must conflict")
	}
	result = MergeTemplateRollout([]TemplateRolloutSpec{old}, nil, []TemplateRolloutSpec{old})
	if len(result.Conflicts) != 0 || len(result.Specs) != 0 {
		t.Fatal("unchanged removed spec must disappear")
	}
	current = next
	result = MergeTemplateRollout([]TemplateRolloutSpec{old}, []TemplateRolloutSpec{next}, []TemplateRolloutSpec{current})
	if len(result.Conflicts) != 0 {
		t.Fatal("same new value is not conflict")
	}
	local := rolloutFixture(800)
	local.Key = "local:8"
	local.Variant.BomSpecID = 8
	result = MergeTemplateRollout([]TemplateRolloutSpec{old}, []TemplateRolloutSpec{next}, []TemplateRolloutSpec{old, local})
	if !reflect.DeepEqual(result.Specs[1], local) {
		t.Fatal("local spec lost")
	}
	next = old
	next.Variant.InventoryUnit = "盒"
	result = MergeTemplateRollout([]TemplateRolloutSpec{old}, []TemplateRolloutSpec{next}, []TemplateRolloutSpec{old})
	if len(result.Conflicts) > 0 || result.Specs[0].Variant.InventoryUnit != "盒" {
		t.Fatal("unit update failed")
	}
}
