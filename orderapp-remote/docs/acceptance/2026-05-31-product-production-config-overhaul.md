# PR-390 商品生产配置与 BOM 模型一把改完

## 范围
- 新增商品生产配置、商品生产配置字段和工艺路线结构。
- BOM 收敛为配方库：分组、BOM 档案、版本和配方明细；预期损耗率和产品信息字段迁到商品生产配置。
- 商品与配方菜单拆为商品档案、客户商品、商品配置模板、生产 BOM、产品价格表和行业字段模板。
- 成本、价格表、录单、生产计划和工单读取并冻结商品生产配置。

## RED 证据
- 前端：`node --test src/lib/menu-ia.test.js src/lib/bom.test.js src/lib/product-settings.test.js` 初始失败，缺少商品档案/客户商品/商品配置模板拆分、BOM 分组树和商品生产配置标记。
- 后端：`go test ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/manufacturing ./internal/infrastructure/postgres/costing ./internal/infrastructure/postgres/production ./internal/infrastructure/postgres/sales -count=1` 初始失败，缺少商品生产配置、工艺路线和 BOM 分组删除/排序能力。

## GREEN 证据
- 已通过定向 Go 测试：catalog、bom、manufacturing、costing、production、sales 相关 application/infrastructure/interfaces 包。
- 已通过前端定向测试：`node --test src/lib/menu-ia.test.js src/lib/bom.test.js src/lib/product-settings.test.js src/lib/product-bean-list-split.test.js src/lib/workspace-mode.test.js src/lib/customer-management-source.test.js`。
- 已通过 Go 全量测试：`go test ./...`。
- 已通过 Vue 构建：`npm run build`（仅保留既有 chunk-size warning）。
- 已通过 changed verifier：`scripts/verify_kferp.sh changed`，退出码 0。
- 按当前约定不做浏览器/人工验收；部署证据以最终部署记录为准。

## 手册证据
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/OPERATION_MANUALS.md`

## 备注
- 按 Van 当前要求，本轮不做浏览器/人工验收。
- 旧 `yield_rate`、BOM 版本特殊属性和 SKU 特殊属性不删除，仅作为历史兼容 fallback。
