package appmain

import (
	"os"
	"strings"
	"testing"
)

func TestSchemaSetupInitializesCompanyBeforeEmployeeDependentModules(t *testing.T) {
	body, err := os.ReadFile("internal/appmain/schema_setup.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	company := strings.Index(src, `Name: "company"`)
	support := strings.Index(src, `Name: "support"`)
	authz := strings.Index(src, `Name: "authz"`)
	if company < 0 || support < 0 || authz < 0 {
		t.Fatalf("schema steps missing company/support/authz: company=%d support=%d authz=%d", company, support, authz)
	}
	if !(company < support && company < authz) {
		t.Fatalf("company schema must run before support/authz because both reference employees")
	}
}

func TestSchemaSetupCreatesConfiguredSchemaBeforeModuleMigrations(t *testing.T) {
	body, err := os.ReadFile("internal/appmain/schema_setup.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(body)
	for _, want := range []string{
		"CREATE SCHEMA IF NOT EXISTS",
		"pgx.Identifier{schema}.Sanitize()",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("schema setup should create configured schema before module migrations; missing %q", want)
		}
	}
}
