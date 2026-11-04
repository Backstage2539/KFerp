package production

import (
	"encoding/json"
	app "orderapp/internal/application/production"
	"testing"
)

func TestFreezeBOMStartRouteUsesSequenceAndPreservesExplicitConfiguration(t *testing.T) {
	raw := `{"yield_rate":0.8,"operations":[{"seq":20,"operation":"检查"},{"seq":10,"operation":"通用加工","execution_json":"{\"version\":1,\"node_key\":\"custom\",\"source_warehouse\":\"raw\",\"output_unit\":\"kg\"}"}]}`
	got, err := freezeBOMStartRoute(raw, "kg", "wip")
	if err != nil {
		t.Fatal(err)
	}
	var r processTemplateSnapshot
	if err = json.Unmarshal([]byte(got), &r); err != nil {
		t.Fatal(err)
	}
	var first, last app.OperationExecution
	json.Unmarshal([]byte(r.Operations[1].ExecutionJSON), &first)
	json.Unmarshal([]byte(r.Operations[0].ExecutionJSON), &last)
	if first.NodeKey != "custom" || first.SourceWarehouse != "raw" || !last.ReceiptEnabled || last.OutputUnit != "kg" {
		t.Fatalf("config changed or missing: %+v %+v", first, last)
	}
	if _, err = freezeBOMStartRoute(`{"operations":[{"seq":1},{"seq":1}]}`, "kg", "wip"); err == nil {
		t.Fatal("duplicate route accepted")
	}
	if _, err = freezeBOMStartRoute(`{}`, "kg", "wip"); err == nil {
		t.Fatal("empty route accepted")
	}
}
