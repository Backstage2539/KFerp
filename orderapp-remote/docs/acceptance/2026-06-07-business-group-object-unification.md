# PR-442 商品 / BOM / 仓库库存分组逻辑重构验收记录

## 范围
- PR-442-BUSINESS-GROUP-OBJECT-UNIFICATION：商品管理、生产 BOM、仓库库存统一使用 `分组管理` 和 `business_group_assignments`。
- 商品档案归组用途：`product_catalog`。
- 生产 BOM 归组用途：`production_bom`。
- 仓库库存归组用途：`warehouse_inventory`，归组对象是仓库 code。
- 商品价格表快照覆盖用途：`price_list`，覆盖只影响当前价格表版本。

## 验收点
- 商品档案不再写旧商品分类字段，列表展示“分组集 / 父组 / 子组”。
- 生产 BOM 不再写旧 `group_id/group_category_id`，旧专用分组写入口只读兼容。
- 仓库库存按仓库分组过滤，不影响库存数量、批次、成本或追溯。
- 商品价格表默认读取商品档案分组；价格表覆盖发布后固化 `group_source=price_list`。
- 归组新增、修改、删除写操作日志。

## 证据
- Target Go/API: `go test ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/stock ./internal/interfaces/http/support -run 'TestBusinessGroupAssignmentsSupportStringObjectRefsAndAudit|TestProductWritesUseBusinessGroupAssignmentsInsteadOfLegacyCategoryColumns|TestProductionBomGroupingUsesBusinessGroupAssignments|TestProductionBomLegacyGroupWritesAreReadonlyCompatibility|TestWarehouseInventoryGroupingUsesWarehouseBusinessGroupAssignments|TestDev442' -count=1`
- Frontend: `node --test src/lib/product-settings.test.js`
- Full verification and browser acceptance are recorded in `ACTIVE_REQUIREMENTS.md` after deployment.
