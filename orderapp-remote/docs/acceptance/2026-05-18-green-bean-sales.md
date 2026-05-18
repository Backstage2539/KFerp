# PR-289 生豆销售验收记录

## 范围
- 在现有 `products` 产品体系中增加生豆/熟豆形态，不新增独立生豆商品表。
- 生豆 SKU 不直接绑定原料；单品和拼配生豆都绑定对应熟豆 BOM。
- 生豆豆单价格由绑定熟豆 BOM 的成本输入和分类梯度模板生成，发布后 ERP 录单和小程序履约订单按已发布生豆豆单价格入单。
- 入库质检保留为原料入库必要流程，跟 BOM 和生豆豆单无直接取数关系。

## 验收口径
- SKU设置新增或编辑生豆产品时，不要求烘焙度、BOM 出品率或销售价，必须填写单品/拼配并绑定熟豆 BOM。
- 客户 SKU 列表 · 公共 SKU 支持形态、名称、一级分类、二级分类过滤，列表不提供销售价列。
- 原料入库记录产季、产地、产家风味描述；入库质检记录工厂风味描述、水分、密度。
- 生豆豆单读取 `green_bean_list` 与 `green_bean_sale_tiers`，PDF 标题为生豆豆单，质检信息取绑定熟豆 BOM 对应产品最新通过生产质检。
- ERP 录单保存订单行时写入 `order_items.product_kind` 快照，规格支持自定义克数；订单列表通过汇总标记区分生豆/熟豆/混合订单。
- 小程序履约和商城订单都沿用 ERP 订单底层，提交生豆产品后订单行保留 `product_kind=green_bean`。

## 自动化证据
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/infrastructure/postgres -run 'Catalog|Product|Schema|Test' -count=1`
- `go test ./internal/domain/costing ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing -count=1`
- `go test ./internal/infrastructure/postgres/orderbeans ./internal/interfaces/http/sales ./internal/infrastructure/postgres/customerportal -count=1`
- `go test ./internal/application/stock ./internal/interfaces/http/stock ./internal/infrastructure/postgres/stock -count=1`
- `go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production -count=1`
- `node --test src/lib/product-settings.test.js src/lib/order-entry.test.js src/lib/customer-mall.test.js src/lib/bean-list-pdf.test.js`
- `npm run build` in `orderapp-remote/frontend-vue-shell`
- `npm test -- mall.test.ts beanListDisplay.test.ts` and `npm run typecheck` in `miniapp`

## 手册
- `OP_MANUAL_GREEN_BEAN_SALES.md`
- `OP_MANUAL_INVENTORY_MATERIALS.md`
- `OP_MANUAL_CUSTOMER_PORTAL.md`
- `OPERATION_MANUALS.md`
- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
