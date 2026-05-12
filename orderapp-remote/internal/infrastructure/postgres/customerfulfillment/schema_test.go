package customerfulfillment

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCustomerFulfillmentSchemaDefinesRequiredTables(t *testing.T) {
	src := string(readCustomerFulfillmentRepoFile(t, "internal/infrastructure/postgres/customerfulfillment/schema.go"))
	for _, want := range []string{
		"customer_fulfillment_import_batches",
		"customer_fulfillment_import_rows",
		"customer_custody_items",
		"customer_custody_ledger_entries",
		"customer_custody_balances",
		"customer_processing_work_orders",
		"customer_processing_work_order_inputs",
		"customer_processing_packaging_jobs",
		"customer_inventory_conversion_jobs",
		"customer_direct_ship_import_orders",
		"customer_direct_ship_import_order_items",
		"customer_billing_rules",
		"customer_fulfillment_import_batches_customer_type_sha_idx",
		"customer_fulfillment_import_rows_batch_type_status_idx",
		"customer_custody_items_customer_type_external_code_idx",
		"customer_direct_ship_import_orders_customer_external_idx",
		"active_customer_bindings",
		"active_employee_bindings",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("schema.go missing %q", want)
		}
	}
}

func TestCustomerFulfillmentSchemaIsRegisteredInAppMain(t *testing.T) {
	src := string(readCustomerFulfillmentRepoFile(t, "internal/appmain/schema_setup.go"))
	for _, want := range []string{
		`postgrescustomerfulfillment`,
		`Name: "customerfulfillment"`,
		`postgrescustomerfulfillment.EnsureSchema`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("schema_setup.go missing %q", want)
		}
	}
}

func readCustomerFulfillmentRepoFile(t *testing.T, path string) []byte {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		candidate := filepath.Join(wd, "go.mod")
		if _, err := os.Stat(candidate); err == nil {
			b, err := os.ReadFile(filepath.Join(wd, path))
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatalf("go.mod not found while resolving %s", path)
		}
		wd = parent
	}
}
