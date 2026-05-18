# 生豆 SKU 形态搜索修复验收记录

## 问题

- 客户SKU列表 · 公共SKU 中，数据库/全部 SKU 存在生豆 SKU。
- 按产品形态筛选“生豆”时没有返回该 SKU。

## 根因

- 挂耳产品形态分支的 `NormalizeProductKind` 只保留了 `drip_bag` 和默认熟豆。
- 已有 `green_bean`、`green`、`raw_bean`、`生豆` 输入被归一成 `roasted_bean`，导致前端按 `product_kind=green_bean` 筛选时匹配不到。

## 修复

- `NormalizeProductKind` 恢复生豆形态支持，并兼容 `green`、`raw`、`raw_bean`、`生豆` 别名。
- `/api/product-settings` 返回公共 SKU 时保留 `product_kind:"green_bean"`，供客户SKU列表形态筛选使用。
- 保留挂耳 `drip_bag` 和熟豆默认兼容。

## 验证

- `go test ./internal/domain/catalog -run 'TestNormalizeProductKind' -count=1`：通过。
- `go test ./internal/interfaces/http/catalog -run 'TestProductSettingsAPISupportsCategoryTreeAndDragAssignments' -count=1`：通过。
- `go test ./internal/domain/catalog ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog -count=1`：通过。
- `go test ./internal/domain/catalog ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/domain/costing ./internal/application/costing -count=1`：通过。

## 手册

- 本次是已有形态筛选契约修复，没有新增入口、字段或操作流程；操作手册无需改动。
