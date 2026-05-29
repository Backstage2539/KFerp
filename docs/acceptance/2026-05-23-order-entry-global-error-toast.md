# PR-334-ORDER-ENTRY-GLOBAL-ERROR-TOAST

## 需求
录单或修改订单保存失败时，错误提示必须在当前视口内全局弹出，避免操作员滚动到页面底部点击保存后看不到页面顶部错误。

## 验收口径
- 缺少收款凭证、货款金额、运费金额、物流公司、物流产品、收款方式或商品明细时，保存失败错误在全局错误提示中展示。
- 全局错误提示固定在当前视口内，不随页面滚动离开视线。
- 错误提示可关闭。
- 客户抽屉内新增客户错误仍保留在抽屉内，不被主录单错误提示替代。

## 验证证据
- 前端单测：`node --test src/lib/order-entry.test.js`
- API/需求表测试：`go test ./internal/interfaces/http/support -run TestDev334 -count=1`
