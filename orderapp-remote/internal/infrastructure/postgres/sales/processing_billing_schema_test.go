package sales

import (
	"os"
	"strings"
	"testing"
)

func TestProcessingBillingSchemaKeepsPublishedTemplateVersionsAndBillSnapshotsImmutable(t *testing.T) {
	body, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"outsource_template_versions",
		"outsource_template_rules",
		"outsource_template_rules_fee_type_check",
		"processing_billing_runs",
		"processing_billing_work_orders",
		"processing_billing_line_snapshots",
		"customer_fee_items ADD COLUMN IF NOT EXISTS processing_billing_run_id",
		"customer_settlement_batches ADD COLUMN IF NOT EXISTS processing_billing_run_id",
		"run_kind TEXT NOT NULL DEFAULT 'standard'",
		"source_billing_run_id BIGINT NOT NULL DEFAULT 0",
		"status IN ('confirmed','paid','reversed')",
		"processing_billing_work_orders_standard_uq",
		"line_kind TEXT NOT NULL DEFAULT 'calculated'",
		"processing_billing_line_snapshots_calculated_uq",
		"processing_billing_runs_reversal_source_uq",
		"processing_billing_runs_adjustment_request_uq",
		"customer_fee_items_fee_type_check",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("processing billing schema missing %q", want)
		}
	}
}

func TestProcessingBillingRepositoryUsesCompletedCustomerWorkOrdersAndExcludesCustomerOwnedMaterial(t *testing.T) {
	body, err := os.ReadFile("processing_billing.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"customer_processing_production_demands",
		"wo.status='completed'",
		"customer_processing_material_reservations",
		"consumed_g",
		"consumed_units",
		"returned_g",
		"returned_units",
		"source_owner_type='factory'",
		"source_customer_id=0",
		"material_consumption_logs",
		"completion_no",
		"MAX(input_g)",
		"actual_input_qty",
		"finished_stock_batch_id",
		"stock_batches",
		"material_batch_code",
		"FOR UPDATE",
		"customer_fee_items",
		"customer_settlement_batches",
		"AuditInsertTx",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("processing billing repository missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"NULLIF(ri.input_g,0),(",
		"WHEN status='consumed' THEN GREATEST(reserved_g,required_g)",
		"WHEN status='consumed' THEN GREATEST(reserved_units,required_units)",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("processing billing repository still contains forbidden legacy fallback %q", forbidden)
		}
	}
}
