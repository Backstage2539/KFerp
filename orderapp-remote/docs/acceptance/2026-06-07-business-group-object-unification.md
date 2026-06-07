# PR-442 商品 / BOM / 仓库库存分组逻辑重构验收记录

## 范围
- PR-442-BUSINESS-GROUP-OBJECT-UNIFICATION：商品管理、生产 BOM、仓库库存统一使用 `分组管理` 和 `business_group_assignments`。
- 商品档案归组用途：`product_catalog`。
- 生产 BOM 归组用途：`production_bom`。
- 仓库库存归组用途：`warehouse_inventory`，归组对象是仓库 code。
- 商品价格表快照覆盖用途：`price_list`，覆盖只影响当前价格表版本。

## 验收点
- 商品档案不再写旧商品分类字段，列表按用户维护的业务分组路径展示商品；普通页面不显示迁移兼容用的系统默认分组集名称，也不把系统默认迁移分组下的旧分组项作为分组管理内容或移动候选；系统默认迁移归组按未分组处理。
- 生产 BOM 不再写旧 `group_id/group_category_id`，旧专用分组写入口只读兼容。
- 仓库库存按仓库分组过滤，不影响库存数量、批次、成本或追溯。
- 商品价格表默认读取商品档案分组；价格表覆盖发布后固化 `group_source=price_list`。
- 归组新增、修改、删除写操作日志。

## 证据
- Target Go/API: `go test ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/stock ./internal/interfaces/http/support -run 'TestBusinessGroupAssignmentsSupportStringObjectRefsAndAudit|TestProductWritesUseBusinessGroupAssignmentsInsteadOfLegacyCategoryColumns|TestProductionBomGroupingUsesBusinessGroupAssignments|TestProductionBomLegacyGroupWritesAreReadonlyCompatibility|TestWarehouseInventoryGroupingUsesWarehouseBusinessGroupAssignments|TestDev442' -count=1`
- Frontend: `node --test src/lib/product-settings.test.js`
- Full verification: `node --test src/lib/product-settings.test.js src/lib/bean-list-pdf.test.js src/lib/product-bean-list-split.test.js src/lib/costing-bean-list-version-ui.test.js`, `npm run build`, `go test ./...`, `scripts/verify_kferp.sh changed`, `git diff --check`.
- Deploy: `origin/develop=627c1befccfaad0de9a7592ff2fa09d00d80423c` deployed to development. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260607205547`.
- Post-deploy scenario: `PR442-SCENARIO-20260607-BNQEZR` passed through generated customer/material/product/group/BOM/warehouse归组/Pricing Rule/阶梯模板/customer reference/price list/order, then cleanup returned OK for all generated data.
- Cleanup verification: `/api/stock/warehouse-inventory?q=PR442-SCENARIO-20260607-BNQEZR` returned 0 rows; generated material `53` has `onhand_g=0` and batch-location sum `0`; `/api/business-groups?usage_key=warehouse_inventory` returned no `PR442-SCENARIO` groups.
- Browser acceptance: 商品档案、分组管理、生产 BOM、仓库库存、商品价格表、录单 all loaded in deployed Vue shell with no SQL/TypeError/加载失败 text; 仓库库存 displayed `库存分组` and no `PR442-SCENARIO` residue.
