# 客户豆单 SKU 分类元数据验收

## 需求
- `PR-319-CUSTOMER-BEAN-LIST-SKU-CATEGORY-METADATA`
- 客户自有/客户定制熟豆进入客户范围产品豆单时，必须使用 SKU设置 的客户分类路径和商品排序生成商用豆单条目，分组标题与公共豆单一致，优先使用实际豆单分类名。

## 验收步骤
1. 在 SKU设置 选择芬纳咖啡，确认“是否使用公共商品分类”为关闭。
2. 新增或确认客户自有/客户定制熟豆 `芬纳定制-红酒日晒-中深烘`，挂到 `咖啡豆 / 定制咖啡熟豆`。
3. 进入客户账户模式的产品豆单，确认豆单范围为芬纳咖啡。
4. 展开“商用批发豆单”，检查分组和 SKU 条目。

## 通过标准
- `芬纳定制-红酒日晒-中深烘` 出现在芬纳咖啡客户范围的商用批发豆单预览和生成候选中。
- 分组名称来自 SKU设置 的实际豆单分类，例如客户 SKU 挂到 `咖啡豆 / 定制咖啡熟豆` 时显示 `1、定制咖啡熟豆`。
- 该 SKU 不再因为缺少旧 Excel 豆单编码而消失。
- 关闭公共商品分类时，不回落到旧公共豆单的 `源产地精选` 等公共分类。

## 验证证据
- `go test ./internal/domain/costing -run TestCustomerCustomRoastBeanListUsesSkuCategoryMetadata -count=1`
- `go test ./internal/interfaces/http/costing -run TestCostingCalculateAPIReturnsCustomerSkuCategoryBeanListMetadata -count=1`
- `go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsReadsSkuCategoryPathForCustomerBeanLists -count=1`
