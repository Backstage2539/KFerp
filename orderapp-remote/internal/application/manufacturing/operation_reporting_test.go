package manufacturing

import (
	"encoding/json"
	"testing"
)

func TestNormalizeRouteExecutionPreservesReportingType(t *testing.T) {
	for _, mode := range []string{"legacy", "batch", "incoming", "piece"} {
		raw, err := NormalizeRouteExecution(`{"version":1,"reporting_type":"`+mode+`"}`, false, "one")
		if err != nil {
			t.Fatal(err)
		}
		var result RouteExecution
		if err = json.Unmarshal([]byte(raw), &result); err != nil || result.ReportingType != mode {
			t.Fatalf("%s %v", raw, err)
		}
	}
	if _, err := NormalizeRouteExecution(`{"version":1,"reporting_type":"guess"}`, false, "one"); err == nil {
		t.Fatal("invalid reporting type accepted")
	}
}
