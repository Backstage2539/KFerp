package postgres

import (
	"strings"
	"testing"
)

func TestMigrationLedgerDDLDefinesVersionedSchemaTable(t *testing.T) {
	ddl := MigrationLedgerDDL("erp")

	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS erp.schema_migrations",
		"version TEXT PRIMARY KEY",
		"checksum TEXT NOT NULL",
		"applied_at TIMESTAMPTZ NOT NULL DEFAULT now()",
	} {
		if !strings.Contains(ddl, want) {
			t.Fatalf("ledger DDL missing %q in %s", want, ddl)
		}
	}
}

func TestValidateMigrationsRejectsInvalidOrUnsortedEntries(t *testing.T) {
	if err := ValidateMigrations([]Migration{
		{Version: "202606120001_p2_architecture", Checksum: "abc"},
		{Version: "202606120002_next", Checksum: "def"},
	}); err != nil {
		t.Fatalf("ValidateMigrations() error = %v", err)
	}
	if err := ValidateMigrations([]Migration{{Version: "", Checksum: "abc"}}); err == nil {
		t.Fatal("expected empty version to fail")
	}
	if err := ValidateMigrations([]Migration{
		{Version: "202606120002_next", Checksum: "def"},
		{Version: "202606120001_p2_architecture", Checksum: "abc"},
	}); err == nil {
		t.Fatal("expected unsorted migrations to fail")
	}
}
