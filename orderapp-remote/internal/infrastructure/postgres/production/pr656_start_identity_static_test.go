package production

import (
	"os"
	"strings"
	"testing"
)

func TestWorkOrderStartDoesNotBlockAnotherWorkOrderForTheSameOrderReference(t *testing.T) {
	text, err := os.ReadFile("production_plan.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(text)
	start := strings.Index(source, "func (r Repository) startWorkOrderTx")
	end := strings.Index(source[start:], "func markProcessingWorkOrderRunningTx")
	if start < 0 || end < 0 {
		t.Fatal("startWorkOrderTx source not found")
	}
	if strings.Contains(source[start:start+end], "ensureStartRefsNotRunningTx") {
		t.Fatal("work-order start still blocks by shared sales-order reference")
	}
}
