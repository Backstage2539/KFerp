package orderresolution

import (
	"encoding/json"
	"testing"
)

func TestReplacementSupplyRequiresEveryExplicitIdentityAndQuantity(t *testing.T) {
	c, cmd := fixture()
	if err := json.Unmarshal([]byte(`[{"kind":"direct_ship","id":16,"relation":"supply","status":"reserved","label":"DSR-16","supply_reservations":[{"id":31,"quantity":3,"targets":[{"id":41,"available_qty":5}]}]}]`), &c.Dependencies); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, selections string
		valid            bool
	}{
		{"unselected", `[]`, false},
		{"missing quantity", `[{"reservation_id":31,"target_id":41}]`, false},
		{"another reservation", `[{"reservation_id":32,"target_id":41,"quantity":3}]`, false},
		{"another identity", `[{"reservation_id":31,"target_id":42,"quantity":3}]`, false},
		{"over quantity", `[{"reservation_id":31,"target_id":41,"quantity":4}]`, false},
		{"duplicate", `[{"reservation_id":31,"target_id":41,"quantity":3},{"reservation_id":31,"target_id":41,"quantity":3}]`, false},
		{"explicit matching supply", `[{"reservation_id":31,"target_id":41,"quantity":3}]`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := json.Unmarshal([]byte(`[{"kind":"direct_ship","id":16,"choice":"replace_supply","target_id":41,"supplies":`+tc.selections+`}]`), &cmd.Dispositions); err != nil {
				t.Fatal(err)
			}
			plan, err := Preview(c, cmd)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				if len(plan.Tasks) != 1 || plan.Tasks[0].Permission != "stock.resolve" {
					t.Fatalf("replacement must require warehouse approval: %+v", plan.Tasks)
				}
			} else if err == nil {
				t.Fatal("invalid source replacement accepted")
			}
		})
	}
}
