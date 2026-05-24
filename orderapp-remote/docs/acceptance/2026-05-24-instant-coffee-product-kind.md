# PR-352-INSTANT-COFFEE-PRODUCT-KIND 验收记录

## 范围
- SKU设置新增 `instant_coffee` 速溶咖啡产品形态。
- `/api/product-settings/products` 可创建速溶咖啡 SKU，不要求烘焙度或 BOM 出品率。
- 录单商品候选和订单详情显示“速溶咖啡”形态。
- 生产计划缺 BOM 时，速溶咖啡默认原料为“速溶咖啡”，并补速溶包装盒，不再生成“产品名 生豆”。

## 验收证据
- Go 单元/API：`go test ./internal/domain/catalog ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -run 'TestNormalizeProductKind|TestProductKindLabels|TestCreateProductAcceptsInstantCoffee|TestProductSettingsAPICreatesInstantCoffee|TestInstantCoffee|TestDev351' -count=1`
- 前端单元：`node --test src/lib/product-settings.test.js src/lib/order-entry.test.js`
- 扩展验证：`go test ./internal/domain/catalog ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1`
- 前端构建：`npm run build`
