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

- development 待验证后部署；production 不部署。
- 不发布、撤回、归档或重新生成任何价格表。
