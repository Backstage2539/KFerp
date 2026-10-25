package production

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletionSchemaAllowsMoreThanOnePostedEventPerJobCard(t *testing.T) {
	path := filepath.Join("operation_completion_schema.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "DROP INDEX IF EXISTS %[1]s.production_operation_events_completion_uq") {
		t.Fatal("existing one-completion-per-card unique index must be removed for batch completion")
	}
	if strings.Contains(text, "CREATE UNIQUE INDEX IF NOT EXISTS production_operation_events_completion_uq") {
		t.Fatal("schema still enforces one completion event per job card")
	}
	if !strings.Contains(text, "production_operation_events_completion_idx ON %[1]s.production_operation_events(job_card_id,id)") {
		t.Fatal("batch completion history needs a non-unique lookup index")
	}
}
