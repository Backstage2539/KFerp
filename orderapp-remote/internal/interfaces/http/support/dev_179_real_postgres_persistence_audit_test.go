package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRealPostgresPersistenceAuditEvidenceExists(t *testing.T) {
	customerFulfillmentTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "customerfulfillment", "repository_test.go")))
	salesSchema := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "schema.go")))
	salesSchemaTest := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "schema_test.go")))
	acceptance := string(readOrderAppFileForTest(t, filepath.Join("..", "docs", "acceptance", "2026-05-13-three-template-business-audit.md")))

	for _, want := range []string{
		"postgrescompany",
		"company.EnsureSchema",
		"account_type",
		"channel_customer",
	} {
		if !strings.Contains(customerFulfillmentTest, want) {
			t.Fatalf("customer fulfillment PostgreSQL test missing marker %q", want)
		}
	}
	for _, want := range []string{
		"ADD COLUMN IF NOT EXISTS ship_tracking_no",
		"regexp_split_to_table(COALESCE(o.ship_tracking_no",
	} {
		if !strings.Contains(salesSchema, want) {
			t.Fatalf("sales schema missing marker %q", want)
		}
		if !strings.Contains(salesSchemaTest, want) {
			t.Fatalf("sales schema source test missing marker %q", want)
		}
	}
	for _, want := range []string{
		"PR-179-REAL-POSTGRES-PERSISTENCE-AUDIT",
		"go test ./internal/infrastructure/postgres/... -count=1",
		"customerportal",
		"customerfulfillment",
		"sales",
		"当前结论：未完成",
	} {
		if !strings.Contains(acceptance, want) {
			t.Fatalf("acceptance evidence missing marker %q", want)
		}
	}
}

func TestRealPostgresPersistenceAuditRequirementSeedsExist(t *testing.T) {
	store := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-179-REAL-POSTGRES-PERSISTENCE-AUDIT",
		"DEV-179-REAL-POSTGRES-PERSISTENCE-AUDIT",
		"UT-179-REAL-POSTGRES-PERSISTENCE-AUDIT",
		"API-179-REAL-POSTGRES-PERSISTENCE-AUDIT",
		"REV-179-REAL-POSTGRES-PERSISTENCE-AUDIT",
	} {
		if !strings.Contains(store, want) {
			t.Fatalf("req_store.go missing %q", want)
		}
	}
}
