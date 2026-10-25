package production

import (
	"math"
	"testing"

	app "orderapp/internal/application/production"
)

func TestIncompleteRoastingRecipeRefreshRequiresCompletePublishedVersionAndUnusedWorkOrder(t *testing.T) {
	for _, tc := range []struct {
		name              string
		frozen, published float64
		eligible          bool
		want              bool
	}{
		{name: "adopts complete published recipe", frozen: 70, published: 100, eligible: true, want: true},
		{name: "keeps incomplete published recipe blocked", frozen: 70, published: 90, eligible: true},
		{name: "does not rewrite a work order with activity", frozen: 70, published: 100, eligible: false},
		{name: "does not replace an already complete frozen recipe", frozen: 100, published: 100, eligible: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldRefreshIncompleteRoastingRecipe(tc.frozen, tc.published, tc.eligible); got != tc.want {
				t.Fatalf("refresh = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRoastingRefreshKeepsWholeNeedVisibleButReservesOnlyCurrentBatch(t *testing.T) {
	for _, tc := range []struct {
		name                               string
		planned, reserved, consumed, batch int64
		wantRequired, wantReserved         int64
	}{
		{name: "first batch may proceed before future stock arrives", planned: 4200, batch: 2100, wantRequired: 4200, wantReserved: 2100},
		{name: "preserves existing reservation and adds only current deficit", planned: 4200, reserved: 2820, batch: 2100, wantRequired: 4200, wantReserved: 2820},
		{name: "later batch reserves cumulative consumption plus current need", planned: 4200, reserved: 2820, consumed: 2100, batch: 2100, wantRequired: 4200, wantReserved: 4200},
		{name: "actual consumption above plan remains required", planned: 4200, consumed: 2100, batch: 2400, wantRequired: 4500, wantReserved: 4500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			required, reserved := roastingReservationTargets(tc.planned, tc.reserved, tc.consumed, tc.batch)
			if required != tc.wantRequired || reserved != tc.wantReserved {
				t.Fatalf("reservation target = required %d / reserved %d, want %d / %d", required, reserved, tc.wantRequired, tc.wantReserved)
			}
		})
	}
}

func TestRoastingRefreshUsesOnlyEffectiveRecipeComponents(t *testing.T) {
	effective := []materialSnapshotRow{
		{MaterialID: 1, MaterialName: "孟连水洗A", Unit: "kg", RatioPct: 70},
		{MaterialID: 2, MaterialName: "孟连水洗5T", Unit: "kg", RatioPct: 15},
		{MaterialID: 3, MaterialName: "巴布亚之光", Unit: "kg", RatioPct: 15},
	}
	keys := effectiveRoastingMaterialKeys(effective)
	if _, ok := keys[operationMaterialKey("material", 1, 0, 0)]; !ok {
		t.Fatal("effective BOM should retain matching component reservation")
	}
	if shouldReleaseStaleRoastingReservation(operationMaterialKey("material", 99, 0, 0), "roast", "roast", keys) != true {
		t.Fatal("removed roasting component reservation should be released")
	}
	if shouldReleaseStaleRoastingReservation(operationMaterialKey("material", 99, 0, 0), "pack", "roast", keys) {
		t.Fatal("reservation bound to another operation must be retained")
	}
	uses, err := app.ResolveRoastingMaterialUses([]app.OperationMaterialUse{
		{MaterialID: 1, Name: "孟连水洗A", Unit: "kg", RatioPct: 70},
		{MaterialID: 2, Name: "孟连水洗5T", Unit: "kg", RatioPct: 15},
		{MaterialID: 3, Name: "巴布亚之光", Unit: "kg", RatioPct: 15},
	}, 3, "kg")
	if err != nil {
		t.Fatal(err)
	}
	want := []float64{2.1, 0.45, 0.45}
	for i, use := range uses {
		if math.Abs(use.Qty-want[i]) > 1e-9 {
			t.Fatalf("ingredient %s quantity = %v, want %v", use.Name, use.Qty, want[i])
		}
	}
}

func TestNewRoastingBOMComponentsInheritOnlyUniqueRoastingRouteBinding(t *testing.T) {
	route := processTemplateSnapshot{Operations: []processSnapshotOperation{
		{Operation: "咖啡烘焙+除石", ExecutionJSON: `{"version":1,"node_key":"roast","source_warehouse":"wip"}`},
		{Operation: "包装", ExecutionJSON: `{"version":1,"node_key":"pack","source_warehouse":"packaging"}`},
	}}
	got, ok := inferredRoastingBinding(route)
	if !ok || got != (app.OperationMaterialBinding{NodeKey: "roast", SourceWarehouse: "wip"}) {
		t.Fatalf("inferred binding = %+v, %v", got, ok)
	}
	if got := roastingRecipeBinding("material:new", map[string]app.OperationMaterialBinding{"material:new": {NodeKey: "explicit", SourceWarehouse: "raw"}}, route); got.NodeKey != "explicit" || got.SourceWarehouse != "raw" {
		t.Fatalf("explicit binding must win: %+v", got)
	}
	ambiguous := processTemplateSnapshot{Operations: append(route.Operations, processSnapshotOperation{Operation: "二次烘焙", ExecutionJSON: `{"version":1,"node_key":"roast-2"}`})}
	if _, ok := inferredRoastingBinding(ambiguous); ok {
		t.Fatal("must not guess a binding when the route has multiple roasting nodes")
	}
}
