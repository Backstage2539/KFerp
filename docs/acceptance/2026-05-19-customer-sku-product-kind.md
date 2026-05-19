# 客户专属 SKU 产品形态验收记录

## 背景
- SKU设置进入客户归属后，“新增公共产品”隐藏，只展示“客户专属 SKU”创建表单。
- 原客户专属 SKU 表单只保留基础产品、定制类型、烘焙度和复制开关，缺少生豆属性、绑定熟豆、挂耳克重/盒数等形态配置。
- 前端对 `drip_bag` 也会按默认熟豆归一，后续保存有覆盖挂耳形态的风险。

## 验收点
- 客户专属 SKU 创建表单提供产品形态选择：熟豆、生豆、挂耳。
- 生豆客户 SKU 没有基础产品，创建时只提交 `product_kind=green_bean`、`green_bean_type` 和绑定熟豆 `green_bean_bom_product_id`。
- 熟豆和挂耳的基础产品下拉按当前形态筛选公共 SKU，避免用熟豆基础产品创建挂耳客户 SKU。
- 挂耳客户 SKU 创建时提交 `product_kind=drip_bag`、`drip_bag_grams` 和 `drip_box_bag_count`。
- 客户 SKU 列表和行内保存保留 `drip_bag`，不会把挂耳保存成默认熟豆。
- 客户专属挂耳复制价格梯度时保留 `product_kind`、`price_basis`、`sales_unit`、`unit_bag_count` 和 `price_source_json` 快照字段。

## 测试证据
- `node --test src/lib/product-settings.test.js`
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog -run 'TestCreateCustomProductAcceptsCustomerGreenBeanWithoutRoastLevel|TestProductSettingsAPICreatesCustomerGreenBeanCustomProduct|TestProductSettingsAPICreatesCustomerCustomProduct|TestCreateCustomProductCopiesDripProductMetadata|TestCreateCustomProductAllowsGreenBeanWithoutBaseProduct' -count=1`
