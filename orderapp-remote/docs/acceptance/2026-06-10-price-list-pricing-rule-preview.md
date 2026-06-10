# PR-465-PRICE-LIST-PRICING-RULE-PREVIEW

## Scope
- 商品价格表预览对 `按价格模板计算` / `按价格计算模板计算` 的商品行使用商品价格管理同一套价格计算模板试算结果。
- 平铺价格行把试算出的最终价、报价单位、库存单位/换算、BOM 版本、工序和成本来源快照写入预览和 PDF 数据。
- `熟豆-红岩拼配` 选择 `咖啡熟豆磅装模板` 后，价格表预览显示该模板试算价格，不再因为没有旧来源档位价而为空或显示 0。
- 试算失败会保留错误状态，不在前端 watcher 中循环重试同一请求。

## Verification
- RED: `node --test src/lib/product-settings.test.js` failed before `applyPricingRuleTrialToPriceTableRow` existed.
- RED: `node --test src/lib/bean-list-pdf.test.js` failed before `applyPriceListFlatRowsToBeanListPdfGroups` existed.
- RED: `node --test src/lib/costing-bean-list-version-ui.test.js` failed before failed pricing-rule trial requests were cached as terminal error states.
- GREEN: `node --test src/lib/product-settings.test.js`.
- GREEN: `node --test src/lib/bean-list-pdf.test.js`.
- GREEN: `node --test src/lib/costing-bean-list-version-ui.test.js`.

## Manual Acceptance
- 打开 商品价格表，选择 `咖啡熟豆`。
- 确认 `意式拼配豆` 下的 `熟豆-红岩拼配` 已被选中。
- 将该商品计价设为 `按价格模板计算` / `咖啡熟豆磅装模板`。
- 确认价格表预览中 `熟豆-红岩拼配` 显示与商品价格管理模板试算一致的价格和报价单位。
- 生成 PDF 前确认预览不显示 0 元价格；没有试算结果时不得发布该商品价格行。
