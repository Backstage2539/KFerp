# PR-437-PRICE-LIST-EMPTY-PRODUCTS 验收记录

## 范围
- 商品价格表当前归属、客户或分类下没有可用商品时，页面不得显示 `products required`。
- `/api/costing/bean-list` 空目录返回空 `items`；`/api/costing/calculate` 空 `products` 仍保持错误校验。

## 自动化证据
- RED：`go test ./internal/application/costing -run TestBeanListAllowsEmptyProductCatalog -count=1` 在修复前失败，错误为 `BeanList() error = products required, want empty response`。
- GREEN：`go test ./internal/application/costing -run 'TestBeanListAllowsEmptyProductCatalog|TestCalculateRejectsEmptyProducts|TestBeanListPreservesCustomerAliasAndProductSnapshots' -count=1`。
- GREEN：`go test ./internal/interfaces/http/costing -run TestBeanListAPIReturnsEmptyItemsWhenCatalogHasNoProducts -count=1`。
- GREEN：`go test ./internal/interfaces/http/support -run TestDev437PriceListEmptyProducts -count=1`。
- GREEN 合并后：`node --test src/lib/bom.test.js src/lib/product-settings.test.js`；`npm run build`；`go test ./...`；`scripts/verify_kferp.sh changed`；`git diff --check`。
- GREEN 部署：`./deploy_orderapp.sh` 完成，Docker build 内 `go test ./...` 通过；`erp_orderapp` 正常运行。
- GREEN smoke：`/app/vue-shell?view=costing` 认证访问 200；`/app/api/req/product?limit=500` 包含 `PR-437-PRICE-LIST-EMPTY-PRODUCTS`；`/app/api/costing/bean-list` 返回 200 且响应不包含 `products required`。

## 浏览器验收
- 打开 `商品价格表`，页面正常加载并显示已发布价格表和生成区域。
- 页面没有 `products required`，没有错误 alert。
- 当前公共豆单范围下空内容的 `生成价格表` 按钮保持禁用，不会创建空价格表。
