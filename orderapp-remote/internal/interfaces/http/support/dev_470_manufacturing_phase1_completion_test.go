package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev470ManufacturingPhase1CompletionContracts(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-470-MANUFACTURING-PHASE1-COMPLETION",
		"DEV-470-DEFAULT-PRODUCTION-BOM",
		"DEV-470-ROUTING-MASTER-DATA",
		"DEV-470-WORK-ORDER-FREEZE",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing PR-470 workflow marker %q", want)
		}
	}

	bomAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "bom", "bom_api.go")))
	for _, want := range []string{
		"/api/products/:id/default-production-bom",
		"production_bom_id",
		"production_bom_version_id",
	} {
		if !strings.Contains(bomAPI, want) {
			t.Fatalf("BOM API missing default production BOM marker %q", want)
		}
	}

	bomService := string(readOrderAppFileForTest(t, filepath.Join("internal", "application", "bom", "service.go")))
	for _, want := range []string{
		"CanSetDefault",
		"CurrentPublishedVersionID",
		"CurrentPublishedVersionNo",
	} {
		if !strings.Contains(bomService, want) {
			t.Fatalf("BOM service usage DTO missing marker %q", want)
		}
	}

	bomRepository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go")))
	for _, want := range []string{
		"set_default_production_bom",
		"product_production_configs",
		"product_production_bom_bindings",
		"pb.output_product_id=$",
		"pb.status='active'",
		"v.status='published'",
		"current_published_version_id",
		"current_published_version_no",
		"can_set_default",
	} {
		if !strings.Contains(bomRepository, want) {
			t.Fatalf("BOM repository missing default BOM marker %q", want)
		}
	}

	manufacturingSchema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "manufacturing", "schema.go")))
	for _, want := range []string{
		"manufacturing_operations",
		"manufacturing_workstations",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS operation_id",
		"ALTER TABLE %[1]s.process_route_operations ADD COLUMN IF NOT EXISTS workstation_id",
	} {
		if !strings.Contains(manufacturingSchema, want) {
			t.Fatalf("manufacturing schema missing operation/workstation marker %q", want)
		}
	}

	manufacturingAPI := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "manufacturing", "api.go")))
	for _, want := range []string{
		"/api/manufacturing-operations",
		"/api/manufacturing-workstations",
	} {
		if !strings.Contains(manufacturingAPI, want) {
			t.Fatalf("manufacturing API missing master-data route %q", want)
		}
	}

	processTemplatesView := string(readOrderAppFileForTest(t, filepath.Join("frontend-vue-shell", "src", "views", "ProcessTemplatesView.vue")))
	for _, want := range []string{
		"/api/manufacturing-operations",
		"/api/manufacturing-workstations",
		"operation_id",
		"workstation_id",
	} {
		if !strings.Contains(processTemplatesView, want) {
			t.Fatalf("process template view missing master-data selector marker %q", want)
		}
	}

	workOrderRepository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "production", "work_order.go")))
	for _, want := range []string{
		"loadProcessRouteSnapshotForWorkOrderTx",
		"process_route_operations",
		"operation_id",
		"workstation_id",
		"COALESCE(NULLIF(ppc.production_bom_version_id,0), pbb.bom_version_id, output_bv.id, 0)",
	} {
		if !strings.Contains(workOrderRepository, want) {
			t.Fatalf("work order repository missing phase1 freeze marker %q", want)
		}
	}
}
