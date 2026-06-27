# PR-503 单位模板多销售单位

## Scope
- 单位模板作为商品 UOM 主数据模板，只维护一个库存单位和多个销售单位。
- 每个销售单位保存为 `1 销售单位 = 数量 库存单位`；库存单位自身固定 `1:1`。
- 商品档案继续强制选择单位模板，不展示商品级单位覆盖或销售单位换算入口。
- 商品价格表只选择价格单位，发布时后端按商品有效单位模板重读换算并固化快照；BOM、生产和库存只使用库存单位。

## RED
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog -run 'ProductUnitTemplate|MultiSales|UnitTemplate|GlobalUnit' -count=1`：实现前缺少 `default_sales_unit/sales_units` 合同、旧销售单位链式换算归一化和默认销售单位换算校验。
- `node --test src/lib/product-settings.test.js`：实现前单位模板 payload 仍只支持一个销售单位，页面仍显示旧 `销售单位` 单字段和通用 `单位换算` 行。

## GREEN
- `TMPDIR=$PWD/.tmp-go go test ./internal/application/catalog ./internal/interfaces/http/catalog -run 'ProductUnitTemplate|MultiSales|UnitTemplate|GlobalUnit' -count=1`
- `TMPDIR=$PWD/.tmp-go go test ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres/costing -run 'ProductUnitTemplate|SalesUnit|PriceList|ResolveProductSalesUnit' -count=1`
- `node --test src/lib/product-settings.test.js src/lib/order-entry.test.js src/lib/produce-plan.test.js`

## Acceptance
- 单位模板页显示 `库存单位 / 默认销售单位 / 销售单位换算`，不显示 `报价单位 / 录单单位`。
- 新建“咖啡豆单位”模板时可选择库存 `kg`，新增销售单位 `盒 / 磅`，换算分别为 `0.2 kg` 和 `0.453592 kg`。
- `/api/product-settings` 返回模板 `default_sales_unit=盒`、`sales_units=["kg","盒","磅"]` 和规范化 `unit_conversion_json`。
- 商品引用该模板后，商品价格表价格单位只能选择模板可销售单位；生产 BOM 产出单位仍显示库存单位 `kg`。
