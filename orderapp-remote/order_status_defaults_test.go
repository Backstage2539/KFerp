package main

import (
	"os"
	"strings"
	"testing"
)

func TestOrderStatusDefaults(t *testing.T) {
	repo, err := os.ReadFile("sales_order_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	repoText := string(repo)
	for _, want := range []string{
		`lookupDefaultStatusID(ctx, tx, r.schema, "pay_statuses", "未付款", "未收款")`,
		`lookupDefaultStatusID(ctx, tx, r.schema, "ship_statuses", "未发货")`,
		"nullInt(payStatusID)",
		"nullInt(shipStatusID)",
	} {
		if !strings.Contains(repoText, want) {
			t.Fatalf("sales order repository missing %q", want)
		}
	}

	tpl, err := os.ReadFile("templates/order.html")
	if err != nil {
		t.Fatal(err)
	}
	tplText := string(tpl)
	for _, want := range []string{
		"function ensureDefaultPayStatus(){",
		"OPTS.payStatuses",
		"未付款",
		"未收款",
		"ensureDefaultPayStatus();",
		"ensureDefaultShipStatus();",
	} {
		if !strings.Contains(tplText, want) {
			t.Fatalf("order template missing %q", want)
		}
	}
}
