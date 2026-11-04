package production

import (
	"context"
	"encoding/json"
	"fmt"
	app "orderapp/internal/application/production"
	"testing"
)

func TestBOMMaterialSourcesDoNotRequireOrChangeOperationBindings(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, _ := startProductCompletionAtomicityFixture(t, ctx, pool, schema, 74745)
	response := serveMultilevelProductionJSON(t, e, "GET", "/api/production-bom-versions/100/material-sources", nil)
	if response.Code != 200 {
		t.Fatalf("sources: %d %s", response.Code, response.Body.String())
	}
	var config app.OperationMaterialConfig
	if err := json.Unmarshal(response.Body.Bytes(), &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Rows) == 0 {
		t.Fatal("fixture needs BOM components")
	}
	bindings := map[string]any{}
	for _, row := range config.Rows {
		bindings[row.Key] = map[string]any{"source_warehouse": "wip"}
	}
	response = serveMultilevelProductionJSON(t, e, "PUT", "/api/production-bom-versions/100/material-sources", map[string]any{"expected_version": config.Version, "bindings": bindings})
	if response.Code != 200 {
		t.Fatalf("save sources: %d %s", response.Code, response.Body.String())
	}
	var raw []byte
	if err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT bindings FROM %s.production_bom_operation_bindings WHERE bom_version_id=100 AND bom_variant_id=0`, schema)).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved map[string]app.OperationMaterialBinding
	if err := json.Unmarshal(raw, &saved); err != nil {
		t.Fatal(err)
	}
	for _, b := range saved {
		if b.SourceWarehouse != "wip" || b.NodeKey != "" {
			t.Fatalf("sources save wrote operation: %+v", b)
		}
	}
	assertProductionFlowCount(t, pool, schema, "audit_logs", "entity_type='production_bom_version' AND action='material_sources'", 1)
}
