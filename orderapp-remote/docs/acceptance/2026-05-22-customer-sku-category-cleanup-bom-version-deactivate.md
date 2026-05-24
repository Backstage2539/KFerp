# PR-322 客户分类清理与 BOM 版本失效验收

## 范围
- SKU设置保存客户“是否使用公共商品分类”开关时，保留仍有活跃产品子类型的客户自有产品类型。
- SKU设置失效 SKU 时，同步把当前 BOM 标记为失效，并把该 SKU 的 active BOM 版本置为 disabled。

## 验收项
- 芬纳咖啡客户归属下，产品类型“咖啡豆”即使与公共分类同名，只要仍有活跃产品子类型“定制咖啡熟豆”，保存公共分类开关后也不得被清理或隐藏。
- `芬纳定制-红酒日晒-中深烘` 仍挂在“咖啡豆 / 定制咖啡熟豆”下时，公共分类开关往返后客户分类树仍显示该 SKU。
- 在 SKU设置 对有 active BOM 版本的 SKU 执行失效后，`product_bom.status=inactive`，对应 `bom_versions.status=active` 记录同步变为 `disabled`。
- 历史 BOM 明细和版本记录保留，可用于追溯；系统不执行物理删除。

## 验证证据
- RED：`go test ./internal/infrastructure/postgres/catalog -run 'TestCustomerPublicUsageCleanupKeepsOwnedParentsWithActiveChildren|TestDeactivateProductsDisablesActiveBomVersions' -count=1` 在实现前失败。
- GREEN：同一命令在实现后通过。
- 手册：`OP_MANUAL_COSTING.md` 与 `OP_MANUAL_INVENTORY_MATERIALS.md` 已补充客户分类开关和 BOM active 版本失效说明。
