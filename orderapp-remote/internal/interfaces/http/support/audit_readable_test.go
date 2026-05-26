package support

import (
	"strings"
	"testing"
)

func int64Ptr(v int64) *int64 {
	return &v
}

func strPtr(v string) *string {
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

	if row.Menu != "库存管理 / 物料档案" {
		t.Fatalf("Menu = %q", row.Menu)
	}
	if row.Feature != "编辑物料档案" {
		t.Fatalf("Feature = %q", row.Feature)
	}
	if row.Summary != "order 在物料档案修改了物料 ALO 688 的销售价：0.00 -> 12.50" {
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

	if row.Menu != "库存管理 / 物料档案" {
		t.Fatalf("Menu = %q", row.Menu)
	}
	if row.Feature != "保存物料行内编辑" {
		t.Fatalf("Feature = %q", row.Feature)
	}
	if row.Summary != "order 在物料档案保存物料行内编辑，请求成功" {
		t.Fatalf("Summary = %q", row.Summary)
	}
}

func TestDecorateAuditLogRowScannedEntitiesUseReadableLabels(t *testing.T) {
	tests := []struct {
		name        string
		entityType  string
		action      string
		field       string
		meta        string
		wantMenu    string
		wantFeature string
		wantEntity  string
		wantAction  string
		wantField   string
		wantTarget  string
	}{
		{
			name:        "company profile",
			entityType:  "company_profile",
			action:      "update",
			field:       "company_name",
			meta:        `{"company_name":"昆山月麓咖啡有限公司"}`,
			wantMenu:    "设置 / 公司设置",
			wantFeature: "保存公司设置",
			wantEntity:  "公司信息",
			wantAction:  "修改",
			wantField:   "公司名称",
			wantTarget:  "公司信息 昆山月麓咖啡有限公司",
		},
		{
			name:        "material receipt",
			entityType:  "material_receipt",
			action:      "submit",
			field:       "qty_g",
			meta:        `{"batch_code":"MR20260502001","material_id":22}`,
			wantMenu:    "库存管理 / 采购入库",
			wantFeature: "提交原料入库",
			wantEntity:  "原料入库单",
			wantAction:  "提交",
			wantField:   "数量(g)",
			wantTarget:  "原料入库单 MR20260502001",
		},
		{
			name:        "cost parameter",
			entityType:  "cost_parameter",
			action:      "update",
			field:       "roast_yield_rate",
			meta:        `{"label":"烘焙得率"}`,
			wantMenu:    "设置 / 成本参数设置",
			wantFeature: "保存成本参数",
			wantEntity:  "成本参数",
			wantAction:  "修改",
			wantField:   "烘焙得率",
			wantTarget:  "成本参数 烘焙得率",
		},
		{
			name:        "bean list publication",
			entityType:  "bean_list_publication",
			action:      "save_draft",
			field:       "status",
			meta:        `{"list_type":"espresso","version_no":3}`,
			wantMenu:    "商品与配方 / 产品设置",
			wantFeature: "保存豆单草稿",
			wantEntity:  "豆单发布",
			wantAction:  "保存草稿",
			wantField:   "状态",
			wantTarget:  "豆单发布 espresso v3",
		},
		{
			name:        "sales order document",
			entityType:  "sales_order_document",
			action:      "create",
			field:       "version_no",
			meta:        `{"order_no":"SO-001","version_no":2}`,
			wantMenu:    "订单销售 / 销售单",
			wantFeature: "生成销售单PDF",
			wantEntity:  "销售单文件",
			wantAction:  "新增",
			wantField:   "版本号",
			wantTarget:  "销售单文件 SO-001 v2",
		},
		{
			name:        "combined sales order document",
			entityType:  "combined_sales_order_document",
			action:      "create",
			field:       "version_no",
			meta:        `{"order_nos":"SO-001, SO-002","version_no":1}`,
			wantMenu:    "订单销售 / 销售单",
			wantFeature: "生成组合销售单PDF",
			wantEntity:  "组合销售单文件",
			wantAction:  "新增",
			wantField:   "版本号",
			wantTarget:  "组合销售单文件 SO-001, SO-002 v1",
		},
		{
			name:        "combined delivery note document",
			entityType:  "combined_delivery_note_document",
			action:      "create",
			field:       "version_no",
			meta:        `{"order_nos":"SO-001, SO-002","version_no":1}`,
			wantMenu:    "订单销售 / 出库单",
			wantFeature: "生成组合出库单PDF",
			wantEntity:  "组合出库单文件",
			wantAction:  "新增",
			wantField:   "版本号",
			wantTarget:  "组合出库单文件 SO-001, SO-002 v1",
		},
		{
			name:        "product category",
			entityType:  "product_category",
			action:      "move",
			field:       "parent_position",
			meta:        `{"parent_id":5,"position":2}`,
			wantMenu:    "商品与配方 / 产品设置",
			wantFeature: "调整产品分类",
			wantEntity:  "产品分类",
			wantAction:  "移动",
			wantField:   "分类位置",
			wantTarget:  "产品分类 8",
		},
		{
			name:        "process template",
			entityType:  "process_template",
			action:      "create",
			field:       "template",
			meta:        `{"status":"draft","product_id":433,"operation_count":1}`,
			wantMenu:    "商品与配方 / 工艺模板",
			wantFeature: "维护工艺模板",
			wantEntity:  "工艺模板",
			wantAction:  "新增",
			wantField:   "模板",
			wantTarget:  "工艺模板 new",
		},
		{
			name:        "industry field template",
			entityType:  "industry_field_template",
			action:      "create",
			field:       "template",
			meta:        `{"status":"active","industry_key":"apparel","field_count":1}`,
			wantMenu:    "商品与配方 / 行业字段模板",
			wantFeature: "维护行业字段模板",
			wantEntity:  "行业字段模板",
			wantAction:  "新增",
			wantField:   "模板",
			wantTarget:  "行业字段模板 new",
		},
		{
			name:        "wip reservation",
			entityType:  "wip_reservation",
			action:      "release",
			field:       "work_order_no",
			meta:        `{"work_order_no":"WO-20260502","released_g":1200}`,
			wantMenu:    "生产管理 / 生产中",
			wantFeature: "释放WIP占用",
			wantEntity:  "WIP占用",
			wantAction:  "释放",
			wantField:   "生产工单号",
			wantTarget:  "WIP占用 WO-20260502",
		},
		{
			name:        "stock adjustment material cost",
			entityType:  "stock_adjustment",
			action:      "submit",
			field:       "unit_cost",
			meta:        `{"adjustment_type":"material_cost","batch_code":"MB-0000000007","material_batch_id":7,"value_change":615}`,
			wantMenu:    "库存管理 / 库存作业",
			wantFeature: "提交库存调整",
			wantEntity:  "库存调整单",
			wantAction:  "提交",
			wantField:   "批次成本/kg",
			wantTarget:  "库存调整单 MB-0000000007",
		},
		{
			name:        "auth account",
			entityType:  "auth_account",
			action:      "reset_password",
			field:       "",
			meta:        `{"employee_id":17}`,
			wantMenu:    "系统 / 员工维护",
			wantFeature: "重置员工密码",
			wantEntity:  "员工账号",
			wantAction:  "重置密码",
			wantTarget:  "员工账号 17",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			row := AuditLogRow{
				Actor:      "order",
				EntityType: tc.entityType,
				EntityID:   int64Ptr(8),
				Action:     tc.action,
				Meta:       strPtr(tc.meta),
				OldValue:   strPtr("old"),
				NewValue:   strPtr("new"),
			}
			if strings.TrimSpace(tc.field) != "" {
				row.Field = strPtr(tc.field)
			}

			decorateAuditLogRow(&row, nil, nil)

			if row.Menu != tc.wantMenu {
				t.Fatalf("Menu = %q, want %q", row.Menu, tc.wantMenu)
			}
			if row.Feature != tc.wantFeature {
				t.Fatalf("Feature = %q, want %q", row.Feature, tc.wantFeature)
			}
			if row.EntityType != tc.wantEntity {
				t.Fatalf("EntityType = %q, want %q", row.EntityType, tc.wantEntity)
			}
			if row.Action != tc.wantAction {
				t.Fatalf("Action = %q, want %q", row.Action, tc.wantAction)
			}
			if row.Field != nil && *row.Field != tc.wantField {
				t.Fatalf("Field = %q, want %q", *row.Field, tc.wantField)
			}
			if !strings.Contains(row.Summary, tc.wantTarget) {
				t.Fatalf("Summary = %q, want target %q", row.Summary, tc.wantTarget)
			}
			if row.EntityLabel == nil || *row.EntityLabel != tc.wantTarget {
				t.Fatalf("EntityLabel = %v, want %q", row.EntityLabel, tc.wantTarget)
			}
		})
	}
}

