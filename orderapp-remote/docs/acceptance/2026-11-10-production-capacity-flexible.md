# PR-757 产能差额与自定义批量验收

状态：自动验证通过，待发布 development 和 Van 业务验收。

- RED：旧逻辑拒绝 45/45.113kg 的不足安排，将 18.1kg × 2 预览为 30kg；新增预定义接口返回 404。前端 RED 同时覆盖差额阻断、自定义和重复提示。
- GREEN：`scripts/verify_kferp.sh backend` 全部通过；隔离 PostgreSQL 上针对产能的应用、仓储及 HTTP 测试通过，覆盖 18.1/18.5/18.7、计划提交、未开工工单编辑、incoming/piece 超欠排和快照。
- 预定义 API、权限及隔离 PostgreSQL 并发测试通过：6 个并发请求只创建一个同参数启用档，操作日志存在。
- 前端 node 测试 1833 项通过，Vue 构建通过；模拟数据浏览器检查 45/45.113、不足可确认、18.7×3 超排 10.987kg、加入预定义及手改 2 批不足 7.713kg。浏览器测试使用模拟 API，不写业务数据库。
- 更新 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`，由前端手册读取；产品验收由 Van 完成。
