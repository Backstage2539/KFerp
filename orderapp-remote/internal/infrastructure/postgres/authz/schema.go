package authz

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"
)

type roleSeed struct {
	Code        string
	Name        string
	Description string
	Permissions []string
}

type permissionSeed struct {
	Code string
	Name string
}

func EnsureSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	q := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s.auth_roles (
	code TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS %s.auth_permissions (
	code TEXT PRIMARY KEY,
	name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS %s.auth_role_permissions (
	role_code TEXT NOT NULL REFERENCES %s.auth_roles(code) ON DELETE CASCADE,
	permission_code TEXT NOT NULL REFERENCES %s.auth_permissions(code) ON DELETE CASCADE,
	PRIMARY KEY(role_code, permission_code)
);
CREATE TABLE IF NOT EXISTS %s.employee_roles (
	employee_id BIGINT NOT NULL REFERENCES %s.company_employees(id) ON DELETE CASCADE,
	role_code TEXT NOT NULL REFERENCES %s.auth_roles(code) ON DELETE CASCADE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY(employee_id, role_code)
);
CREATE TABLE IF NOT EXISTS %s.auth_view_permissions (
	view_key TEXT PRIMARY KEY,
	permission_code TEXT NOT NULL REFERENCES %s.auth_permissions(code) ON DELETE RESTRICT
);
`, schema, schema, schema, schema, schema, schema, schema, schema, schema, schema)
	if _, err := pool.Exec(ctx, q); err != nil {
		return err
	}
	if err := seedPermissions(ctx, pool, schema); err != nil {
		return err
	}
	if err := seedRoles(ctx, pool, schema); err != nil {
		return err
	}
	if err := seedViewPermissions(ctx, pool, schema); err != nil {
		return err
	}
	return removeMergedViewPermissions(ctx, pool, schema)
}

func seedPermissions(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	for _, permission := range defaultPermissions() {
		if _, err := pool.Exec(ctx, "INSERT INTO "+schema+".auth_permissions(code,name) VALUES($1,$2) ON CONFLICT (code) DO UPDATE SET name=excluded.name", permission.Code, permission.Name); err != nil {
			return err
		}
	}
	return nil
}

func seedRoles(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	for _, role := range defaultRoles() {
		if _, err := pool.Exec(ctx, "INSERT INTO "+schema+".auth_roles(code,name,description) VALUES($1,$2,$3) ON CONFLICT (code) DO UPDATE SET name=excluded.name,description=excluded.description", role.Code, role.Name, role.Description); err != nil {
			return err
		}
		for _, permission := range role.Permissions {
			if _, err := pool.Exec(ctx, "INSERT INTO "+schema+".auth_role_permissions(role_code,permission_code) VALUES($1,$2) ON CONFLICT DO NOTHING", role.Code, permission); err != nil {
				return err
			}
		}
	}
	return nil
}

func seedViewPermissions(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	for viewKey, permission := range defaultViewPermissions() {
		if _, err := pool.Exec(ctx, "INSERT INTO "+schema+".auth_view_permissions(view_key,permission_code) VALUES($1,$2) ON CONFLICT (view_key) DO UPDATE SET permission_code=excluded.permission_code", viewKey, permission); err != nil {
			return err
		}
	}
	return nil
}

func removeMergedViewPermissions(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	_, err := pool.Exec(ctx, "DELETE FROM "+schema+".auth_view_permissions WHERE view_key=ANY($1::text[])", []string{"userPermissions"})
	return err
}

func defaultPermissions() []permissionSeed {
	return []permissionSeed{
		{Code: "auth.manage", Name: "用户角色管理"},
		{Code: "orders.read", Name: "查看订单"},
		{Code: "orders.write", Name: "录入和修改订单"},
		{Code: "customers.read", Name: "查看客户"},
		{Code: "customers.write", Name: "维护客户"},
		{Code: "production.read", Name: "查看生产"},
		{Code: "production.run", Name: "执行生产"},
		{Code: "stock.read", Name: "查看库存"},
		{Code: "stock.write", Name: "调整库存"},
		{Code: "customer_processing.read", Name: "客户查看代加工数据"},
		{Code: "customer_processing.submit", Name: "客户提交代加工和代发"},
		{Code: "purchase.read", Name: "查看采购"},
		{Code: "purchase.write", Name: "维护采购"},
		{Code: "materials.read", Name: "查看物料"},
		{Code: "materials.write", Name: "维护物料"},
		{Code: "bom.read", Name: "查看 BOM"},
		{Code: "bom.write", Name: "维护 BOM"},
		{Code: "products.read", Name: "查看商品"},
		{Code: "products.write", Name: "维护商品"},
		{Code: "costing.read", Name: "查看成本核算"},
		{Code: "costing.write", Name: "维护成本核算"},
		{Code: "finance.read", Name: "查看财务报表"},
		{Code: "finance.write", Name: "维护财务费用和设置"},
		{Code: "finance.close", Name: "执行财务月结和调整"},
		{Code: "finance.close_mode.manage", Name: "切换财务锁账模式"},
		{Code: "settings.write", Name: "维护系统设置"},
		{Code: "company.manage", Name: "维护部门员工"},
		{Code: "audit.read", Name: "查看操作日志"},
		{Code: "requirements.manage", Name: "维护需求流程"},
	}
}

func defaultRoles() []roleSeed {
	all := make([]string, 0, len(defaultPermissions()))
	for _, permission := range defaultPermissions() {
		all = append(all, permission.Code)
	}
	sort.Strings(all)
	return []roleSeed{
		{Code: "admin", Name: "管理员", Description: "系统全权限", Permissions: all},
		{Code: "sales", Name: "销售", Description: "录单、订单、客户与商品查询", Permissions: []string{"orders.read", "orders.write", "customers.read", "customers.write", "products.read"}},
		{Code: "production", Name: "生产", Description: "生产执行、库存/物料/BOM 查看", Permissions: []string{"production.read", "production.run", "stock.read", "materials.read", "bom.read"}},
		{Code: "warehouse", Name: "仓库", Description: "库存、采购收货和物料维护", Permissions: []string{"stock.read", "stock.write", "purchase.read", "purchase.write", "materials.read", "materials.write", "products.read"}},
		{Code: "finance", Name: "财务", Description: "订单、库存、采购、报价、成本核算和财务月结", Permissions: []string{"orders.read", "stock.read", "purchase.read", "products.read", "costing.read", "finance.read", "finance.write", "finance.close"}},
		{Code: "product", Name: "商品", Description: "商品、BOM 和成本维护", Permissions: []string{"products.read", "products.write", "bom.read", "bom.write", "costing.read", "costing.write"}},
		{Code: "system", Name: "系统管理员", Description: "员工、设置、日志和需求维护", Permissions: []string{"auth.manage", "company.manage", "settings.write", "audit.read", "requirements.manage"}},
		{Code: "customer_processing_customer", Name: "代加工客户", Description: "客户登录 ERP 后查看自己的代加工数据并提交工单和代发信息", Permissions: []string{"customer_processing.read", "customer_processing.submit"}},
		{Code: "customer_direct_ship_customer", Name: "公共SKU代发客户", Description: "客户登录 ERP 后查看自己的代发订单并提交公共 SKU 一件代发信息", Permissions: []string{"customer_processing.read", "customer_processing.submit"}},
	}
}

func defaultViewPermissions() map[string]string {
	return map[string]string{
		"order":                       "orders.write",
		"orders":                      "orders.read",
		"orderSalesManual":            "orders.read",
		"orderInvoice":                "orders.read",
		"salesOrder":                  "orders.read",
		"deliveryNote":                "orders.read",
		"contracts":                   "orders.write",
		"customers":                   "customers.read",
		"producePlan":                 "production.run",
		"productionFlow":              "production.read",
		"productionConfig":            "bom.read",
		"productionAcceptance":        "production.read",
		"produceRunning":              "production.read",
		"workOrders":                  "production.read",
		"jobCards":                    "production.read",
		"qualityInspections":          "production.read",
		"produceLogs":                 "production.read",
		"productionCosts":             "costing.read",
		"productionManual":            "production.read",
		"warehouseInventory":          "stock.read",
		"stockOperations":             "stock.write",
		"purchase":                    "purchase.write",
		"materials":                   "materials.read",
		"materialReceipts":            "stock.write",
		"materialBatches":             "stock.read",
		"wipMaterials":                "stock.read",
		"stockLedger":                 "stock.read",
		"stockBatches":                "stock.read",
		"stockAdjustments":            "stock.write",
		"stockOutboundLogs":           "stock.read",
		"inventoryMaterialsManual":    "stock.read",
		"inventory":                   "stock.read",
		"allocationLogs":              "stock.read",
		"productSettings":             "products.write",
		"mallSettings":                "products.write",
		"bom":                         "bom.read",
		"products":                    "products.read",
		"costing":                     "costing.read",
		"costingManual":               "costing.read",
		"costingSettings":             "costing.write",
		"financeDashboard":            "finance.read",
		"financeExpenses":             "finance.write",
		"processingBilling":           "finance.read",
		"financeClosing":              "finance.close",
		"financeReport":               "finance.read",
		"financeTaxLedger":            "finance.write",
		"financeSettings":             "finance.write",
		"financeManual":               "finance.read",
		"machines":                    "settings.write",
		"companyProfile":              "settings.write",
		"salesOrderSettings":          "settings.write",
		"senderSettings":              "settings.write",
		"outsourceSettings":           "settings.write",
		"uiSettings":                  "settings.write",
		"notificationSettings":        "settings.write",
		"notificationManual":          "settings.write",
		"customerCapabilityTemplates": "customers.write",
		"customerPortalSettings":      "customers.write",
		"customerPortalManual":        "customers.read",
		"customerFulfillment":         "stock.write",
		"customerFulfillmentManual":   "stock.read",
		"customerProcessingPortal":    "customer_processing.read",
		"settingsAuditManual":         "audit.read",
		"departments":                 "company.manage",
		"employees":                   "company.manage",
		"audit":                       "audit.read",
		"reqProduct":                  "requirements.manage",
		"reqDev":                      "requirements.manage",
		"reqUnit":                     "requirements.manage",
		"reqApi":                      "requirements.manage",
		"reqReview":                   "requirements.manage",
		"requirementsManual":          "requirements.manage",
	}
}
