# PR-321-CUSTOMER-SKU-CATEGORY-TOGGLE-FACTORY-BEANLIST-TITLE

## 验收目标
- 客户 SKU 设置中勾选再取消“是否使用公共商品分类”后，客户自有分类和客户 SKU 不消失。
- 从客户账户切回“工厂总览”后，SKU设置 回到公共SKU归属。
- 客户产品豆单的商用批发豆单标题与公共豆单一致，使用实际豆单分类名，例如 `1、定制咖啡熟豆`。

## 验收步骤
1. 进入客户账户模式，当前客户选择芬纳咖啡。
2. 打开 SKU设置，确认客户自有一级分类 `咖啡豆`、二级分类 `定制咖啡熟豆`，且 `芬纳定制-红酒日晒-中深烘` 已挂到该二级分类。
3. 勾选“是否使用公共商品分类”，再取消勾选。
4. 点击顶部“工厂总览”，再进入 SKU设置。
5. 进入产品豆单，豆单范围选择芬纳咖啡，查看商用批发豆单。

## 通过标准
- 第 3 步后客户自有 `咖啡豆 / 定制咖啡熟豆` 仍在分类树中，`芬纳定制-红酒日晒-中深烘` 仍挂在该二级分类下。
- 第 4 步后 SKU设置 显示公共SKU，不再残留芬纳咖啡客户 SKU 归属。
- 第 5 步商用批发豆单包含 `芬纳定制-红酒日晒-中深烘`，条目编码按客户分类排序，分类标题显示 `1、定制咖啡熟豆`。

## 验证证据
- 前端单元：`node --test src/lib/product-settings.test.js`
- 成本单元：`go test ./internal/domain/costing -run 'TestCustomer(CustomRoastBeanListUsesSkuCategoryMetadata|AliasBeanListOverridesExcelCategoryWithSkuCategory)' -count=1`
- API：`go test ./internal/interfaces/http/costing -run TestCostingCalculateAPIReturnsCustomerSkuCategoryBeanListMetadata -count=1`
- Schema 守护：`go test ./internal/infrastructure/postgres/catalog -run TestProductCategorySchemaRepairsActiveChildrenWithInactiveParents -count=1`
- 支持层守护：`go test ./internal/interfaces/http/support -run TestDev321CustomerSkuCategoryToggleFactoryBeanListTitle -count=1`
