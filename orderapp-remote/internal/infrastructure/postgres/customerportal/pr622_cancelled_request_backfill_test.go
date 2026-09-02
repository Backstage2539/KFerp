package customerportal

import (
	"os"
	"strings"
	"testing"
)

func TestProcessingRequestStartupBackfillSkipsRetiredRequests(t *testing.T) {
	source, err := os.ReadFile("schema.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, marker := range []string{
		`lower(COALESCE(r.status,'')) NOT IN ('completed','cancelled','closed')`,
		`SELECT 1 FROM %[1]s.processing_job_request_items item WHERE item.request_id=r.id`,
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("startup backfill must not recreate retired processing request items; missing %q", marker)
		}
	}
}
