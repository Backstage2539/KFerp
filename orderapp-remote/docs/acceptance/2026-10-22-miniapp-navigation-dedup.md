# PR-712 客户小程序入口去重与导航整改

## 范围与根因

首页动作、摘要和底栏分别硬编码；底栏去重后摘要重新添加同业务入口，且订单的 source=official 参数未同步。源码片段测试没有组合检查实际整页导航，手册留下互相矛盾的历史描述。

- DEV-829：去掉四类摘要、客户及访客价格表横幅、重复切换用户操作；删除摘要查询。
- DEV-830：共享目的地、权限、子页底栏；正式订单保留筛选及代加工发货入口。
- DEV-831：手册、回归、dev/main/生产与匹配小程序包交付。

## TDD 与自动验证

- 实施中；RED/GREEN、完整测试与构建结果将在完成后补充。
- API 沿用后端现有权限和客户隔离，无业务写入或数据库迁移。

## 手册

- docs/OP_MANUAL_CUSTOMER_PORTAL.md
- docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md，Vue/Vite customerPortalManual/customerFulfillmentManual 直接读取此单一来源。

## 交付与边界

- 实施中，未部署此需求。微信上传/审核/发布与Van业务验收单独记录。
