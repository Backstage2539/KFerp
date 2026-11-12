package production

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWorkOrderFrozenDetailsExposeOnlyFrozenValues(t *testing.T) {
	row := FrozenWorkOrderBOM{VersionID: 461, MaterialSnapshot: `[{"material_name":"半成品","qty_per_unit":1}]`}
	b, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{`"frozen_bom_version_id":461`, `"material_snapshot":"[{\"material_name\":\"半成品\"`} {
		if !strings.Contains(string(b), expected) {
			t.Fatalf("missing %s: %s", expected, b)
		}
	}
}
