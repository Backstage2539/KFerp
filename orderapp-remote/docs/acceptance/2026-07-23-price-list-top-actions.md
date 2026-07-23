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
- development 部署和浏览器冒烟待完成后补充。

## 验收口径

- 顶部三个入口按管理阶梯模板、计价模式规则、价格表配置顺序排列，均能打开原有界面。
- 商品数与价格表归属块视觉高度一致；宽屏与按钮组同行，中等宽度按钮组在顶部区域换行，窄屏不溢出。
- 页面顶部没有刷新按钮；切换价格表归属和商品类型后仍会自动加载。
- 生成价格表说明区不再重复放置计价模式规则或价格表配置入口。

## 交付边界

- development 部署待完成；production 不部署。
- 不发布、撤回、归档或重新生成任何价格表。
