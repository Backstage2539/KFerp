# PR-617 生产 BOM 失效物料门禁验收记录

日期：2026-08-30
目标环境：development
范围：生产 BOM 物料选项与失效产出物料错误处理；不修改 `main` 和 production。

## 自动验证口径

- Repository 源码合同：物料选项查询必须包含 `deprecated_at IS NULL`，创建物料产出 BOM 必须把空记录转换为“产出物料不存在或已失效”。
- 真实 PostgreSQL：有效与失效物料同时存在时，全量和按 ID 范围加载均只返回有效物料；使用失效产出物料创建 BOM 时整笔事务无 BOM、版本和操作日志写入。
- Handler/API：`POST /api/production-boms` 保持 HTTP 400，并返回稳定中文错误，不泄漏 `no rows in result set`。
- 发布门禁：定向 Go、全量验证器、development preflight、合入后的发布树验证与开发环境健康检查全部通过后才能部署。

## 开发环境验收口径

1. 已登录请求 `/api/bom/materials`，确认失效物料 ID 74 不存在、有效替代物料 ID 72 仍存在。
2. 用 ID 74 发送一笔不会落库的 BOM 创建请求，确认 HTTP 400、错误为“产出物料不存在或已失效”。
3. 查询 BOM、版本和操作日志，确认该失败请求没有业务写入。
4. 检查登录页、受保护接口、服务容器和 PostgreSQL 健康；业务页面最终点击验收由 Van 完成。

## 证据

- RED：`TestProductionBomMaterialOptionsAndOutputValidationGuardDeprecatedRows` 在实现前失败，原因是物料选项查询缺少失效过滤。
- GREEN：定向源码合同、真实 PostgreSQL 和 handler/API 测试通过；真实事务同时覆盖创建和编辑过期产出身份，确认原 BOM 产出不变。
- 完整门禁：Go 全量、Vue `1044/1044`、Vite `6596 modules`、小程序 `219/219`、类型检查、development 构建和远端隔离镜像预检均通过。
- 发布：行为修复合入并部署 `develop@50d07c54fc12d6e9aad8561bfe3ff23efac863fd`。源码备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260830003214-50d07c54fc12`，回滚镜像为 `kferp-orderapp-rollback:development-20260830003214-50d07c54fc12`。
- 健康：`erp_orderapp` 运行、`erp_postgres` healthy、登录页 HTTP 200、未认证受保护物料接口 HTTP 401，部署源码包含稳定中文错误门禁。
- development 已认证 API：`GET /api/bom/materials` 返回 54 个有效物料，ID 72 存在、ID 74 不存在；用 ID 74 创建得到 HTTP 400“产出物料不存在或已失效”，请求前后目标 BOM/版本计数均为 `0/0`。
- 边界：没有创建业务 BOM、版本或审计写入；`main`、production 和正式微信小程序均未操作。Van 仍需在 development 页面完成最终交互验收。
