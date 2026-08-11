package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev596SemiFinishedPackagingBomContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "domain", "bom", "bom_kind.go"): {
			"BomKindProduct",
			"BomKindSpecPackaging",
			"IsValidBomKind",
			"IsPackagingBomKind",
		},
		filepath.Join("internal", "domain", "catalog", "semi_finished.go"): {
			"IsWeightInventoryUnit",
			"ValidateSemiFinishedProduct",
			"SemiFinishedValidationInput",
		},
		filepath.Join("internal", "domain", "catalog", "semi_finished_validity.go"): {
			"CheckSemiFinishedPackagingValidity",
			"SemiFinishedPackagingResult",
		},
		filepath.Join("internal", "domain", "production", "two_stage.go"): {
			"ProductionStageSemiFinished",
			"ProductionStagePackaging",
			"WorkOrderDependencySemiToPackaging",
		},
		filepath.Join("internal", "domain", "production", "two_stage_planning.go"): {
			"CalculateSemiFinishedDemand",
			"DetermineTwoStagePlan",
		},
		filepath.Join("internal", "domain", "production", "spec_cost.go"): {
			"CalculateSpecStandardCost",
			"SpecStandardCostInput",
		},
		filepath.Join("internal", "infrastructure", "postgres", "stock", "schema.go"): {
			"semi_finished",
		},
		filepath.Join("internal", "infrastructure", "postgres", "production", "schema.go"): {
			"work_order_dependencies",
			"production_stage",
			"source_warehouse",
			"packaging_bom_version_id",
			"semi_finished_demand_qty",
		},
		filepath.Join("internal", "application", "bom", "service.go"): {
			"BomKind",
			"OutputIsSemiFinished",
			"validatePackagingBomDraftItem",
			"packaging BOM items must use fixed quantity",
			"packaging BOM items must be materials",
			"semi_finished_packaging_required",
			"CheckSemiFinishedPackagingValidity",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"IsSemiFinished",
			"semi-finished product must reference a sales spec template",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "schema.go"): {
			"bom_kind",
			"output_is_semi_finished",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"is_semi_finished",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"): {
			"validatePackagingBomVersionItems",
			"pb.bom_kind",
			"pb.output_is_semi_finished",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"is_semi_finished",
		},
		filepath.Join("internal", "interfaces", "http", "bom", "bom_api.go"): {
			"BomKind",
			"OutputIsSemiFinished",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"IsSemiFinished",
			"is_semi_finished",
		},
		filepath.Join("internal", "interfaces", "http", "support", "req_store.go"): {
			"PR-596-SEMI-FINISHED-PACKAGING-BOM",
			"DEV-596-BOM-KIND-EXTENSION",
			"DEV-596-PRODUCT-SEMI-FINISHED",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-596 marker %q", rel, want)
			}
		}
	}
}
