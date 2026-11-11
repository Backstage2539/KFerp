# PR-758 前道供应与调拨提示修复

- 范围：物料准备区将计划前道供应显示为“等待前道生产”；有现货部分时，单独显示现货缺料。已有待确认调拨只减少可新建数量，不增加 WIP 库存。
- 前端：针对纯前道、前道与现货混合、WIP 已覆盖、件数及待确认单据新增测试，先记录 RED，再实现 GREEN；历史已提交单据的重量显示补充回归，先 RED 后 GREEN；全量前端 1884 项、Vue 构建通过。
- 接口：`TestPlanTransferAPIHonoursUpstreamSupply` 在隔离 PostgreSQL 上验证纯前道与混合供应。直接创建或单项预览不能超出需现货数量；已有调拨草稿后批量只生成剩余数量，重复请求不重复开单，仓库库存不因创建草稿移动。该测试验证原有后端保护，不将其描述为修复前失败。
- 手册：`OP_MANUAL_PRODUCTION.md`、`OP_MANUAL_STOCK.md` 和生产计划页面说明已同步。
- 浏览器：本地 Vite 载入实际组件和模拟数据，桌面及 390×844 窄窗口检查了纯前道、混合供应及待确认单据的文案与按钮；窄屏可读。此检查并非真实接口端到端。
- 开发与正式环境已发布；未使用正式库存写入测试，真实业务验收待完成。

## 开发环境发布与只读检查

- 发布日志版本 `a4112c102fa2eefa15017328b2018c8f96ee035c`；应用 running、重启 0，数据库 healthy、重启 0，登录 200，未登录计划接口 401。
- 公网、主机与容器内 `ProducePlanView-DpcMsTq6.js` 的 SHA-256 均为 `8795180e8f12e76d2dd7088e0ca2d3c04fb804bf77dc85acf59f8359ed7bb0a2`，包含等待前道生产文案。PR/DEV 均为待验收。未做真实业务写入或真实业务页面端到端验收。
- 保留既有小程序包；回滚源 `/opt/stacks/erp/orderapp.backup.deploy-20261111095407-a4112c102fa2`，回滚镜像 `kferp-orderapp-rollback:development-20261111095407-a4112c102fa2`。

## 正式环境发布与只读检查

- 选择修复及开发证据提交，不包含后续通知、分次收款和小程序改动。正式发布日志版本 `1cae6999937e78f24ff6a2d4a3a7769afae7a03a`，原小程序 `1613c632b867` 保留。
- 候选前端 1884 项、构建、隔离 API 纯前道/混合场景、无数据库 Go 全量通过。生产接口隔离全量原有 21 项失败及生产仓储 1 项基线夹具失败未在本次修复；详见 `2026-11-10-production-material-preparation.md`。
- 应用 running、重启 0；数据库 healthy、历史重启 13；Caddy running、重启 0。登录 HTTP 200，未登录计划 API HTTP 401；PR 和两项 DEV 为待验收。
- 公网、主机和容器内 `ProducePlanView-DpcMsTq6.js` SHA-256 均为 `8795180e8f12e76d2dd7088e0ca2d3c04fb804bf77dc85acf59f8359ed7bb0a2`，包含“等待前道生产”。这些检查不等同正式业务页面实操验收。
- 发布前数据库备份 `/opt/stacks/erp-production/backups/pre-upstream-wait-20261111113656.dump`，权限 600，SHA-256 `4d15dab1e9a0556c2caef820acbdc83ed8f9f80d08ce04a13c7c299f9b966fac`；已验证 pg_restore 列表可读，未进行完整恢复演练。回滚源 `/opt/stacks/erp-production/orderapp.backup.deploy-20261111114035-1cae6999937e`，镜像 `kferp-orderapp-rollback:production-20261111114035-1cae6999937e`。未创建或过账真实业务单据。
