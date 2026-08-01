package support

import (
	"strings"
	"testing"
)

func TestEmployeeOrderDraftAuditRowsAreBusinessSearchable(t *testing.T) {
	entityID := int64(12)
	field := "payload"
	meta := `{"employee_id":41}`
	row := AuditLogRow{
		Actor:      "mini-employee:41:销售甲",
		EntityType: "employee_order_draft",
		EntityID:   &entityID,
		Action:     "update",
		Field:      &field,
		Meta:       &meta,
	}
	decorateAuditLogRow(&row, nil, nil)

	if row.EntityType != "订单草稿" || row.Menu != "订单销售 / 录单" || row.Feature != "保存订单草稿" {
		t.Fatalf("decorated draft row=%+v", row)
	}
	if row.EntityLabel == nil || !strings.Contains(*row.EntityLabel, "员工 41") || !strings.Contains(row.Summary, "保存了订单草稿") {
		t.Fatalf("draft label=%v summary=%q", row.EntityLabel, row.Summary)
	}
	if row.Field == nil || *row.Field != "草稿内容" {
		t.Fatalf("draft field=%v", row.Field)
	}

	row = AuditLogRow{Actor: "mini-employee:41:销售甲", EntityType: "employee_order_draft", EntityID: &entityID, Action: "delete", Meta: &meta}
	decorateAuditLogRow(&row, nil, nil)
	if row.Feature != "清除订单草稿" || !strings.Contains(row.Summary, "清除了订单草稿") {
		t.Fatalf("delete draft row=%+v", row)
	}
}
