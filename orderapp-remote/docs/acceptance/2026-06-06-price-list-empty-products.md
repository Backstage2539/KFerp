# PR-437-PRICE-LIST-EMPTY-PRODUCTS 验收记录

## 范围
- 商品价格表当前归属、客户或分类下没有可用商品时，页面不得显示 `products required`。
- `/api/costing/bean-list` 空目录返回空 `items`；`/api/costing/calculate` 空 `products` 仍保持错误校验。

## 自动化证据
- RED：`go test ./internal/application/costing -run TestBeanListAllowsEmptyProductCatalog -count=1` 在修复前失败，错误为 `BeanList() error = products required, want empty response`。
- GREEN：`go test ./internal/application/costing -run 'TestBeanListAllowsEmptyProductCatalog|TestCalculateRejectsEmptyProducts|TestBeanListPreservesCustomerAliasAndProductSnapshots' -count=1`。
- GREEN：`go test ./internal/interfaces/http/costing -run TestBeanListAPIReturnsEmptyItemsWhenCatalogHasNoProducts -count=1`。
- GREEN：`go test ./internal/interfaces/http/support -run TestDev437PriceListEmptyProducts -count=1`。

## 手工验收口径
- 打开 `商品价格表`，当前范围如果商品数为 0，应显示空状态或 `商品数 0`。
- 页面不得弹出 `products required`。
- 发布/生成价格表按钮在空内容时保持不可发布。
