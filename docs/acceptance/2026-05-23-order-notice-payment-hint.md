# PR-342-ORDER-NOTICE-DEDUP-PAYMENT-HINT

## 需求

- 同一个订单创建事件在 ERP 顶部只能展示一条新订单通知，不能因为当前用户同时命中订单读取权限和生产角色投递规则而出现重复通知。
- 录单或编辑订单进入需要填写收款凭证的状态时，货款金额栏需要显示当前商品合计的货款提示；点击货款提示后自动填入货款栏，填入后仍允许人工修改。

## 验收

- 创建一个订单后，顶部通知区只出现一条对应订单号的新订单通知。
- 同一通知列表返回重复投递行时，前端展示前会再次去重。
- 收款状态需要凭证且商品合计大于 0 时，货款金额输入框下方展示“货款 金额”的货款提示。
- 点击货款提示后，货款金额填入当前商品合计；用户随后可以继续编辑该金额。

## 证据

- `go test ./internal/application/messagecenter ./internal/interfaces/http/messagecenter ./internal/interfaces/http/support -run 'TestServiceDedupesNotificationsByEventID|TestMessageCenter|TestDev342' -count=1`
- `node --test src/lib/global-notifications.test.js src/lib/order-entry.test.js`
