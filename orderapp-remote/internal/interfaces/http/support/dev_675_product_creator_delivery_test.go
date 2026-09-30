package support

import (
	"path/filepath"
	"regexp"
	"testing"
)

func TestProductCreatorV3DeliveryRequirementSeeds(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_product", code: "PR-675-PRODUCT-CREATOR-VARIABLES-INPUTS", status: "review"},
		{table: "req_dev", code: "DEV-675-PC-GRAPH-V3", status: "done"},
		{table: "req_dev", code: "DEV-675-PC-NAME-VARIABLES", status: "done"},
		{table: "req_dev", code: "DEV-675-PC-BOM-INPUTS", status: "done"},
		{table: "req_dev", code: "DEV-675-PC-RUN-UX", status: "done"},
		{table: "req_dev", code: "DEV-675-PC-DOCS-DELIVERY", status: "done"},
	} {
		pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(row.table) + `"[^\n]*code: "` + regexp.QuoteMeta(row.code) + `"[^\n]*status: "` + regexp.QuoteMeta(row.status) + `"[^\n]*\},[\t ]*$`)
		if !pattern.MatchString(reqStore) {
			t.Errorf("req_store.go must seed %s as %s", row.code, row.status)
		}
	}
}

func TestProductCreatorV4SpecTemplateDeliveryRequirementSeeds(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_product", code: "PR-676-PRODUCT-CREATOR-BOM-SPEC-TEMPLATE", status: "review"},
		{table: "req_dev", code: "DEV-707-PC-BOM-SPEC-TEMPLATE", status: "done"},
		{table: "req_dev", code: "DEV-708-PC-TEMPLATE-EXECUTION", status: "done"},
		{table: "req_dev", code: "DEV-709-PC-TEMPLATE-UI-DOCS-DELIVERY", status: "done"},
	} {
		pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(row.table) + `"[^\n]*code: "` + regexp.QuoteMeta(row.code) + `"[^\n]*status: "` + regexp.QuoteMeta(row.status) + `"[^\n]*\},[\t ]*$`)
		if !pattern.MatchString(reqStore) {
			t.Errorf("req_store.go must seed %s as %s", row.code, row.status)
		}
	}
}

func TestProductCreatorV5ProcessAndNamePreviewFollowupSeeds(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table  string
		code   string
		status string
	}{
		{table: "req_dev", code: "DEV-710-PC-V5-ROUTE-CONNECTION", status: "done"},
		{table: "req_dev", code: "DEV-711-PC-V5-ROUTE-OVERRIDE", status: "done"},
		{table: "req_dev", code: "DEV-712-PC-NAME-LIVE-PREVIEW", status: "done"},
		{table: "req_dev", code: "DEV-713-PC-V5-DOCS-DELIVERY", status: "done"},
		{table: "req_dev", code: "DEV-714-PC-V5-PREVIEW-ROUTE-LOOKUP", status: "done"},
	} {
		pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(row.table) + `"[^\n]*code: "` + regexp.QuoteMeta(row.code) + `"[^\n]*status: "` + regexp.QuoteMeta(row.status) + `"[^\n]*\},[\t ]*$`)
		if !pattern.MatchString(reqStore) {
			t.Errorf("req_store.go must seed %s as %s", row.code, row.status)
		}
	}
}
