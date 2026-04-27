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
		"PR-103",
		"DEV-103-01",
		"UT-103-01",
		"API-103-01",
		"REV-103-01",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("req_store.go missing smoke remediation seed %q", want)
		}
	}
}
