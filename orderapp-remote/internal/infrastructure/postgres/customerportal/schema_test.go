package customerportal

import (
	"os"
	"strings"
	"testing"
)

func TestCustomerPortalSchemaDefinesP0Tables(t *testing.T) {
	body, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %s.mini_users",
		"CREATE TABLE IF NOT EXISTS %s.mini_sessions",
		"CREATE TABLE IF NOT EXISTS %s.customer_portal_profiles",
		"CREATE TABLE IF NOT EXISTS %s.customer_portal_user_bindings",
		"CREATE TABLE IF NOT EXISTS %s.customer_service_capabilities",
		"mini_users_openid_uq",
		"customer_portal_user_bindings_user_customer_uq",
		"customer_service_capabilities_customer_code_uq",
		"jsonb_typeof(config_json) = 'object'",
		"ADD CONSTRAINT customer_service_capabilities_config_object_chk CHECK (jsonb_typeof(config_json) = 'object') NOT VALID",
		"VALIDATE CONSTRAINT customer_service_capabilities_config_object_chk",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema missing %q", want)
		}
	}
}

func TestCustomerPortalSchemaDefinesBusinessTables(t *testing.T) {
	body, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatalf("read schema.go: %v", err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS %s.direct_ship_import_batches",
		"CREATE TABLE IF NOT EXISTS %s.processing_job_requests",
		"CREATE TABLE IF NOT EXISTS %s.customer_fee_items",
		"CREATE TABLE IF NOT EXISTS %s.customer_settlement_batches",
		"direct_ship_import_batches_customer_idx",
		"processing_job_requests_customer_idx",
		"customer_fee_items_customer_status_idx",
		"customer_settlement_batches_customer_status_idx",
		"fee_type TEXT NOT NULL",
		"direct_ship_service",
		"processing",
		"shipping",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("business schema missing %q", want)
		}
	}
}
