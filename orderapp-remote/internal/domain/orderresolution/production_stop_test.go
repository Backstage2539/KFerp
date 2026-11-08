package orderresolution

import (
	"encoding/json"
	"testing"
)

func TestProductionStopRequiresExplicitRemainderDispositions(t *testing.T) {
	for _, tc := range []struct {
		name, choices string
		valid         bool
	}{
		{"missing", `[]`, false},
		{"implicit stock", `[{"work_order_id":31}]`, false},
		{"invented return", `[{"work_order_id":31,"materials":"returned","wip":"retain_original","reservations":"release_unconsumed","outputs":"keep_original"}]`, false},
		{"missing wip", `[{"work_order_id":31,"materials":"retain_original","wip":"none","reservations":"keep","outputs":"keep_original"}]`, false},
		{"wrong work", `[{"work_order_id":32,"materials":"retain_original","wip":"retain_original","reservations":"keep","outputs":"keep_original"}]`, false},
		{"explicit", `[{"work_order_id":31,"materials":"retain_original","wip":"retain_original","reservations":"release_unconsumed","outputs":"keep_original"}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, cmd := fixture()
			if err := json.Unmarshal([]byte(`[{"kind":"production","id":19,"status":"running","stop_work_orders":[{"work_order_id":31,"label":"WO-31","version":"v1","has_materials":true,"has_wip":true,"has_reservations":true,"has_outputs":true}]}]`), &c.Dependencies); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(`[{"kind":"production","id":19,"choice":"stop","stops":`+tc.choices+`}]`), &cmd.Dispositions); err != nil {
				t.Fatal(err)
			}
			plan, err := Preview(c, cmd)
			if !tc.valid && err == nil {
				t.Fatal("stop accepted without exact stock and WIP decisions")
			}
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				production, warehouse := false, false
				for _, task := range plan.Tasks {
					production = production || task.Permission == "production.resolve"
					warehouse = warehouse || task.Permission == "stock.resolve"
				}
				if !production || !warehouse {
					t.Fatalf("stop needs both production and warehouse confirmation: %+v", plan.Tasks)
				}
			}
		})
	}
}
