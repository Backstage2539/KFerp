package production

import (
	"os"
	"strings"
	"testing"
)

func TestPR629ComponentSourceContractIsWarehouseAndOwnerAware(t *testing.T) {
	schema, err := os.ReadFile("multilevel_schema.go")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := os.ReadFile("component_sources.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"production_plan_component_sources",
		"source_warehouse",
		"source_owner_customer_id",
		"available_g_snapshot",
		"available_units_snapshot",
	} {
		if !strings.Contains(string(schema), want) {
			t.Fatalf("component source schema missing %q", want)
		}
	}
	for _, want := range []string{
		"UpdateProductionPlanItemComponentSources",
		"validateProductionPlanComponentSourcesAtSubmitTx",
		"warehouseCustomerID",
		"ownerCustomerID",
	} {
		if !strings.Contains(string(plan), want) {
			t.Fatalf("component source implementation missing %q", want)
		}
	}
}

func TestPR629ComponentSourceOwnerScope(t *testing.T) {
	tests := []struct {
		name                                   string
		planCustomer, warehouseCustomer, owner int64
		want                                   int64
		wantErr                                bool
	}{
		{name: "factory warehouse and factory owner", want: 0},
		{name: "customer warehouse fixes its owner", planCustomer: 74, warehouseCustomer: 74, want: 74},
		{name: "shared wip keeps plan customer batch", planCustomer: 74, owner: 74, want: 74},
		{name: "another customer warehouse rejected", planCustomer: 74, warehouseCustomer: 88, wantErr: true},
		{name: "another customer batch rejected", planCustomer: 74, owner: 88, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validateComponentSourceOwner(tt.planCustomer, tt.warehouseCustomer, tt.owner)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("owner = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestPR629DirectInventorySourceOwnsSubmitGapValidation(t *testing.T) {
	b, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	submitStart := strings.Index(src, "func (r Repository) SubmitProductionPlan")
	if submitStart < 0 {
		t.Fatal("SubmitProductionPlan not found")
	}
	submit := src[submitStart:]
	sourceValidation := strings.Index(submit, "validateProductionPlanComponentSourcesAtSubmitTx")
	gapValidation := strings.Index(submit, "countBlockingProductionPlanSupplyGapsTx")
	if sourceValidation < 0 || gapValidation < 0 || sourceValidation > gapValidation {
		t.Fatal("component source selection and selected-source shortage must be validated before residual supply gaps")
	}
	if !strings.Contains(src, "NOT EXISTS") || !strings.Contains(src, "production_plan_component_sources source") {
		t.Fatal("direct inventory components with a source warehouse must not remain blocked by legacy no-default-BOM gaps")
	}
}

func TestPR629CutoverIsGuardedAndRepeatable(t *testing.T) {
	b, err := os.ReadFile("pr629_cutover.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	for _, want := range []string{"pg_advisory_xact_lock", "IsoLevel: pgx.Serializable", "priorBackup", "inventory or reservation totals changed"} {
		if !strings.Contains(src, want) {
			t.Fatalf("cutover guard missing %q", want)
		}
	}
}

func TestPR629CutoverStillFindsLegacyDraftPlansAfterDemandRowsWereMarkedMigrated(t *testing.T) {
	b, err := os.ReadFile("pr629_cutover.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)
	if strings.Contains(src, `demandFilter = "d.migrated_at IS NULL"`) {
		t.Fatal("legacy draft/work-order discovery must include already-migrated demand rows so a rerun can repair an orphaned legacy plan")
	}
	for _, want := range []string{
		"d.migrated_at IS NULL OR plan.created_at <= d.migrated_at",
		"d.migrated_at IS NULL OR wo.created_at <= d.migrated_at",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("legacy cutover discovery missing pre-cutover boundary %q", want)
		}
	}
}

func TestPR629LegacyOrderNumberSplitPatternMatchesCommaSeparatedPlanOrders(t *testing.T) {
	if PR629LegacyOrderNoSplitPattern != `\s*[,，;；\n]+\s*` {
		t.Fatalf("legacy order number split pattern = %q", PR629LegacyOrderNoSplitPattern)
	}
	b, err := os.ReadFile("pr629_cutover.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "regexp_split_to_array(COALESCE(item.order_nos,''),$1)") {
		t.Fatal("cutover must bind the regex pattern as a query parameter instead of double-escaping it inside SQL")
	}
}
