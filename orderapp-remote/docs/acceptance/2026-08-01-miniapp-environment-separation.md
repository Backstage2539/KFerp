# PR-568 小程序开发/生产环境隔离与防误发验收

日期：2026-08-01

## 需求与开发项

- `PR-568-MINIAPP-ENVIRONMENT-SEPARATION`
- `DEV-568-CLIENT-ENVIRONMENT-GUARD`
- `DEV-568-STORAGE-BOUNDARY`
- `DEV-568-FIXED-ARTIFACTS`
- `DEV-568-DOCS-ACCEPTANCE`
- `UT-568-MINIAPP-ENVIRONMENT-SEPARATION`
- `API-568-MINIAPP-ENVIRONMENT-SEPARATION`
- `REV-568-MINIAPP-ENVIRONMENT-SEPARATION`

## 已确认前置条件

- development 与 production 服务器 `.env` 均已配置微信 AppID/AppSecret；只读检查确认 AppID 相同，未读取或输出 Secret。
- 服务器现有 development `RELEASE_INFO` 指向 `https://dev.qacoohee.com/app`，production 指向 `https://erp.qacoohee.com/app`。
- 用户已将两个 HTTPS 域名加入微信小程序服务器合法域名；验收仍需确认 `request` 和 `downloadFile` 两类均配置。

## TDD 证据

- RED：`npm --prefix miniapp test -- src/config/environment.test.ts src/stores/session.test.ts src/utils/beanListPageCache.test.ts src/utils/fileOutput.test.ts`，环境模块/令牌 helper 缺失、缓存未分区、下载提示固定生产域名，按预期失败。
- RED：`go test ./internal/interfaces/http/support -run TestDev568MiniappEnvironmentSeparationContract -count=1`，发布/手册合同缺失，按预期失败。
- GREEN targeted：同一 Vitest 命令通过，4 个文件共 12 项；环境门禁缺失/错配返回非零，development 与 production 正确配对返回零。
- GREEN miniapp：完整 `npm test -- --maxWorkers=1 --minWorkers=1 --no-file-parallelism` 通过，19 个文件共 97 项；`npm run typecheck` 通过。
- GREEN release contract：`go test ./internal/interfaces/http/support -run TestDev568MiniappEnvironmentSeparationContract -count=1` 与完整 support 包测试通过；`bash -n deploy_orderapp.sh`、`bash -n scripts/remote_orderapp_release.sh`、`git diff --check` 通过。
- GREEN remote development preflight：功能提交 `1a3d4c4f329963c3e28c44f03a92af23a7457181` 通过 Vue 852 项测试/构建、小程序 97 项测试/类型检查/development 包构建、完整 Go 测试与隔离 Docker 构建；产物只含开发 API 地址。未提升源码、重启容器或同步固定目录。
- GREEN remote production preflight：同一功能提交通过 production 包构建及相同完整门禁；产物只含生产 API 地址。未部署 production、未重启生产容器、未覆盖正式小程序目录。
- development 集成部署与开发固定目录只读验收：待合入 `develop` 后执行。

## 部署与环境边界

- 目标：合入 `develop` 并部署 development。
- development 固定小程序目录：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`。
- production 固定小程序目录：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，本需求不部署、不覆盖。
- 微信开发者工具的预览、上传、体验版及正式发布均需人工确认，脚本不代替微信平台操作。
