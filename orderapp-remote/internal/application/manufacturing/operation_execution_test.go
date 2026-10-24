package manufacturing

import (
	"encoding/json"
	"testing"
)

func TestNormalizeRouteExecutionDefaultsAndIdentity(t *testing.T) {
	first, err := NormalizeRouteExecution("", false, "route-7-step-1")
	if err != nil {
		t.Fatal(err)
	}
	last, err := NormalizeRouteExecution("", true, "route-7-step-2")
	if err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	_ = json.Unmarshal([]byte(first), &a)
	_ = json.Unmarshal([]byte(last), &b)
	if a["receipt_enabled"] != false || b["receipt_enabled"] != true || a["source_warehouse"] != "wip" {
		t.Fatalf("defaults %s %s", first, last)
	}
	kept, err := NormalizeRouteExecution(`{"version":1,"node_key":"stable","receipt_enabled":true,"receipt_warehouse":"packing","source_warehouse":"raw_material","output_unit":"kg"}`, false, "replacement")
	if err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(kept), &a)
	if a["node_key"] != "stable" || a["receipt_warehouse"] != "packing" {
		t.Fatal(kept)
	}
	if _, err = NormalizeRouteExecution(`{"version":1,"receipt_enabled":true}`, false, "mid"); err == nil {
		t.Fatal("middle receipt needs a warehouse")
	}
}
