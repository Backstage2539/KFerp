# PR-341-ORDER-ENTRY-STALE-BEANLIST-AUTOLINE

## 范围
- 录单/编辑订单商品行使用非最新豆单发布版本时，豆单版本文字标红并显示感叹号提示。
- 商品明细“新增明细”按钮移动到列表下方；选择商品后自动补一个空明细，但全列表最多只保留一个空明细。

## 验收证据
- 单元测试：`node --test src/lib/order-entry.test.js` 覆盖 `rowUsesStaleBeanListPublication`、`needsTrailingBlankOrderLine` 和 `OrderEntryView` 静态接线。
- API 测试：`go test ./internal/interfaces/http/sales -run TestOrderAPIFormReturnsLatestBeanListVersionDefaultForStaleWarning -count=1` 覆盖 `/api/order/form` 返回旧版与最新版豆单，并把最新版标为 `is_default=true`。
- 手册：`OP_MANUAL_ORDER_SALES.md` 记录旧版豆单提示、手机点击提示和自动补空明细规则。

## 验收点
- 选择旧版豆单后录入商品，行内豆单版本标红，感叹号 hover/click 显示“非新版本豆单”。
- 选择商品后自动出现一个空明细；已有空明细时修改商品不新增第二个空明细。
- “新增明细”按钮位于明细列表下方。
