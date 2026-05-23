# PR-336-MOBILE-NOTICE-STACK

## 需求
手机端顶部提示、绿色新订单通知和红色录单错误提示必须支持排列展示，不能互相覆盖。

## 验收口径
- 顶部 ERP 站内通知最多展示 3 条。
- 多条新订单通知按层叠卡片展示，点击或关闭单条通知只处理当前通知。
- 录单红色保存错误读取顶部通知堆叠高度并自动下移。
- 红色错误提示与绿色/信息通知分开展示，不出现重合遮挡。

## 验证证据
- 前端单测：`node --test src/lib/global-notifications.test.js src/lib/order-entry.test.js`
- API/需求表测试：`go test ./internal/interfaces/http/sales ./internal/interfaces/http/support`
- 前端构建：`npm run build`
- 空白检查：`git diff --check`
