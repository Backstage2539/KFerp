# 验收记录：生豆豆单手工磅价发布与录单展示

日期：2026-05-21

## 范围

- ERP 录单选择客户生豆 SKU 时，只读取该客户最新已发布 `green` 生豆豆单快照。
- 生豆豆单的阶梯区间按发布模板保持 KG 档位，例如 `1-59kg`、`60kg+`。
- 生豆销售单价按发布快照中的元/磅价展示，例如 `23.49/磅`、`62/磅`，不把磅价误显示为 `/kg`。
- 产品豆单中手工修改的生豆档位价按元/磅写入草稿和发布内容；复制价格源或后端发布兜底都不能把手工价还原为成本参考价。
- 不回退到旧版生豆豆单、同名熟豆 SKU 梯度或商品直连熟豆梯度。

## 验收点

- [ ] 岩师傅“兰卡拼配生豆”录单梯度展示为 `1-59kg 23.49/磅` 和 `60kg+ 62/磅`。
- [ ] `/app/api/order/form?customer_id=...` 返回的生豆 SKU 梯度来自最新已发布 green 豆单，`unit_price` 为发布快照 `price_per_lb`，`price_source_json` 保留 `price_unit=lb`。
- [ ] 前端价格提示保留 KG 区间，但单价单位显示 `/磅`。
- [ ] 产品豆单把 `60kg+` 手工改为 62 后，发布内容 `green_bean_sale_tiers` 含 `price_unit=lb`、`price_per_lb=62`，不再显示或入单为 51.75。
- [ ] 保存订单时仍记录所选生豆豆单发布 ID 和版本号，缺少生豆豆单价格时保存失败，不使用熟豆阶梯兜底。

## 验证证据

- 单元测试：`node --test frontend-vue-shell/src/lib/order-entry.test.js`、`node --test frontend-vue-shell/src/lib/bean-list-pdf.test.js` 通过。
- API/应用层测试：`go test ./internal/application/costing -run TestPublishGreenBeanListAppliesManualLbPriceOverridesToContentSnapshot -count=1` 通过。
- 静态解析测试：`go test ./internal/infrastructure/postgres/sales -run TestGreenBeanOrderPublicationTiersParseTemplatePrices -count=1` 通过。
- API 测试：`go test ./internal/interfaces/http/sales -run 'TestOrderAPIFormReturnsPublishedGreenBeanListTiersForGreenBeanProduct|TestOrderAPISavesGreenBeanOrderUsingSelectedGreenBeanListPublication' -count=1` 通过。
