package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev383CustomerManagementPortalWarehouseRequirementSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-383-CUSTOMER-MANAGEMENT-PORTAL-WAREHOUSE",
		"DEV-383-CUSTOMER-MANAGEMENT-MENU",
		"DEV-383-PORTAL-WAREHOUSE-BINDING",
		"DEV-383-CUSTOMER-CUSTOM-TYPES",
		"UT-383-CUSTOMER-MANAGEMENT-PORTAL-WAREHOUSE",
		"API-383-CUSTOMER-MANAGEMENT-PORTAL-WAREHOUSE",
		"REV-383-CUSTOMER-MANAGEMENT-PORTAL-WAREHOUSE",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("customer management portal warehouse seed missing %q", want)
		}
	}
}

func TestDev383CustomerManagementPortalWarehouseSourceMarkers(t *testing.T) {
	sources := map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "App.vue"): {
			":key=\"currentViewIdentity\"",
			"currentViewIdentity",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "menu-ia.js"): {
			"customerManagement",
			"客户管理",
			"客户门户能力模板",
			"customerFulfillmentManual",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CustomerPortalSettingsView.vue"): {
			"客户仓库",
			"row.customer.warehouses",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"): {
			"绑定客户",
			"/api/stock/warehouses/${encodeURIComponent(selectedWarehouse.value)}/customer",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CustomersView.vue"): {
			"新增客户类型",
			"createCustomerTypeOption",
			"新增订单类型",
			"createOrderTypeOption",
		},
		filepath.Join("internal", "interfaces", "http", "stock", "stock_api.go"): {
			"/api/stock/warehouses/:code/customer",
			"BindWarehouseCustomer",
		},
		filepath.Join("internal", "interfaces", "http", "customer", "customer_routes.go"): {
			"/api/customers/customer-types",
			"/api/customers/order-types",
		},
		filepath.Join("internal", "infrastructure", "postgres", "customerportal", "admin_repository.go"): {
			"portalCustomerWarehouses",
			"portalProfileWarehouseFromCommand",
		},
	}

	for rel, wants := range sources {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer management portal warehouse marker %q", rel, want)
			}
		}
	}
}

func TestDev383CustomerManagementPortalWarehouseDocs(t *testing.T) {
	docs := map[string][]string{
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-383-CUSTOMER-MANAGEMENT-PORTAL-WAREHOUSE",
			"客户门户能力模板",
			"仓库库存页必须支持给仓库绑定或解绑客户",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-383-CUSTOMER-MANAGEMENT-PORTAL-WAREHOUSE",
			"从 SKU 设置切到客户档案",
			"门户客户配置不显示“代加工仓库”编辑项",
		},
		filepath.Join("docs", "OP_MANUAL_CUSTOMER_PORTAL.md"): {
			"客户管理：客户档案、门户客户配置、客户门户能力模板、客户履约手册",
			"客户仓库绑定",
			"不再维护“豆单展示版本”开关和“代加工仓库”",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"仓库绑定客户",
			"绑定客户",
			"门户客户配置只展示绑定结果",
		},
		filepath.Join("docs", "acceptance", "2026-05-28-customer-management-portal-warehouse.md"): {
			"PR-383",
			"客户管理",
			"浏览器验收",
		},
	}

	for rel, wants := range docs {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing customer management portal warehouse doc marker %q", rel, want)
			}
		}
	}
}
