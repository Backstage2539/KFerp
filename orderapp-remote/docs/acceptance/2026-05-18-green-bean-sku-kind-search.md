# 生豆 SKU 形态搜索修复验收记录

## 问题

- 客户SKU列表 · 公共SKU 中，数据库/全部 SKU 存在生豆 SKU。
- 按产品形态筛选“生豆”时没有返回该 SKU。

## 根因

- 挂耳产品形态分支的 `NormalizeProductKind` 只保留了 `drip_bag` 和默认熟豆。
- 已有 `green_bean`、`green`、`raw_bean`、`生豆` 输入被归一成 `roasted_bean`，导致前端按 `product_kind=green_bean` 筛选时匹配不到。
- 后续复查发现，客户SKU列表被全局表格自动分页接管；生豆在后续页时，DOM 层分页与 Vue 筛选状态分离，可能造成用户选择“生豆”后看不到后页生豆。

## 修复

- `NormalizeProductKind` 恢复生豆形态支持，并兼容 `green`、`raw`、`raw_bean`、`生豆` 别名。
- `/api/product-settings` 返回公共 SKU 时保留 `product_kind:"green_bean"`，供客户SKU列表形态筛选使用。
- 保留挂耳 `drip_bag` 和熟豆默认兼容。
- SKU设置页改为显式 Vue 分页：先对当前归属下全部 SKU 执行形态/名称/分类筛选，再对筛选结果分页；客户SKU列表表格关闭全局自动分页。

## 验证

- `go test ./internal/domain/catalog -run 'TestNormalizeProductKind' -count=1`：通过。
- `go test ./internal/interfaces/http/catalog -run 'TestProductSettingsAPISupportsCategoryTreeAndDragAssignments' -count=1`：通过。
- `go test ./internal/domain/catalog ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog -count=1`：通过。
- `go test ./internal/domain/catalog ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/domain/costing ./internal/application/costing -count=1`：通过。
- `node --test src/lib/product-settings.test.js`：通过，覆盖“生豆在后页时，筛选生豆先匹配全量 SKU 再分页”。
- `npm run build`：通过。

## 手册

- `OP_MANUAL_COSTING.md` 已补充客户SKU列表筛选与分页规则。
