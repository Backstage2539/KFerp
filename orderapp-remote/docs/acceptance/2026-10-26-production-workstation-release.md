# 生产工位修复发布记录

日期：2026-10-26
环境：生产 `https://erp.qacoohee.com`
发布来源：`main` / `d064197308c5c98561df2a6ead04d2bc8639784a`

## 发布结果

- `./deploy_orderapp.sh production` 正常完成，脚本返回成功；仅重建并重启 `erp_prod_orderapp`，没有重启数据库或写入业务记录。
- 生产镜像：`sha256:6431817fb3d6468dfd4732836fbf86e11c260143449c4ac054df13f1d6b9ce69`。应用容器运行中，发布后 restart count 为 0；启动日志显示 HTTP 服务正常监听。
- PostgreSQL 容器保持运行且健康，未在本次发布中重启。
- 生产根盘发布后可用 12 GB（59 GB 总量，80% 使用率）；无需清理缓存。
- 回滚源码快照：`/opt/stacks/erp-production/orderapp.backup.deploy-20261026181159-d064197308c5`。
- 回滚镜像：`kferp-orderapp-rollback:production-20261026181159-d064197308c5`，镜像 ID `sha256:12c99e2cdaa86699b81acb81cfcda146c4a81696ab1e231fd9f174c69990eb48`。

## 验证证据

- 发布候选完整验证通过：所有 Go 包、Vue 1,613/1,613、Vite 生产构建；服务器再次通过 Vue 1,613、微信小程序 432、类型检查、小程序构建、全量 Go 测试及 Docker 镜像构建。
- `GET https://erp.qacoohee.com/app/login` 返回 HTTP 200。
- 通过运行容器提供的认证信息只读访问 `/app/vue-shell?fresh_login=1` 返回 HTTP 200；工作站 bundle `/vue-shell/assets/WorkstationView-6R7WpIPm.js` 返回 HTTP 200，49,269 字节，包含“合并领料”“补齐物料”“工序完成”等本次界面文案。未记录或保存认证信息。
- 匿名访问受保护的 `/app/` 返回 HTTP 401，符合现有 Basic Auth 保护行为。
- 容器最近启动日志为 `orderapp listening on :8080`，没有观察到启动错误。
- 小程序产物已同步到 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，并校验 96 个清单文件；本次没有上传或发布微信小程序版本。

## 业务边界

生产环境只验证了部署状态和页面资源，没有点击真实工单完成、入库、领料、库存修复或其他业务写入操作。生产工单的真实完成数量、耗料、成品入库及经 Van 的业务验收仍由现场完成。开发环境点击检查记录见 `2026-10-26-development-workstation-clickthrough.md`。
