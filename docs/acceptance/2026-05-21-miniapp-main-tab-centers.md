# 小程序底部中心入口恢复验收

## 需求
- Van 反馈小程序下方“个人中心、订单中心、费用中心”不见了。
- 底部固定入口必须明确显示：首页、订单中心、费用中心、个人中心。
- 订单中心继续进入 `/pages/service/service?key=orders`，费用中心继续进入 `/pages/service/service?key=settlement`，个人中心继续进入 `/pages/profile/profile`。

## 实现
- `miniapp/src/components/MainTabBar.vue`：底部入口文案恢复为“订单中心 / 费用中心 / 个人中心”，并把底栏布局从 `grid` 调整为更适合小程序 WXSS 的 `flex` 固定栏，提升 z-index。
- `miniapp/src/utils/servicePage.ts`：订单与费用服务页标题同步为“订单中心”和“费用中心”。
- `REQUIREMENTS.md`、`ACCEPTANCE_TESTS.md`、`OP_MANUAL_CUSTOMER_PORTAL.md` 及 `orderapp-remote/docs/` 镜像文档同步更新操作口径。

## 证据
- RED：`npm test --prefix miniapp -- src/utils/mainTabs.test.ts` 曾失败，失败点为 `MainTabBar.vue` 缺少“订单中心”且仍使用 `display: grid`。
- GREEN：`npm test --prefix miniapp -- src/utils/mainTabs.test.ts src/utils/servicePage.test.ts src/utils/capabilities.test.ts src/utils/customerSwitch.test.ts` 通过，21/21。
- Unit：`npm test --prefix miniapp` 通过，58/58。
- API：`go test ./internal/interfaces/http/customerportal -run 'TestMini(OrdersServicePageAPI|SettlementServicePageAPI|ServicePageAPIRequiresToken)' -count=1` 通过。
- Support：`go test ./internal/interfaces/http/support -run TestDev278MiniappMainTabs -count=1` 通过。
- Typecheck：`npm run typecheck --prefix miniapp` 通过。
- Build：`VITE_KFERP_API_BASE=https://erp.qacoohee.com/app npm run build:mp-weixin --prefix miniapp` 通过。
- 构建产物检查：`miniapp/dist/build/mp-weixin/components/MainTabBar.js` 包含“订单中心 / 费用中心 / 个人中心”；`MainTabBar.wxss` 包含 `display:flex` 和 `z-index:999`。
- Whitespace：`git diff --check` 通过。

## 验收结论
- 通过。底部固定入口恢复为 Van 反馈的三个中心入口命名，并保持原有订单、费用、个人中心跳转能力。
