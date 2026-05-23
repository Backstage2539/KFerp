# PR-339-ORDER-ENTRY-KG-TIER-UNIT-VERSION-WARNING

## 验收范围
- 录单和编辑订单自动价以命中的豆单阶梯展示单位为准，KG 阶梯不因选择 80g、100g 等小包装而改成元/磅。
- 小计提示展示对应梯度单价，例如 `梯度 82/kg`，不展示内部梯度 ID。
- 商品行显示当前豆单版本号；能从豆单选到商品时不显示“未记录”。
- 低于最低梯度时红字提示，仍按最低档价格试算并允许保存订单。

## 验收证据
- 单元：`resolveWholesaleTierPrice keeps kg tier unit, source version, and below-min warning for small package orders` 覆盖前端单位、版本、低档提示和小计口径；`OrderEntryView shows tier unit price, bean list version without unrecorded fallback, and below-min warning` 覆盖界面接线。
- API：`TestPublishedPricingKeepsKgDisplayUnitForSmallCommercialPack` 覆盖发布豆单 KG 价格解析；`TestOrderAPISavesBeanListKgDisplayUnitForSmallPackageOrder` 覆盖订单保存 80g × 1 时落库 82 元/kg、小计 6.56、豆单版本和价格来源。
- 手册：`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md` 记录 KG 阶梯小包装、最低档红字提示和豆单版本显示规则。
