# PR-548 商品价格表顶部紧凑布局与选择记忆

## 范围

- 商品数、价格表归属和管理阶梯模板在桌面端同一行展示。
- 浏览器记住价格表归属和商品类型，下次进入自动恢复。
- 不修改价格表 API、数据库、发布快照或操作日志。

## TDD 证据

### RED

- 前端合同测试因缺少 `price-list-top-toolbar`、浏览器偏好 helper 和页面接入失败。
- `go test ./internal/interfaces/http/support -run '^TestDev548PriceListScopeMemoryContracts$' -count=1` 因缺少 PR-548 需求种子失败。

### GREEN

- `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/price-list-page-preferences.test.js src/lib/product-bean-list-split.test.js`：64/64 通过。
- `go test ./internal/interfaces/http/support -count=1`：通过。
- `scripts/verify_kferp.sh backend`：完整 Go 测试通过。
- Vue/Vite production build：通过，共转换 402 个模块；仅保留既有 chunk-size 提示。
- `scripts/verify_kferp.sh changed` 与 `git diff --check`：通过。
- 本地浏览器被系统登录页拦截，未绕过登录；实际布局与刷新恢复留待开发环境登录态验收。

## 验收口径

- 桌面端顶部三项在同一行，窄屏不溢出。
- 切换价格表归属和商品类型后刷新页面，选择保持不变。
- 失效选择安全回退；客户工作区锁定归属不覆盖普通页面偏好。

## 交付边界

- 功能提交 `d63ada84`，合并后的 `develop` 为 `ee22949f3d3e80f8a91dc90695026a52f7b2ba82`，已通过 `npm_config_audit=false ./deploy_orderapp.sh development` 部署；备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260723115210`。
- 开发栈 `erp_orderapp` 正常运行、`erp_postgres` healthy；公开入口未认证返回 401，BasicAuth shell 返回 200；开发数据库存在 PR-548，服务器源码、偏好 helper 与 dist 均包含新功能标记。
- 第一次部署仅在本地小程序 `npm ci` 在线审计阶段卡住，服务器同步尚未开始；关闭重复在线审计后重跑，Vue、小程序类型检查/构建和 Docker 内完整 Go 测试全部通过。
- production 不部署。
- 不发布、撤回、归档或重新生成任何价格表。
