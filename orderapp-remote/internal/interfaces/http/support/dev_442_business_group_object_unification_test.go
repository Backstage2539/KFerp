package support

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDev442BusinessGroupObjectUnificationSeeds(t *testing.T) {
	src := string(readOrderAppFileForTest(t, filepath.Join("internal", "interfaces", "http", "support", "req_store.go")))
	for _, want := range []string{
		"PR-442-BUSINESS-GROUP-OBJECT-UNIFICATION",
		"DEV-442-GENERIC-GROUP-ASSIGNMENTS",
		"DEV-442-PRODUCT-GROUP-ASSIGNMENT",
		"DEV-442-BOM-GROUP-ASSIGNMENT",
		"DEV-442-WAREHOUSE-GROUP-ASSIGNMENT",
		"DEV-442-PRICE-LIST-GROUP-SNAPSHOT",
		"REV-442-BUSINESS-GROUP-OBJECT-UNIFICATION",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("PR-442 requirement seed missing %q", want)
		}
	}
}

func TestDev442BusinessGroupAssignmentBackendContracts(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "schema.go"): {
			"business_group_assignments",
			"object_ref TEXT NOT NULL DEFAULT ''",
			"business_group_assignments_object_ref_idx",
			"migrateProductCategoriesToBusinessGroups",
			"migrateProductionBomGroupsToBusinessGroups",
			"migrateWarehousesToBusinessGroups",
		},
		filepath.Join("internal", "application", "catalog", "service.go"): {
			"type BusinessGroupAssignment struct",
			"ObjectRef",
			"type BusinessGroupAssignmentQuery struct",
			"SaveBusinessGroupAssignment",
			"DeleteBusinessGroupAssignment",
			"product_catalog",
			"production_bom",
			"warehouse_inventory",
			"price_list",
		},
		filepath.Join("internal", "interfaces", "http", "catalog", "product_routes.go"): {
			"/api/business-group-assignments",
			"saveBusinessGroupAssignmentAPI",
			"deleteBusinessGroupAssignmentAPI",
			"classification write APIs are legacy readonly",
		},
		filepath.Join("internal", "infrastructure", "postgres", "catalog", "repository.go"): {
			"func (r Repository) ListBusinessGroupAssignments",
			"func (r Repository) SaveBusinessGroupAssignment",
			"func (r Repository) DeleteBusinessGroupAssignment",
			"save_business_group_assignment",
			"delete_business_group_assignment",
			"object_ref",
		},
		filepath.Join("internal", "infrastructure", "postgres", "bom", "repository.go"): {
			"business_group_assignments",
			"lower(bga.usage_key)='production_bom'",
			"lower(bga.object_key)='production_bom'",
			"saveBusinessGroupAssignmentForProductionBomTx",
		},
		filepath.Join("internal", "interfaces", "http", "bom", "bom_api.go"): {
			"production BOM groups are legacy readonly",
			"business_group_assignments",
		},
		filepath.Join("internal", "application", "stock", "service.go"): {
			"GroupID",
			"GroupItemID",
			"GroupSource",
		},
		filepath.Join("internal", "infrastructure", "postgres", "stock", "repository.go"): {
			"business_group_assignments",
			"lower(bga.usage_key)='warehouse_inventory'",
			"lower(bga.object_key)='warehouse'",
			"object_ref=w.code",
			"query.GroupID",
			"query.GroupItemID",
		},
		filepath.Join("internal", "interfaces", "http", "stock", "stock_api.go"): {
			"group_id",
			"group_item_id",
		},
		filepath.Join("internal", "application", "costing", "service.go"): {
			"group_source",
			"product_catalog",
			"price_list",
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-442 backend marker %q", rel, want)
			}
		}
	}
}

func TestDev442BusinessGroupAssignmentFrontendAndDocs(t *testing.T) {
	for rel, wants := range map[string][]string{
		filepath.Join("frontend-vue-shell", "src", "views", "ProductSettingsView.vue"): {
			"data-pr442-product-group-assignments",
			"/api/business-group-assignments",
			"分组集 / 父组 / 子组",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "BomView.vue"): {
			"data-pr442-bom-business-groups",
			"/api/business-group-assignments",
			"production_bom",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "WarehouseInventoryView.vue"): {
			"data-pr442-warehouse-business-groups",
			"/api/business-group-assignments",
			"warehouse_inventory",
			"库存分组",
		},
		filepath.Join("frontend-vue-shell", "src", "views", "CostingView.vue"): {
			"data-pr442-price-list-group-source",
			"group_source",
			"商品档案分组",
			"价格表覆盖",
		},
		filepath.Join("frontend-vue-shell", "src", "lib", "product-settings.js"): {
			"buildBusinessGroupAssignmentPayload",
			"businessGroupAssignmentLabel",
			"productCatalogGroupOfProduct",
		},
		filepath.Join("docs", "REQUIREMENTS.md"): {
			"PR-442-BUSINESS-GROUP-OBJECT-UNIFICATION",
			"business_group_assignments",
			"product_catalog",
			"warehouse_inventory",
		},
		filepath.Join("docs", "ACCEPTANCE_TESTS.md"): {
			"PR-442-BUSINESS-GROUP-OBJECT-UNIFICATION",
			"商品档案不再写旧商品分类字段",
			"仓库库存按仓库分组过滤",
		},
		filepath.Join("docs", "OP_MANUAL_INVENTORY_MATERIALS.md"): {
			"PR-442-BUSINESS-GROUP-OBJECT-UNIFICATION",
			"分组管理",
			"仓库库存分组对象是仓库",
		},
		filepath.Join("docs", "OP_MANUAL_COSTING.md"): {
			"PR-442-BUSINESS-GROUP-OBJECT-UNIFICATION",
			"商品档案分组",
			"价格表覆盖",
		},
		filepath.Join("scripts", "scenario_acceptance.py"): {
			"PR442-SCENARIO",
			"assign generated product to product_catalog group",
			"assign generated BOM to production_bom group",
			"assign existing warehouse code to warehouse_inventory group",
			"create generated customer capability template",
			"login generated customer miniapp user",
			"read customer settlement service",
			"mini settlement did not expose generated order",
			"disable generated customer portal visibility",
			"deactivate generated customer capability template",
			"/api/stock/warehouse-inventory?group_id={group_id}&group_item_id={group_item_id}",
			"group_source",
			`"onhand_g": 0`,
		},
	} {
		src := string(readOrderAppFileForTest(t, rel))
		for _, want := range wants {
			if !strings.Contains(src, want) {
				t.Fatalf("%s missing PR-442 frontend/docs marker %q", rel, want)
			}
		}
	}
}
