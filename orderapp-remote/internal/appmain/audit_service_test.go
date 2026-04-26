package appmain

import (
	"os"
	"strings"
	"testing"
)

func TestAuditUnifiedOwnsAuditServiceAndTxHelper(t *testing.T) {
	body, err := os.ReadFile("internal/appmain/audit_unified.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	for _, want := range []string{"type AuditService struct", "type AuditEntry struct", "func auditInsertTx"} {
		if !strings.Contains(content, want) {
			t.Fatalf("audit_unified.go missing %q", want)
		}
	}
}

func TestInlineOrderAuditUsesTransactionHelper(t *testing.T) {
	body, err := os.ReadFile("internal/appmain/audit.go")
	if err != nil {
		t.Fatal(err)
	}
	content := string(body)
	if strings.Contains(content, "auditInsert(ctx, pool") {
		t.Fatal("inline order updates should write audit rows through the same transaction")
	}
	if !strings.Contains(content, "auditInsertTx(ctx, tx") {
		t.Fatal("inline order updates should use auditInsertTx")
	}
}
