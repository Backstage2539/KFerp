package support

import "testing"

func int64Ptr(v int64) *int64 {
	return &v
}

func TestDecorateAuditLogRowMakesMaterialUpdateReadable(t *testing.T) {
	field := "sale_price"
	oldValue := "0.00"
	newValue := "12.50"
	label := "ALO 688"
	row := AuditLogRow{
		Actor:       "order",
		EntityType:  "material",
		EntityID:    int64Ptr(22),
		EntityLabel: &label,
		Action:      "update",
		Field:       &field,
		OldValue:    &oldValue,
		NewValue:    &newValue,
	}

	decorateAuditLogRow(&row, nil, nil)

	if row.Menu != "物料管理 / 物料档案/库存" {
		t.Fatalf("Menu = %q", row.Menu)
	}
	if row.Feature != "编辑物料档案" {
		t.Fatalf("Feature = %q", row.Feature)
	}
	if row.Summary != "order 在物料档案/库存修改了物料 ALO 688 的销售价：0.00 -> 12.50" {
		t.Fatalf("Summary = %q", row.Summary)
	}
	if row.EntityType != "物料" {
		t.Fatalf("EntityType = %q", row.EntityType)
	}
	if row.Action != "修改" {
		t.Fatalf("Action = %q", row.Action)
	}
}

func TestDecorateAuditLogRowIdentifiesOperationMenuAndFeature(t *testing.T) {
	field := "POST /app/api/materials/22"
	status := "200"
	meta := `{"route":"/api/materials/:id","method":"POST","path":"/app/api/materials/22"}`
	row := AuditLogRow{
		Actor:      "order",
		EntityType: "operation",
		Action:     "request",
		Field:      &field,
		NewValue:   &status,
		Meta:       &meta,
	}

	decorateAuditLogRow(&row, nil, nil)

	if row.Menu != "物料管理 / 物料档案/库存" {
		t.Fatalf("Menu = %q", row.Menu)
	}
	if row.Feature != "保存物料行内编辑" {
		t.Fatalf("Feature = %q", row.Feature)
	}
	if row.Summary != "order 在物料档案/库存保存物料行内编辑，请求成功" {
		t.Fatalf("Summary = %q", row.Summary)
	}
}
