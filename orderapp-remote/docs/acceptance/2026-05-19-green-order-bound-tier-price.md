# 2026-05-19 生豆录单绑定熟豆梯度兜底验收

## 范围
- 修复岩师傅“兰卡拼配生豆”在 ERP 录单后没有价格的问题。
- 适用于没有自身启用梯度、也没有可用已发布生豆豆单价格，但已绑定熟豆 BOM 且绑定熟豆存在启用梯度的客户可见生豆 SKU。
- 只调整 ERP 录单取价和保存兜底，不改变 SKU 设置页的生豆建档规则。

## 根因
- 前端录单保存 payload 把非挂耳商品统一写成 `roasted_bean`，导致生豆订单行形态快照丢失。
- `/app/api/order/form` 只返回商品自身梯度；岩师傅“兰卡拼配生豆”没有自身梯度，所以页面无法显示自动价。
- 后端保存生豆订单优先读已发布生豆豆单；该 SKU 没有可用生豆发布价时，没有继续使用绑定熟豆梯度兜底。

## 验收点
- [x] 前端保存 payload 对生豆商品输出 `product_kind=green_bean`。
- [x] `/app/api/order/form` 对无自身梯度的生豆 SKU 返回绑定熟豆启用梯度，价格来源标记为 `green_bean_bound_roasted_tier`。
- [x] 保存订单时，如果已发布生豆豆单未给出价格，后端按绑定熟豆梯度计算单价和小计，并在订单行价格来源记录绑定熟豆来源。
- [x] 生豆订单行仍保存 `product_kind=green_bean`，订单列表和详情可以继续区分生豆/熟豆。

## 验证证据
- `node --test src/lib/order-entry.test.js`
- `go test ./internal/infrastructure/postgres/sales -run 'TestOrderFormProductQueryExposesBoundRoastedTiersForGreenBeanProducts|TestOrderSaveUsesBoundRoastedTierFallbackForGreenBeanOrders|TestOrderFormProductQueryKeepsRoastLevelAndProductKindScanShape' -count=1`
- `go test ./internal/interfaces/http/sales -run 'TestOrderAPIFormReturnsBoundRoastedTiersForGreenBeanProduct|TestOrderAPISavesGreenBeanOrderUsingBoundRoastedTierFallback' -count=1 -v`（本地无测试数据库时跳过，保留为数据库环境 API 验收）
- 开发库只读核对：产品 414“兰卡拼配生豆”无自身梯度，绑定熟豆产品 146；兜底查询返回 1000g 的 24-49、50-99、100-199 三档，来源为 `green_bean_bound_roasted_tier`。

## 手册
- `OP_MANUAL_ORDER_SALES.md`
- `OP_MANUAL_GREEN_BEAN_SALES.md`
- `REQUIREMENTS.md`
- `ACCEPTANCE_TESTS.md`
