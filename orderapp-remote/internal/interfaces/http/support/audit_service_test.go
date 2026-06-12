package support

import (
	"os"
	"strings"
	"testing"
)

func TestAuditUnifiedDelegatesAuditServiceAndTxHelper(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/support/audit_unified.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{
		"type AuditService = postgresinfra.AuditService",
		"type AuditEntry = postgresinfra.AuditEntry",
		"func AuditInsertTx",
		"postgresinfra.AuditInsertTx(ctx, tx",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("audit_unified.go missing %q", want)
		}
	}
	if strings.Contains(content, "INSERT INTO %s.audit_logs") {
		t.Fatal("support audit_unified.go should not own audit SQL")
	}
}

func TestInlineOrderAuditUsesApplicationRepositoryTransactionHelper(t *testing.T) {
	body, err := os.ReadFile("internal/infrastructure/postgres/sales/repository.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if strings.Contains(content, "AuditInsert(ctx, pool") {
		t.Fatal("inline order updates should write audit rows through the same transaction")
	}
	if !strings.Contains(content, "AuditInsertTx(ctx, tx") {
		t.Fatal("inline order updates should use AuditInsertTx")
	}
}
