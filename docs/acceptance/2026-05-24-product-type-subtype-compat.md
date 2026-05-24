# PR-354-PRODUCT-TYPE-SUBTYPE-COMPAT 验收记录

## 范围
- SKU设置把原一级分类展示为“产品类型”，把原二级分类展示为“产品子类型”。
- 后端保留 `product_kind` 兼容字段，并把熟豆、生豆、挂耳、速溶咖啡映射到默认产品类型。
- 产品价格表继续复用原产品豆单的公共/客户专属、草稿/发布/撤回、版本快照和录单取价能力，不新增独立价格表模块。

## 验收证据
- Go 单元：`go test ./internal/domain/catalog -run 'TestLegacyKindDefaultTypeNameMapsExistingProductKinds|TestProductCategoryRoleLabels' -count=1`
- 前端单元：`node --test src/lib/product-settings.test.js`
- 支持/API：`go test ./internal/interfaces/http/support -run TestDev354 -count=1`
