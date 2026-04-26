package support

import (
	"os"
	"strings"
	"testing"
)

func TestAuditUnifiedOwnsAuditServiceAndTxHelper(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/support/audit_unified.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{"type AuditService struct", "type AuditEntry struct", "func AuditInsertTx"} {
		if !strings.Contains(content, want) {
			t.Fatalf("audit_unified.go missing %q", want)
		}
	}
}

func TestInlineOrderAuditUsesTransactionHelper(t *testing.T) {
	body, err := os.ReadFile("internal/interfaces/http/support/audit.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if strings.Contains(content, "AuditInsert(ctx, pool") {
		t.Fatal("inline order updates should write audit rows through the same transaction")
	}
	if !strings.Contains(content, "AuditInsertTx(ctx, tx") {
		t.Fatal("inline order updates should use auditInsertTx")
	}
}
