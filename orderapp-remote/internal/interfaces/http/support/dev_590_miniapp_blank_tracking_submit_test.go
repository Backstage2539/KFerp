package support

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDev590MiniappBlankTrackingSubmitContracts(t *testing.T) {
	reqStore := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, row := range []struct {
		table    string
		code     string
		status   string
		assignee string
	}{
		{table: "req_product", code: "PR-590-MINIAPP-BLANK-TRACKING-SUBMIT", status: "review", assignee: "VA"},
		{table: "req_dev", code: "DEV-590-ORDER-NOT-NULL-TEXT-COMPAT", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-590-DRAFT-TRANSACTION-AUDIT-COMPAT", status: "done", assignee: "Codex"},
		{table: "req_dev", code: "DEV-590-DUAL-ENVIRONMENT-DELIVERY", status: "done", assignee: "Codex"},
		{table: "req_review", code: "REV-590-MINIAPP-BLANK-TRACKING-SUBMIT", status: "todo", assignee: "VA"},
	} {
		requireDev590SeedRow(t, reqStore, row.table, row.code, row.status, row.assignee)
	}

	repository := string(readOrderAppFileForTest(t, filepath.Join("internal", "infrastructure", "postgres", "sales", "repository.go")))
	for _, forbidden := range []string{
		"nullText(shipTrackingNo)",
		"nullText(shipMethod)",
		"nullText(cmd.Notes)",
		"nullText(cmd.ExpressFee)",
		"nullText(req.Notes)",
		"nullText(req.ExpressFee)",
		"nullText(req.ShipMethod)",
		"nullText(reason)",
		"var nextNotesPtr *string",
	} {
		if strings.Contains(repository, forbidden) {
			t.Fatalf("order writes must not use %s for a NOT NULL text column", forbidden)
		}
	}
	for binding, want := range map[string]int{
		"notNullText(shipTrackingNo)": 3,
		"notNullText(shipMethod)":     2,
		"notNullText(cmd.Notes)":      2,
		"notNullText(cmd.ExpressFee)": 2,
		"notNullText(req.Notes)":      1,
		"notNullText(req.ExpressFee)": 1,
		"notNullText(req.ShipMethod)": 1,
		"notNullText(reason)":         1,
		"nextNotesPtr := &nextNotes":  1,
		"notNullTextPtr(it.unit)":     1,
		"notNullTextPtr(it.spec)":     1,
	} {
		if got := strings.Count(repository, binding); got != want {
			t.Fatalf("%s bindings = %d, want %d", binding, got, want)
		}
	}
	if !strings.Contains(repository, "nextShip, nextProc, nextNotes); err != nil") {
		t.Fatal("inline order update must bind the normalized non-null notes string")
	}
	deleteAt := strings.Index(repository, `deleteEmployeeOrderDraftTx(ctx, tx, r.schema, cmd.DraftEmployeeID`)
	commitAt := strings.Index(repository, `tx.Commit(ctx)`)
	if deleteAt < 0 || commitAt < 0 || deleteAt > commitAt {
		t.Fatal("formal order submission must clear the employee draft inside the order transaction")
	}

	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "sales", "order_ship_tracking_not_null_test.go"): {
			"TestOrderWritesKeepBlankNotNullTextFieldsNonNull",
			"notNullText(shipTrackingNo)",
			"notNullText(reason)",
			"nextNotesPtr := &nextNotes",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-590-MINIAPP-BLANK-TRACKING-SUBMIT",
			"DEV-590-ORDER-NOT-NULL-TEXT-COMPAT",
			"DEV-590-DRAFT-TRANSACTION-AUDIT-COMPAT",
			"DEV-590-DUAL-ENVIRONMENT-DELIVERY",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-590-MINIAPP-BLANK-TRACKING-SUBMIT",
			"只有完整提交成功后草稿才消失",
			"development",
			"production",
		},
		filepath.Join("docs", "OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md"): {
			"未填写物流单号后提交订单失败",
			"只有正式订单完整提交成功后草稿才会清除",
			"明细单位/规格等非空文本字段按数据库空字符串保存",
		},
		filepath.Join("docs", "acceptance", "2026-08-10-miniapp-blank-tracking-submit.md"): {
			"PR-590 订单非空文本字段兼容与小程序草稿提交安全验收记录",
			"DEV-590-ORDER-NOT-NULL-TEXT-COMPAT",
			"DEV-590-DRAFT-TRANSACTION-AUDIT-COMPAT",
			"DEV-590-DUAL-ENVIRONMENT-DELIVERY",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-590 marker %q", rel, want)
			}
		}
	}

	orderAppRoot := findAncestorForTest(t, "go.mod")
	repoRoot := filepath.Dir(orderAppRoot)
	for _, rel := range []string{"REQUIREMENTS.md", "ACCEPTANCE_TESTS.md"} {
		src, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(src), "PR-590-MINIAPP-BLANK-TRACKING-SUBMIT") {
			t.Fatalf("root %s missing PR-590 contract", rel)
		}
	}
}

func requireDev590SeedRow(t *testing.T, src, table, code, status, assignee string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^[\t ]*\{table: "` + regexp.QuoteMeta(table) + `"[^\n]*code: "` + regexp.QuoteMeta(code) + `"[^\n]*status: "` + regexp.QuoteMeta(status) + `"[^\n]*assignee: "` + regexp.QuoteMeta(assignee) + `"[^\n]*\},[\t ]*$`)
	if !pattern.MatchString(src) {
		t.Fatalf("req_store.go missing one-line %s seed %s with status %s and assignee %s", table, code, status, assignee)
	}
}
