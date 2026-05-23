# PR-335-ORDER-ENTRY-MOBILE-LAYOUT

## 需求
手机窄屏录单和修改订单页面必须完整展示弹窗、全局错误提示和输入控件，不能因条件面板或文件上传框撑宽页面而出现左右裁切。

## 验收口径
- 发货物流和收款凭证条件面板在手机宽度下按单列展示。
- 商品明细、价格提示和表单输入控件不产生横向滚动。
- 收款凭证上传使用可控的文件选择控件，文件名过长时省略展示，不裁切其他字段。
- 全局错误提示在手机安全区内显示，左右边缘不被裁掉，仍可关闭。

## 验证证据
- 前端单测：`node --test src/lib/order-entry.test.js`
- API/需求表测试：`go test ./internal/interfaces/http/support -run TestDev335 -count=1`
