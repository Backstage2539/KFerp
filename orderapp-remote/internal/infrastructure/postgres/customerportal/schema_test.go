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
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("schema missing %q", want)
		}
	}
}
