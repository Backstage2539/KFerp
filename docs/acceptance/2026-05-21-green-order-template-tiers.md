# 2026-05-21 生豆录单展示模板价档验收

## 背景
- 岩师傅客户的生豆 SKU 位于 `咖啡生豆 > 拼配豆` 分类，二级分类绑定了“岩师傅 - 生豆模板”。
- 已发布的岩师傅生豆豆单 V3.0.5 中，`兰卡拼配生豆` 和 `曲奇拼配2.0` 都有来自该模板的 1KG/60kG 生豆价档。
- 录单页此前从 `product_price_tiers` 读取商品梯度，生豆 SKU 没有展示发布豆单价档，容易看到空价或同名熟豆 SKU 梯度。

## 验收项
- [x] `/app/api/order/form?customer_id=152` 返回岩师傅生豆 SKU 时，`product_kind=green_bean` 的商品 `tiers` 来自最新已发布 `green` 豆单 `green_bean_sale_tiers`。
- [x] 生豆 SKU 的录单展示价档保留发布快照中的规格、数量区间、单价和价格来源，不读取绑定熟豆 SKU 或同名熟豆 SKU 的 `product_price_tiers`。
- [x] 熟豆 SKU 仍保留原熟豆/商用豆单或商品价格梯度，不被生豆豆单价档覆盖。

## 验证
- `go test ./internal/infrastructure/postgres/sales -run 'TestGreenBeanOrderPublicationTiersParseTemplatePrices|TestApplyGreenBeanOrderPublicationTiersReplacesDirectProductTiers'`
- `go test ./internal/interfaces/http/sales -run TestOrderAPIFormReturnsPublishedGreenBeanListTiersForGreenBeanProduct`
- 待部署后用 development API 冒烟确认岩师傅 `兰卡拼配生豆` 和 `曲奇拼配2.0` 生豆 SKU 返回 1KG/60kG 生豆价档。
