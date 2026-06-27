# PR-502 商品引用单位模板

## 范围

- 单位模板恢复为商品 UOM 主数据模板，维护库存单位、默认销售单位、可销售单位换算和整数规则。
- 商品档案通过 `products.unit_template_id` 引用单位模板；商品级高级覆盖继续写入 `products.unit_rule_override_json`，仅用于例外 SKU。
- 商品价格管理、阶梯价模板和价格模板不定义单位换算；商品价格表发布时按商品有效单位规则固化快照。
- BOM、生产计划、工单、WIP、成品入库和库存流水只读取商品有效库存单位，不读取销售单位或价格单位。

## RED

- `go test ./internal/infrastructure/postgres/catalog -run TestProductsReferenceUnitTemplatesAsPrimaryUOMMasterData -count=1`：商品列表和配置读取缺少 `unit_template_id`、`unit_template_name`、`unit_rule_source`，无法把单位模板作为商品 UOM 主数据来源。
- `go test ./internal/interfaces/http/catalog -run TestProductUnitTemplateReferenceAPIContract -count=1`：商品新增/编辑 API 不接收和返回 `unit_template_id`，且更新商品时可能把历史 `unit_rule_override_json` 误清空。
- `go test ./internal/infrastructure/postgres/costing -run TestProductSalesUnitResolversPreferProductDirectUnitTemplateBeforeLegacyTemplateChain -count=1`：商品价格表发布的单位解析缺少商品直接单位模板优先级。
- `go test ./internal/infrastructure/postgres/sales -run TestOrderFormProductsExposeProductTypeAndUnitRuleFields -count=1`：录单候选商品缺少商品直接单位模板的有效单位换算。
- `go test ./internal/infrastructure/postgres/bom -run TestBomRepositoryProductsUseDirectProductUnitTemplateBeforeLegacyFallbacks -count=1`：`/api/bom/products` 未按商品直接单位模板解析库存单位，也未把模板库存单位视为已配置来源。
- `node --test src/lib/product-settings.test.js`：商品新增/配置抽屉缺少单位模板主入口、批量设置单位模板和高级单位覆盖/清除覆盖。

## GREEN

- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/sales ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales ./internal/application/production ./internal/interfaces/http/production -count=1`
- `node --test src/lib/product-settings.test.js src/lib/costing-price-list-workflow.test.js src/lib/order-entry.test.js src/lib/produce-plan.test.js`
- `npm ci`
- `npm run build`
- `scripts/verify_kferp.sh changed`
- `git diff --check`
- `codex review --uncommitted`：发现并修复两个 P2，分别是创建商品抽屉首次套默认单位模板时未关闭高级覆盖、costing product-input resolver 中商品直接单位模板优先级低于 legacy 配置；修复后重跑上述 Go/Node/build/verifier/diff check 均通过。

## 待部署验证

- development 部署后 smoke：`/api/product-settings`、`/api/bom/products`、商品价格表发布相关 API 返回单位模板来源和快照字段正确。
