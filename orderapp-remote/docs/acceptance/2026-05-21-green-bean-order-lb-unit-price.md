# 验收记录：生豆豆单 KG 模板单价发布与录单展示

日期：2026-05-21

## 范围

- ERP 录单选择客户生豆 SKU 时，只读取该客户最新已发布 `green` 生豆豆单快照。
- 生豆豆单的阶梯区间按发布模板保持 KG 档位，例如 `1-59kg`、`60kg+`。
- 生豆销售单价按发布模板单位展示；KG 模板展示为元/KG，例如 `51.75/kg`、`62/kg`，不把 KG 模板误显示为 `/磅`。
- 产品豆单中手工修改的生豆档位价按模板单位写入草稿和发布内容；复制价格源或后端发布兜底都不能把手工价还原为成本参考价。
- 不回退到旧版生豆豆单、同名熟豆 SKU 梯度或商品直连熟豆梯度。

## 验收点

- [ ] 岩师傅“兰卡拼配生豆”录单梯度展示为 `1-59kg 51.75/kg` 和 `60kg+ 62/kg`。
- [ ] `/app/api/order/form?customer_id=...` 返回的生豆 SKU 梯度来自最新已发布 green 豆单，`unit_price` 为发布快照 `price_per_kg`，`price_source_json` 保留 `price_unit=kg`。
- [ ] 前端价格提示保留 KG 区间，单价单位也显示 `/kg`。
- [ ] 产品豆单把 `60kg+` 手工改为 62 后，发布内容 `green_bean_sale_tiers` 含 `price_unit=kg`、`price_per_kg=62`，不再显示或入单为 51.75。
- [ ] 保存订单时仍记录所选生豆豆单发布 ID 和版本号，缺少生豆豆单价格时保存失败，不使用熟豆阶梯兜底。

## 验证证据

- 单元测试：`node --test frontend-vue-shell/src/lib/order-entry.test.js`、`node --test frontend-vue-shell/src/lib/bean-list-pdf.test.js` 通过。
- API/应用层测试：`go test ./internal/application/costing -run TestPublishGreenBeanListAppliesManualKgPriceOverridesToKgContentSnapshot -count=1` 通过。
- 静态解析测试：`go test ./internal/infrastructure/postgres/sales -run TestGreenBeanOrderPublicationTiersParseTemplatePrices -count=1` 通过。
- API 测试：`go test ./internal/interfaces/http/sales -run 'TestOrderAPIFormReturnsPublishedGreenBeanListTiersForGreenBeanProduct|TestOrderAPISavesGreenBeanOrderUsingSelectedGreenBeanListPublication' -count=1` 通过。
