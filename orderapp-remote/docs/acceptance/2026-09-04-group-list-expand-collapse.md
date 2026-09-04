# PR-626 全部展开与全部收缩补充验收

- DEV：`DEV-626-BULK-EXPAND-COLLAPSE`。
- 范围：商品档案、物料档案、生产 BOM、具体仓库库存的共享 Vue 分组工作区；沿用既有分组 helper，不新增 API/数据库/业务写入。
- 发布授权：2026-09-04 Van 明确要求依次合并 develop、部署 development、合并 main、部署 production。微信上传/审核/发布与人工业务验收仍由 Van 完成。

## RED / GREEN

- RED：真实 Vue SFC 组件测试 5 项因缺少“全部展开/全部收缩”按钮失败；support 交付合同因 DEV、手册和组件入口缺失失败。
- GREEN：新增 5 项组件运行时测试通过；合并既有分组 helper/四页合同回归共 29/29 通过。覆盖所有层级、未分类、无模板平铺组、空列表、加载/移动禁用、直接调用防护、搜索快照与移动恢复；业务行、分类页码及勾选不被改写。
- 完整门禁：`scripts/verify_kferp.sh all` 通过，Go 全包通过、前端 1065/1065、Vite 构建通过；仅保留既有大 chunk 提示。
- API：`go test ./internal/interfaces/http/support -run TestDev626GroupListInteractionDeliveryContracts -count=1` 通过，既有 assignment 合同不变。
- 本地浏览器样例：实际共享组件展开/收缩和移动禁用验证通过，390px 与桌面视口按钮成组换行正常；样例不连接业务数据，不能替代 Van 的真实 ERP 验收。
- 手册：`OP_MANUAL_INVENTORY_MATERIALS.md`、`OP_MANUAL_PRODUCTION.md`；既有 Vue 手册页面读取同一 Markdown 源。

## 发布检查点

- [x] development：功能分支已推送并快进合入 `develop@b34a3fd20b2a4dfda063effacc0fd47083ab1831`；远程预检与 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development` 成功，前端 1065/1065、小程序 220/220、Go 全包、Vue/小程序及 Docker 构建通过。
  - `erp_orderapp` running，重启计数 0；PostgreSQL healthy。外部登录页 200，四页 Vue 入口均 200，未登录受保护需求 API 为 401；根 `/app/` 保留原有 303 跳转到 orders。
  - 需求 API 可见 `DEV-626-BULK-EXPAND-COLLAPSE`；构建包包含“全部收缩”。组件 SHA-256 `c0277673a58b94bf589b65bb48c387f8854bf73cb3748f32e6fa26d26654b782`，helper SHA-256 `accfc0c63db655a906d30c4cbd49ea7880ee29f7bdebf9f4353b816e95005fef`，均与发布源码一致；`RELEASE_INFO` 环境/提交匹配。
  - 回滚源码 `/opt/stacks/erp/orderapp.backup.deploy-20260904103201-b34a3fd20b2a`；回滚镜像 `kferp-orderapp-rollback:development-20260904103201-b34a3fd20b2a`。
- [x] production：发布分支 `codex/pr626-bulk-production-release-20260904` 从 `origin/main@b7326d94` 合并已验证的 `origin/develop@b34a3fd2`，无冲突且保留原生产记录；生产隔离预检通过后，将相同提交 `6a631048320f78db20526bbeba724e0eb0d0bfd7` 推送 main，并从干净、与远端一致的 main 执行 `./deploy_orderapp.sh production` 成功。
  - 预检和部署均通过前端 1065/1065、小程序 220/220、Go 全包、Vue/小程序及 Docker 构建；合并后的 `TestDev626GroupListInteractionDeliveryContracts` 与 `scripts/verify_kferp.sh changed` 通过。
  - `erp_prod_orderapp` 与 `erp_orderapp` 均 running/restarts=0，两环境 PostgreSQL healthy；生产最近 5 分钟日志无 panic/fatal/error 命中。外部登录页及四页 Vue 入口 200，未登录根页面和受保护 API 均 401；使用生产容器当前 APP_USER/APP_PASS 的只读需求 API 返回 200，包含新 DEV，认证根页面保留 303 跳转。未更改凭据或权限。
  - 组件/helper SHA-256 与上述开发发布值及生产提交一致，生产构建包包含“全部收缩”，`RELEASE_INFO` 匹配 production 与 `6a631048320f78db20526bbeba724e0eb0d0bfd7`。
  - 回滚源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20260904110004-6a631048320f`；回滚镜像 `kferp-orderapp-rollback:production-20260904110004-6a631048320f`。
  - 本地小程序包 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，14 个声明页面与清单 56 个文件验证通过，提交/生产环境/API 地址匹配；旧包保留 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin.backup-20260904110630-6a631048320f`。未执行微信上传/审核/发布。
- [ ] Van 四页面业务验收。

## 交付说明

- 功能分支 `codex/group-list-expand-collapse-20260904` 的实现提交 `b34a3fd2` 已合并至两环境；上线后的证据单独作为该分支的文档提交保留，不再次部署或改动两个已部署的 develop/main 提交。
- 生产发布后磁盘约剩 1.7GB（使用率 98%）；没有清理历史备份、数据卷或业务文件，后续需要独立安排容量处理。
- 日志：`/private/tmp/pr626-bulk-dev-preflight.log`、`/private/tmp/pr626-bulk-dev-deploy.log`、`/private/tmp/pr626-bulk-prod-preflight.log`、`/private/tmp/pr626-bulk-prod-deploy.log`。
