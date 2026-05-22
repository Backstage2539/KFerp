# PR-325-ORDER-ENTRY-MATCHED-TIER-HIGHLIGHT

## 验收范围
- 录单自动价已经命中豆单梯度并写入行 `tier_id` 后，梯度价格提示必须高亮同一 `tier_id` 的按钮。
- 当前规格和价格提示规格不一致时仍按实际命中档位高亮。例如芬纳咖啡 `芬纳定制-红酒日晒-中深烘` 选择 1000g、数量 20，单价显示 117 元/kg、行内显示 `梯度 58` 时，`454g 24-47件 53/磅` 价格提示必须高亮。
- `auto` 和 `manual` 状态不高亮任何豆单梯度，避免把未匹配或手动改价误判成自动命中。

## 验收证据
- 单元：`isOrderTierActive highlights fallback wholesale tier by matched id when selected spec differs` 覆盖 1000g 行命中 454g 档位 ID 时的高亮判断。
- 单元：`syncWholesaleTierPrice falls back to bean-list weight tiers when selected spec has no exact tier` 继续覆盖跨规格折算后返回实际命中的 `tierID`。
- 前端：`OrderEntryView.vue` 的价格提示按钮使用 `isOrderTierActive(row, tier)`，不再额外要求 `spec_mode` 和提示梯度 `specG` 相同。
- 手册：`OP_MANUAL_ORDER_SALES.md` 记录跨规格折算时仍高亮实际命中的豆单梯度。
