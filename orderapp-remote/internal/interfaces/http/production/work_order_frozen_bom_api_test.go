package production

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
)

func TestWorkOrderDetailFrozenBOMUnaffectedByNewVersion(t *testing.T) {
	pool, schema := newProductionFlowTestDB(t)
	ctx := context.Background()
	e, _, wo := createSubmittedFullStockTypedPlan(t, ctx, pool, schema, 74891)
	type payload struct {
		Frozen struct {
			VersionID        int64  `json:"frozen_bom_version_id"`
			MaterialSnapshot string `json:"material_snapshot"`
		} `json:"frozen_bom"`
	}
	get := func() payload {
		t.Helper()
		r := serveMultilevelProductionJSON(t, e, http.MethodGet, fmt.Sprintf("/api/produce/work-orders/%d", wo), nil)
		if r.Code != http.StatusOK {
			t.Fatalf("detail %d: %s", r.Code, r.Body.String())
		}
		var p payload
		if err := json.Unmarshal(r.Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		return p
	}
	before := get()
	var materials []any
	if err := json.Unmarshal([]byte(before.Frozen.MaterialSnapshot), &materials); err != nil || len(materials) == 0 {
		t.Fatalf("missing frozen materials: %q (%v)", before.Frozen.MaterialSnapshot, err)
	}
	if before.Frozen.VersionID != 100 {
		t.Fatalf("unexpected original frozen version %d", before.Frozen.VersionID)
	}
	mustExecProductionFlowTestSQL(t, ctx, pool, fmt.Sprintf(`
		UPDATE %[1]s.production_bom_versions SET status='archived' WHERE id=100;
		INSERT INTO %[1]s.production_bom_versions(id,bom_id,version_no,status,yield_rate,material_loss_rate,output_qty,output_unit,process_route_id,published_at)
		VALUES (101,100,'V002','published',0.8,20,1,'kg',31,now());
		INSERT INTO %[1]s.production_bom_version_items(version_id,material_id,component_type,consume_unit,qty_per_unit,ratio_pct)
		VALUES (101,10,'material','g_per_bag',300,0),(101,20,'material','unit_per_bag',2,0);
		UPDATE %[1]s.product_production_bom_bindings SET bom_version_id=101 WHERE product_id=1;
		UPDATE %[1]s.production_bom_output_bindings SET bom_version_id=101 WHERE output_type='product' AND output_id=1;
		UPDATE %[1]s.product_production_configs SET production_bom_version_id=101 WHERE product_id=1;`, schema))
	after := get()
	if !reflect.DeepEqual(before.Frozen, after.Frozen) {
		t.Fatalf("frozen BOM changed after publishing new version: before=%+v after=%+v", before.Frozen, after.Frozen)
	}
}
