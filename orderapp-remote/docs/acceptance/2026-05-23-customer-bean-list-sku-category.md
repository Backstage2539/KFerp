# 2026-05-23 客户豆单分类按 SKU 设置验收

## 范围

- 客户豆单生成时，分类标题必须来自当前客户 SKU 分类。
- 客户 SKU 已挂分类时，商用/零售豆单使用该分类名和排序生成分类标题和商品编号。
- 客户 SKU 未挂分类时，豆单显示“未分类”，不能继承基础公共 SKU 的旧 Excel 豆单分类。

## 根因

- 岩师傅 `曲奇拼配2.0` 是客户 SKU，基础款为 `红岩2.0`。
- 线上该客户 SKU 当前未挂 `product_category_id`。
- 旧逻辑用基础款 `红岩2.0` 的 Excel 元数据生成豆单分类，因此错误显示为 `4、精品意式拼配：`。

## 验收点

- [x] 客户 SKU 未挂分类时，`CalculateProduct` 返回 `commercial_bean_list.category = "未分类"`，不再返回 `精品意式拼配`。
- [x] 客户 SKU 已挂分类时，`CalculateProduct` 使用 `SkuCategoryName`、`SkuCategoryPosition`、`SkuProductCategoryPosition` 生成分类和编号。
- [x] `/api/costing/calculate` 接口级测试覆盖上述两个场景。
- [x] 成本仓储从 `product_categories` 读取 SKU 分类名、分类排序和 SKU 在分类内排序。
- [x] 操作手册和需求/验收清单已同步说明客户豆单分类来源。

## 证据

- RED：`go test ./internal/domain/costing -run TestCustomerBeanListWithoutSkuCategoryDoesNotInheritExcelCategory -count=1`，失败显示返回 `4、精品意式拼配：`。
- RED：`go test ./internal/interfaces/http/costing -run TestCostingCalculateAPICustomerSkuWithoutCategoryDoesNotReturnExcelCategory -count=1`，失败显示接口返回 `4、精品意式拼配：`。
- GREEN：`go test ./internal/domain/costing -run 'TestCustomerBeanList(WithoutSkuCategoryDoesNotInheritExcelCategory|UsesSkuCategoryWhenPresent)' -count=1`。
- GREEN：`go test ./internal/interfaces/http/costing -run 'TestCostingCalculateAPICustomerSku(WithoutCategoryDoesNotReturnExcelCategory|UsesSkuCategory)' -count=1`。
- GREEN：`go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsReadsSkuCategoryMetadataForBeanList -count=1`。

