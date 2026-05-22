# 2026-05-22 录单商品单价优惠验收记录

## 需求
- PR-310-ORDER-ENTRY-UNIT-AMOUNT-DISCOUNT

## 验收点
- 录单商品行“优惠”下拉包含“单价优惠”。
- 选择单价优惠后，前端小计按当前单价单位扣减：零售按件数，挂耳按袋/盒数量，454g 等按总磅数，1000g/2.5kg 等按总 kg。
- 保存订单时 `discount_type=unit_amount` 和 `discount_value` 传入后端，后端按同一计价基数计算 `order_items.discount_amount`、`order_items.line_total`、`orders.discount_amount` 和 `orders.grand_total`。
- 操作手册、需求和验收用例已同步说明单价优惠口径。

## 验证证据
- `node --test src/lib/order-entry.test.js`
- `go test ./internal/infrastructure/postgres/sales -run 'TestApplyOrderItemDiscountSupportsUnitAmount|TestOrderItemUnitDiscountUnitsUsesCurrentPriceUnit' -count=1`
- `go test ./internal/interfaces/http/sales -run 'TestOrderAPISaveCarriesUnitAmountItemDiscount' -count=1`
- `go test ./internal/interfaces/http/support -run TestDev310Order -count=1`
