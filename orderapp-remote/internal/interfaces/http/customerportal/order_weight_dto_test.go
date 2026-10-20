package customerportal

import (
	"encoding/json"
	salesapp "orderapp/internal/application/sales"
	"reflect"
	"testing"
)

func TestMiniEmployeeOrderDetailIncludesNullableFrozenWeights(t *testing.T) {
	item := salesapp.OrderEditItem{ItemID: 1, ProductID: 7}
	fields := map[string]string{"MaterialUnitWeightG": "227.000000", "MaterialItemWeightG": "908.000000", "LogisticsUnitWeightG": "250.000000", "LogisticsItemWeightG": "1000.000000"}
	for name, value := range fields {
		f := reflect.ValueOf(&item).Elem().FieldByName(name)
		if !f.IsValid() {
			t.Fatalf("order item missing frozen weight field %s", name)
		}
		v := value
		f.Set(reflect.ValueOf(&v))
	}
	detail := miniEmployeeOrderDetail(salesapp.OrderRow{ID: 42}, salesapp.OrderFormData{EditData: &salesapp.OrderEditData{ID: 42, Items: []salesapp.OrderEditItem{item, {ItemID: 2}}}})
	b, err := json.Marshal(detail.Items)
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err = json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{"material_unit_weight_g": "227.000000", "material_item_weight_g": "908.000000", "logistics_unit_weight_g": "250.000000", "logistics_item_weight_g": "1000.000000"} {
		if got[0][key] != want {
			t.Errorf("%s = %v, want %s", key, got[0][key], want)
		}
		value, present := got[1][key]
		if !present || value != nil {
			t.Errorf("legacy unknown %s must be explicit null: %s", key, b)
		}
	}
}
