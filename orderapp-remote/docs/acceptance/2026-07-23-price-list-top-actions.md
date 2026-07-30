# PR-549 商品价格表顶部管理入口整合

## 范围

- `管理阶梯模板`、`计价模式规则`、`价格表配置` 三个按钮统一放到商品价格表顶部。
- 商品数与价格表归属卡片等高，宽屏同行、中等宽度按钮换行、窄屏安全单列。
- 删除商品价格表顶部 `刷新`，保留现有自动加载链路。
- 不修改价格表 API、数据库、发布快照、权限或操作日志。

## TDD 证据

### RED

- `node --test src/lib/costing-bean-list-version-ui.test.js`：新增顶部入口测试失败，确认页面仍显示顶部 `刷新`，且计价模式规则和价格表配置仍在生成区。
- `go test ./internal/interfaces/http/support -run '^TestDev549PriceListTopActionsContracts$' -count=1`：因缺少 PR-549 需求种子和交付文档失败。

### GREEN

- `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/price-list-page-preferences.test.js src/lib/product-bean-list-split.test.js`：65/65 通过。
- `go test ./internal/interfaces/http/support -run '^TestDev54(7|8|9)' -count=1`：通过。
- `scripts/verify_kferp.sh backend`：完整 Go 测试通过。
- `npm run build`：Vue/Vite production build 通过，共转换 402 个模块；仅保留既有 chunk-size 提示。
- `scripts/verify_kferp.sh changed` 与 `git diff --check`：通过。
- 独立代码审查发现 901–1200px 内容区可能裁切按钮，补充 1200px 中间断点后复核通过，最终无 P0-P3 阻断问题。
- `npm_config_audit=false ./deploy_orderapp.sh development`：从干净 `origin/develop=31b7d19092aeda01d715f56b7eb297bc6763effd` 部署成功；Docker build 内完整 `go test ./...` 通过，应用备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260723145039`。
- 服务器冒烟：`erp_orderapp` 正常运行且重启数为 0，`erp_postgres` healthy，应用日志为正常监听 `:8080`；开发 Vue shell、新 JS/CSS 与需求 API 均返回 200，需求响应包含 `PR-549-PRICE-LIST-TOP-ACTIONS`。
- 浏览器冒烟：开发环境商品价格表在 1470×835 视口下，商品数与价格表归属块实测高度均为 82px；顶部按钮依次为 `管理阶梯模板`、`计价模式规则`、`价格表配置`，顶部无 `刷新`，页面没有横向溢出。
- 浏览器分别打开并关闭阶梯模板抽屉、计价模式规则弹窗和价格表配置抽屉，原入口功能均正常；没有保存、发布、撤回、归档或修改任何业务数据。

## 验收口径

- 顶部三个入口按管理阶梯模板、计价模式规则、价格表配置顺序排列，均能打开原有界面。
- 商品数与价格表归属块视觉高度一致；宽屏与按钮组同行，中等宽度按钮组在顶部区域换行，窄屏不溢出。
- 页面顶部没有刷新按钮；切换价格表归属和商品类型后仍会自动加载。
- 生成价格表说明区不再重复放置计价模式规则或价格表配置入口。

## 交付边界

- development 已部署并完成服务器/API/浏览器只读冒烟；production 不部署。
- 不发布、撤回、归档或重新生成任何价格表。
