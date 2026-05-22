# 客户门户账号刷新和客户档案入口验收记录

- 日期：2026-05-22
- 需求：PR-311-CUSTOMER-PORTAL-ACCOUNT-PROFILE

## 验收点
- 客户门户配置中创建外部用户、重置密码或启停登录后，ERP 顶部“客户账户”的当前客户下拉立即刷新。
- 符合条件的批发客户（active 外部用户、手机号、密码、启用登录）不需要整页刷新即可出现在顶部客户账户下拉。
- 客户门户配置模板摘要里原“客户履约工作台”按钮改为“打开客户档案”，点击后打开当前客户的客户档案抽屉。

## 证据
- 前端单测：`node --test src/lib/workspace-mode.test.js src/lib/customer-portal-settings.test.js`
- 静态/文档守卫：`go test ./internal/interfaces/http/support -run TestDev311CustomerPortalAccountProfile -count=1`
- 线上只读复现：`/api/customer-fulfillment/customers?limit=200` 已返回“芬纳咖啡”，说明后端绑定有效，修复重点为前端顶部客户列表刷新。
