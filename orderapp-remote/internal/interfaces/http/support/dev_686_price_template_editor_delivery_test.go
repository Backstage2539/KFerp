package support

import (
	"path/filepath"
	"regexp"
	"testing"
)

func TestPriceTemplateEditorDeliveryRequirementSeeds(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_product", code: "PR-686-PRICE-TEMPLATE-EDITOR-TRIAL", status: "review"},
		{table: "req_dev", code: "DEV-746-PRICE-RULE-DRAFT", status: "done"},
		{table: "req_dev", code: "DEV-747-PRICE-EDITOR-TRIAL", status: "done"},
		{table: "req_dev", code: "DEV-748-PRICE-EDITOR-DELIVERY", status: "done"},
	} {
		pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(row.table) + `"[^\n]*code: "` + regexp.QuoteMeta(row.code) + `"[^\n]*status: "` + regexp.QuoteMeta(row.status) + `"[^\n]*\},[\t ]*$`)
		if !pattern.MatchString(reqStore) {
			t.Errorf("req_store.go must seed %s as %s", row.code, row.status)
		}
	}
	if !regexp.MustCompile(`(?m)^\s*\{table: "req_dev"[^\n]*code: "DEV-748-PRICE-EDITOR-DELIVERY"[^\n]*evidence: "[^"]*57c5b299[^"]*Van business acceptance pending[^"]*"`).MatchString(reqStore) {
		t.Error("DEV-748 evidence must record the deployed development commit and pending business acceptance")
	}
}
