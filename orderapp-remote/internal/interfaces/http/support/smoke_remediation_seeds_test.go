package support

import (
	"os"
	"strings"
	"testing"
)

func TestSmokeRemediationRequirementSeeds(t *testing.T) {
	src, err := os.ReadFile("internal/interfaces/http/support/req_store.go")
	if err != nil {
		src, err = os.ReadFile("req_store.go")
	}
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{
		"PR-104",
		"DEV-104-01",
		"UT-104-01",
		"API-104-01",
		"REV-104-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("req_store.go missing smoke remediation seed %q", want)
		}
	}
}
