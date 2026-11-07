package orderresolution

import (
	"encoding/json"
	"strings"
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

func TestMixedSupplyRequiresExplicitPhysicalRetentionAndCurrentVersion(t *testing.T) {
	var d Dependency
	if err := json.Unmarshal([]byte(`{"kind":"direct_ship","id":16,"relation":"supply","quantity":2,"supply_reservations":[{"id":31,"quantity":2,"converted_quantity":1,"physical_version":"physical-v1","targets":[{"id":41,"available_qty":5}]}]}`), &d); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, physical string
		valid          bool
	}{
		{"unselected", ``, false},
		{"missing version", `,"physical_choice":"keep_reserved"`, false},
		{"stale version", `,"physical_choice":"keep_reserved","physical_version":"old"`, false},
		{"explicit retention", `,"physical_choice":"keep_reserved","physical_version":"physical-v1"`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var choice Disposition
			if err := json.Unmarshal([]byte(`{"choice":"replace_supply","supplies":[{"reservation_id":31,"target_id":41,"quantity":2`+tc.physical+`}]}`), &choice); err != nil {
				t.Fatal(err)
			}
			err := ValidateSupplyReplacement(d, choice)
			if tc.valid && err != nil {
				t.Fatal(err)
			}
			if !tc.valid && err == nil {
				t.Fatal("mixed physical inventory silently retained without explicit current confirmation")
			}
			if tc.valid {
				c, cmd := fixture()
				c.Dependencies = []Dependency{d}
				choice.Kind = "direct_ship"
				choice.ID = 16
				cmd.Dispositions = []Disposition{choice}
				plan, err := Preview(c, cmd)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(plan.Tasks[0].Label, "已入库 1") || !strings.Contains(plan.Tasks[0].Label, "保留原预留") {
					t.Fatalf("preview omitted the explicit physical disposition: %+v", plan.Tasks)
				}
			}
		})
	}
}

func TestReplacementIncludesFullyConvertedReservations(t *testing.T) {
	var d Dependency
	if err := json.Unmarshal([]byte(`{"relation":"supply","quantity":2,"supply_reservations":[{"id":31,"quantity":0,"converted_quantity":3,"physical_version":"full-v1"},{"id":32,"quantity":2,"targets":[{"id":41,"available_qty":5}]}]}`), &d); err != nil {
		t.Fatal(err)
	}
	var choice Disposition
	if err := json.Unmarshal([]byte(`{"choice":"replace_supply","supplies":[{"reservation_id":31,"quantity":0,"target_id":0,"physical_choice":"keep_reserved","physical_version":"full-v1"},{"reservation_id":32,"quantity":2,"target_id":41}]}`), &choice); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSupplyReplacement(d, choice); err != nil {
		t.Fatal(err)
	}
	choice.Supplies = choice.Supplies[1:]
	if err := ValidateSupplyReplacement(d, choice); err == nil {
		t.Fatal("fully converted reservation silently omitted")
	}
}
