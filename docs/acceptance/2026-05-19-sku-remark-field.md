# SKU 备注字段

## 需求

- SKU设置需要增加“备注”项。
- 公共 SKU 和客户专属 SKU 都应支持备注。
- 客户上下文中引用的公共 SKU 只读展示公共备注，不把公共 SKU 复制为客户数据。

## 实现

- `products` 主档新增 `remark` 字段。
- `/api/product-settings`、`POST /api/product-settings/products`、`PUT /api/products/:id`、`POST /api/product-settings/custom-products` 均读写 `remark`。
- SKU设置新增公共产品表单、客户专属 SKU 表单和客户SKU列表行内编辑都展示备注；公共引用行禁用备注编辑。

## 验证

- `node --test src/lib/product-settings.test.js`
- `go test ./internal/interfaces/http/catalog -run 'TestProductSettingsAPISupportsCategoryTreeAndDragAssignments|TestProductSettingsAPIUpdatesProductRemark|TestProductSettingsAPICreatesCustomerCustomProduct|TestProductSettingsAPICreatesPublicProduct' -count=1`
- `go test ./internal/infrastructure/postgres/catalog -run TestProductRemarkPersistsOnProducts -count=1`
