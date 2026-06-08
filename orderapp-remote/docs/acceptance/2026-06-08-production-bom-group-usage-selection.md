# PR-450 生产 BOM 使用分组与分组展示收敛（历史兼容）

## Scope

- 生产 BOM 页面不再把所有通用分组项直接展示为顶部筛选 Tab。
- PR-450 曾要求通过 `POST /api/business-groups/:id/usages` 显式启用 `production_bom` 用途；PR-453 后该接口仅作为历史兼容能力保留，普通生产 BOM 页面不再暴露 `使用分组`。
- PR-453 后，生产 BOM 页面直接选择启用的 `分组模板`，再通过 `移动到分类` 移动到 `未分类`、大类或小类；顶部分组 Tab 只展示当前模板下当前 BOM 列表实际归类使用过的分类项。
- 业务标签只显示父组 / 子组路径，不显示分组集名称前缀。
- `移动到分类` 位于目标分类选择左侧；分组 Tab 位于分组操作区下方、列表过滤区上方。
- 生产 BOM 表格不再单独展示“分组”列，分组分类由顶部 Tab 和当前列表范围体现。

## RED

- `node --test src/lib/product-settings.test.js src/lib/bom.test.js`：PR-450 实现前，BOM 页面缺少旧启用用途入口，Tab 和目标分组仍共用所有分组选项，标签仍带分组集前缀。
- `go test ./internal/interfaces/http/catalog -run 'TestBusinessGroup(ItemsAPIWritesGenericGroupItems|UsageAPIEnablesGenericGroupForProductionBOM)$' -count=1`：`POST /api/business-groups/:id/usages` 缺失，返回 405。
- Follow-up `node --test src/lib/bom.test.js`：分组 Tab 仍在操作区上方、仍从可移动分组候选派生，并且表格仍展示独立“分组”列。

## GREEN

- `node --test src/lib/product-settings.test.js src/lib/bom.test.js`：137/137 passed。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog -count=1`：passed。
- Follow-up `node --test src/lib/bom.test.js`：13/13 passed。

## Manual

- PR-453 已更新 `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md` 和 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`：生产 BOM 直接选择分组模板，再移动到分类；普通页面不再展示 `使用分组`。
