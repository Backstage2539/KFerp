# PR-502 商品引用单位模板

## 范围

- 单位模板恢复为商品 UOM 主数据模板，维护库存单位、默认销售单位、可销售单位换算和整数规则。
- 商品档案通过 `products.unit_template_id` 引用单位模板；商品新增和编辑必须选择单位模板，普通 UI 不再展示商品级高级覆盖、库存单位、整数库存或销售单位换算直填入口。历史商品级覆盖继续兼容读取 `products.unit_rule_override_json`。
- 商品价格管理、阶梯价模板和价格模板不定义单位换算；商品价格表发布时按商品有效单位规则固化快照。
- BOM、生产计划、工单、WIP、成品入库和库存流水只读取商品有效库存单位，不读取销售单位或价格单位。

## RED

- `go test ./internal/infrastructure/postgres/catalog -run TestProductsReferenceUnitTemplatesAsPrimaryUOMMasterData -count=1`：商品列表和配置读取缺少 `unit_template_id`、`unit_template_name`、`unit_rule_source`，无法把单位模板作为商品 UOM 主数据来源。
- `go test ./internal/interfaces/http/catalog -run TestProductUnitTemplateReferenceAPIContract -count=1`：商品新增/编辑 API 不接收和返回 `unit_template_id`，且更新商品时可能把历史 `unit_rule_override_json` 误清空。
- `go test ./internal/infrastructure/postgres/costing -run TestProductSalesUnitResolversPreferProductDirectUnitTemplateBeforeLegacyTemplateChain -count=1`：商品价格表发布的单位解析缺少商品直接单位模板优先级。
- `go test ./internal/infrastructure/postgres/sales -run TestOrderFormProductsExposeProductTypeAndUnitRuleFields -count=1`：录单候选商品缺少商品直接单位模板的有效单位换算。
- `go test ./internal/infrastructure/postgres/bom -run TestBomRepositoryProductsUseDirectProductUnitTemplateBeforeLegacyFallbacks -count=1`：`/api/bom/products` 未按商品直接单位模板解析库存单位，也未把模板库存单位视为已配置来源。
- `node --test src/lib/product-settings.test.js`：商品新增/配置抽屉缺少单位模板主入口、批量设置单位模板；后续浏览器反馈要求移除 `不引用单位模板`、高级单位覆盖和商品级单位直填控件。

## GREEN

- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/sales ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales ./internal/application/production ./internal/interfaces/http/production -count=1`
- `node --test src/lib/product-settings.test.js src/lib/costing-price-list-workflow.test.js src/lib/order-entry.test.js src/lib/produce-plan.test.js`
- `npm ci`
- `npm run build`
- `scripts/verify_kferp.sh changed`
- `git diff --check`
- `codex review --uncommitted`：发现并修复两个 P2，分别是创建商品抽屉首次套默认单位模板时未关闭高级覆盖、costing product-input resolver 中商品直接单位模板优先级低于 legacy 配置；修复后重跑上述 Go/Node/build/verifier/diff check 均通过。

## 部署验证

- development 部署：feature branch pushed and fast-forwarded into `develop`，Docker build ran `go test ./...`，development stack rebuilt and restarted.
- Deploy backup：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260627202730`。
- Container smoke：`erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` running；unauthenticated `/app/` returned `303`；protected unauthenticated `/app/api/product-settings?limit=1` and `/app/api/bom/products` returned `401`。
- Authenticated API smoke：`/app/api/product-settings?limit=1`、`/app/api/bom/products`、`/app/api/production-boms?status=all&limit=1`、`/app/api/costing/bean-list` returned `200`。
- Field smoke：deployed `/api/product-settings` exposes `unit_template_id`、`unit_rule_source`、`inventory_unit`、`default_sales_unit`、`unit_conversion_json`；deployed `/api/bom/products` exposes `inventory_unit` and `inventory_unit_explicit`。
- Vue smoke：authenticated `/app/vue-shell/?view=productSettings` and `/app/vue-shell/?view=costing` returned `200`。

## Follow-up：商品档案单位模板入口

- RED：`node --test src/lib/product-settings.test.js` failed because 商品档案列表工具栏只有 `设置单位模板`，没有 `维护单位模板` 入口，也没有到 `productUnitTemplates` 的返回式 SPA 跳转。
- GREEN：商品档案列表工具栏新增 `维护单位模板`；点击后通过 `kferp:navigate-view` 进入 `productUnitTemplates`，并提供 `返回商品档案` 返回上下文。

## Follow-up：商品必须引用单位模板

- RED：`node --test src/lib/product-settings.test.js` failed because 商品新增/配置抽屉仍允许 `不引用单位模板`，并展示 `高级单位覆盖`、`库存单位`、`整数库存` 和 `销售单位换算` 直填控件；无模板保存还会把隐藏表单字段写入商品级 `unit_rule_override_json`。
- GREEN：商品新增/配置抽屉的单位模板下拉改为必选 `请选择单位模板`；删除普通 UI 中的高级单位覆盖、库存单位、整数库存和销售单位换算区块；保存时未选模板提示 `请选择单位模板`；payload 只在历史显式覆盖标记为 true 时写商品级单位覆盖，普通 UI 不再写入模板派生单位。
