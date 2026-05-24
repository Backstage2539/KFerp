# PR-357-PRODUCT-PRICE-LIST-GENERALIZATION

## 范围
- 产品豆单页面升级为产品价格表页面。
- 继续复用 `bean_list_publications`，不另建一套价格表。
- 发布快照增加 `product_type_category_id` 和 `product_type_name`，支持按产品类型查询。
- 生成入口和版本列表的产品类型选项来自 SKU设置 的产品类型分类，不再写死商用、挂耳、零售、生豆四个选项。
- 没有旧 Excel 豆单编码的新产品类型 SKU 会按产品类型/产品子类型分类生成展示信息，避免“速溶咖啡”等新类型生成空表。
- 旧 `list_type`、旧 API 路由和 `/public/bean-list/:list_type` 保留兼容。

## 验收
- 页面主标题和主操作显示“产品价格表”“生成价格表”“发布价格表”。
- 发布价格表仍写入 `bean_list_publications`，并保留公共/客户专属、草稿/发布/撤回、版本快照。
- 页面在“产品类型”选择 SKU设置 中的“速溶咖啡”后，只生成该产品类型下的 SKU，并把 `product_type_category_id/product_type_name` 写入草稿、发布、下载和公共链接参数。
- 旧 `list_type=commercial/green/drip/retail` 可以继续查询原发布快照。
- 新 `product_type_category_id` 可以查询指定产品类型的产品价格表发布快照。

## 证据
- `go test ./internal/application/costing -run 'TestLegacyBeanListTypeProductTypeName|TestNormalizeBeanListCommandCarriesProductTypeFields|TestNormalizeBeanListPublicationQueryAllowsProductTypeCategory' -count=1`
- `go test ./internal/domain/costing -run TestPublicCustomProductBeanListUsesSkuCategoryMetadata -count=1`
- `go test ./internal/infrastructure/postgres/costing -run 'TestBeanListPublicationSchemaSupportsProductPriceListGeneralization|TestBeanListPublicationRepositoryQueriesByProductType' -count=1`
- `go test ./internal/interfaces/http/costing -run 'TestBeanListPublicationAPIPassesProductTypeCategory|TestPublicBeanListPagePassesProductTypeCategory|TestCostingViewUsesProductPriceListLanguage' -count=1`
- `node --test src/lib/product-bean-list-split.test.js --test-name-pattern "product bean-list generation uses product type categories"`
- `go test ./internal/interfaces/http/support -run TestDev357 -count=1`
