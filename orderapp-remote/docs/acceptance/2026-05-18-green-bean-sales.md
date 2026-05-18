# PR-289 生豆销售验收记录

## 范围
- 在现有 `products` 产品体系中增加生豆/熟豆形态，不新增独立生豆商品表。
- 生豆产品销售价由管理员直接维护，和原料仓库批次成本分离，不参与熟豆成本试算发布覆盖。
- ERP 录单、订单列表、生豆豆单、小程序履约客户和小程序商城都能识别并下单生豆产品。

## 验收口径
- SKU设置新增或编辑生豆产品时，不要求烘焙度和 BOM 出品率，保存后产品列表能显示生豆标记。
- 生豆豆单读取 `green_bean_list` 与 `green_bean_sale_tiers`，PDF 标题为生豆豆单，价格来自管理员维护的销售档位。
- ERP 录单保存订单行时写入 `order_items.product_kind` 快照；订单列表通过汇总标记区分生豆/熟豆/混合订单。
- 小程序履约和商城订单都沿用 ERP 订单底层，提交生豆产品后订单行保留 `product_kind=green_bean`。
- 熟豆产品继续走既有成本试算、梯度模板和豆单发布逻辑。

## 自动化证据
- `go test ./internal/domain/catalog ./internal/application/costing ./internal/interfaces/http/catalog ./internal/interfaces/http/sales ./internal/infrastructure/postgres/customerportal ./internal/interfaces/http/support -run 'TestNormalizeProductKind|TestProductKindLabels|TestBeanListKeepsGreenBean|TestPublishBeanListValidates|TestProductSettingsAPI.*GreenBean|TestOrderAPISavesGreenBean|TestCreateFulfillmentOrderSavesGreenBean|TestCreateMallOrderSavesGreenBean|TestGreenBeanSales' -count=1`
- `go test ./... -count=1`
- `node --test src/lib/order-entry.test.js src/lib/bean-list-pdf.test.js src/lib/operation-manuals.test.js src/lib/menu-ia.test.js` in `orderapp-remote/frontend-vue-shell`
- `node --test src/lib/*.test.js src/api/*.test.js` in `orderapp-remote/frontend-vue-shell`
- `npm run build` in `orderapp-remote/frontend-vue-shell`
- `npm test -- --run src/utils/mall.test.ts` in `miniapp`
- `npm test -- --run` in `miniapp`

## 手册
- `OP_MANUAL_GREEN_BEAN_SALES.md`
- `OPERATION_MANUALS.md`
- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
