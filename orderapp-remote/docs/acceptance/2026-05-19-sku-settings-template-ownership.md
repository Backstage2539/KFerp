# SKU设置公共模板/客户私有化验收记录

## 需求
- SKU设置中，客户可尽量复用公共商品分类、公共 SKU、公共梯度模板。
- 客户需要修改分类名称、SKU 名称或创建自己的梯度时，必须形成客户私有数据，不能直接覆盖公共模板。
- 客户专属 SKU 拖到公共分类时，系统应先复制公共分类路径，再把 SKU 放到客户分类中。

## 实现
- `customer_sku_public_usage` 新增 `use_public_gradient_templates`，商品分类、SKU、梯度模板三个公共引用开关独立保存并审计。
- `product_categories` 和 `pricing_gradient_templates` 增加来源 ID 与模板状态；公共行为 `public_template`，客户派生行为 `derived_from_public`。
- 新增 API：
  - `POST /api/product-settings/customer-categories/derive`
  - `POST /api/product-settings/customer-products/derive`
  - `POST /api/product-settings/customer-gradient-templates/derive`
- 客户上下文拖拽到公共分类、复制公共 SKU、点击公共梯度模板“复制为客户模板”、绑定公共梯度模板时均 copy-on-write 生成客户私有行，并写入操作日志。

## 验证
- `node --test src/lib/product-settings.test.js src/lib/gradient-templates.test.js`
- `node --test src/lib/*.test.js src/api/*.test.js`
- `npm run build`
- `go test ./...`
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog -count=1`
