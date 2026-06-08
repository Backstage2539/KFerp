# PR-450 生产 BOM 使用分组与分组展示收敛

## Scope

- 生产 BOM 页面不再把所有通用分组项直接展示为顶部筛选 Tab。
- 生产 BOM 需要先在工具区点击 `使用分组`，把分组管理中的某套分组启用到 `production_bom` 用途后，才能移动 BOM 到该分组项。
- 顶部分组 Tab 只展示已经被生产 BOM 启用的分组项；业务标签只显示父组 / 子组路径，不显示分组集名称前缀。

## RED

- `node --test src/lib/product-settings.test.js src/lib/bom.test.js`：BOM 页面缺少 `使用分组` 入口，Tab 和目标分组仍共用所有分组选项，标签仍带分组集前缀。
- `go test ./internal/interfaces/http/catalog -run 'TestBusinessGroup(ItemsAPIWritesGenericGroupItems|UsageAPIEnablesGenericGroupForProductionBOM)$' -count=1`：`POST /api/business-groups/:id/usages` 缺失，返回 405。

## GREEN

- `node --test src/lib/product-settings.test.js src/lib/bom.test.js`：137/137 passed。
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog -count=1`：passed。

## Manual

- 更新 `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`：说明先 `使用分组` 再移动 BOM，顶部 Tab 只显示已启用分组项。
- 更新 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`：说明生产 BOM 分组来自通用分组管理，但启用状态由生产 BOM 功能自己保存。