func TestDecorateAuditLogRowScannedOperationRoutesUseCurrentMenuIA(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		route       string
		path        string
		wantMenu    string
		wantFeature string
	}{
		{"cost settings", "POST", "/api/costing/settings/:key", "/api/costing/settings/roast_yield_rate", "设置 / 成本参数设置", "保存成本参数"},
		{"stock material transfer", "POST", "/api/stock/material-transfers", "/api/stock/material-transfers", "库存管理 / 库存作业", "提交原料转仓"},
		{"finished inventory", "POST", "/api/products/inventory/adjust", "/api/products/inventory/adjust", "库存管理 / 仓库库存", "调整成品库存"},
		{"production quality", "POST", "/api/produce/quality-inspections", "/api/produce/quality-inspections", "生产管理 / 生产质检", "保存质检记录"},
		{"production quality list", "GET", "/api/produce/quality-inspections", "/api/produce/quality-inspections", "生产管理 / 生产质检", "查看质检记录"},
		{"company profile", "POST", "/api/company/profile", "/api/company/profile", "设置 / 公司设置", "保存公司设置"},
		{"company profile view", "GET", "/api/company/profile", "/api/company/profile", "设置 / 公司设置", "查看公司设置"},
		{"stock material transfer list", "GET", "/api/stock/material-transfers", "/api/stock/material-transfers", "库存管理 / 库存作业", "查看原料转仓"},
		{"reset employee password", "POST", "/api/auth/password/reset", "/api/auth/password/reset", "系统 / 员工维护", "重置员工密码"},
		{"sales order settings", "POST", "/api/settings/sales-order/seal-position", "/api/settings/sales-order/seal-position", "设置 / 销售单设置", "保存公章位置"},
		{"sales order payment layout", "POST", "/api/settings/sales-order/payment-layout", "/api/settings/sales-order/payment-layout", "设置 / 销售单设置", "保存收款版式"},
		{"sales order shared seal select", "POST", "/api/settings/sales-order/seal/select", "/api/settings/sales-order/seal/select", "设置 / 销售单设置", "选择共享公章"},
		{"sales order settings view", "GET", "/api/settings/sales-order", "/api/settings/sales-order", "设置 / 销售单设置", "查看销售单设置"},
		{"product category move", "POST", "/api/product-settings/products/:id/category", "/api/product-settings/products/15/category", "商品与配方 / 产品设置", "调整产品分类"},
		{"process template save", "POST", "/api/process-templates", "/api/process-templates", "商品与配方 / 工艺模板", "保存工艺模板"},
		{"process template publish", "POST", "/api/process-templates/:id/publish", "/api/process-templates/1/publish", "商品与配方 / 工艺模板", "发布工艺模板"},
		{"industry field template save", "POST", "/api/industry-field-templates", "/api/industry-field-templates", "商品与配方 / 行业字段模板", "保存行业字段模板"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			field := tc.method + " " + tc.path
			meta := `{"route":"` + tc.route + `","method":"` + tc.method + `","path":"` + tc.path + `"}`
			row := AuditLogRow{
				Actor:      "order",
				EntityType: "operation",
				Action:     "request",
				Field:      &field,
				NewValue:   strPtr("200"),
				Meta:       &meta,
			}

			decorateAuditLogRow(&row, nil, nil)

			if row.Menu != tc.wantMenu {
				t.Fatalf("Menu = %q, want %q", row.Menu, tc.wantMenu)
			}
			if row.Feature != tc.wantFeature {
				t.Fatalf("Feature = %q, want %q", row.Feature, tc.wantFeature)
			}
			if strings.Contains(row.Summary, "未分类") || strings.Contains(row.Summary, "访问系统页面") {
				t.Fatalf("Summary should not use fallback wording: %q", row.Summary)
			}
		})
	}
}
