# PR-339-ORDER-ENTRY-KG-TIER-UNIT-VERSION-WARNING

## 验收范围
- 录单和编辑订单自动价以命中的豆单阶梯展示单位为准，KG 阶梯不因选择 80g、100g 等小包装而改成元/磅。
- 小计提示展示对应梯度单价，例如 `梯度 82/kg`，不展示内部梯度 ID。
- 商品行显示当前豆单版本号；能从豆单选到商品时不显示“未记录”。
- PR-537 已取代原“低于最低梯度按最低档试算”的范围行为：低于起订量、高于有限最高量或落在档位空隙时，自动单价为空并提示“当前数量无已发布价格，不能保存”；调整数量、补齐价格档或按授权输入大于 0 的手动价后才能保存。

## 验收证据
- 单元：`resolveWholesaleTierPrice keeps kg tier unit and source version for small packages inside the published range` 覆盖前端单位、版本和合法档位小计口径，并覆盖低于、超出和档位空隙返回无自动价；`OrderEntryView` 合同覆盖严格范围提示和保存阻断。
- API：`TestPublishedPricingKeepsKgDisplayUnitForSmallCommercialPackInsideTier` 覆盖发布豆单 KG 价格解析；`TestOrderAPISavesBeanListKgDisplayUnitForSmallPackageInsidePublishedRange` 覆盖订单保存 80g × 313（25.04kg）时落库 82 元/kg、小计 2053.28、豆单版本和价格来源。
- 手册：`orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md` 记录 KG 阶梯小包装、严格价格范围、无价保存阻断和豆单版本显示规则。
