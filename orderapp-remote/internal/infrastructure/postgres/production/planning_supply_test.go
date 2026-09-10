package production

import (
	app "orderapp/internal/application/production"
	domain "orderapp/internal/domain/production"
	"testing"
)

func TestManufacturingUpstreamTraceIncludesOnlyRootsNeedingItsNewOutput(t *testing.T) {
	expansion := manufacturingExpansion{Roots: map[string]app.ProductionPlanItem{"a": {ID: 1, OrderNos: "SO-A"}, "b": {ID: 2, OrderNos: "SO-B"}, "c": {ID: 3, OrderNos: "SO-C"}}, Plan: domain.ManufacturingPlan{Edges: []domain.ManufacturingSupplyEdge{
		{ConsumerKey: "a", SupplierKey: "middle", ShortageQty: 2}, {ConsumerKey: "middle", SupplierKey: "raw", ShortageQty: 4},
		{ConsumerKey: "b", SupplierKey: "raw", StockCoveredQty: 3}, {ConsumerKey: "c", SupplierKey: "other", ShortageQty: 2},
	}}}
	roots := manufacturingUpstreamDemandRoots(expansion, "raw")
	if len(roots) != 1 || roots[0].ID != 1 {
		t.Fatalf("upstream demand trace=%+v", roots)
	}
}
