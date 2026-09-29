# PR-674 商品价格表命名表 Tab 切换

## 目标

将“当前编辑价格表”下拉框改为单行可横向滚动的键盘可达 Tab；“价格表配置”固定在 Tab 行右侧，表数量、统一版本和复制入口位于下方。切表沿用原有草稿持久化与恢复逻辑。

## TDD 证据

- RED 前端：`node --test src/lib/price-table-batch.test.js` 在原下拉框实现下失败，缺少 Tab 行与可访问交互合同。
- RED 需求/API 跟踪：`go test ./internal/interfaces/http/support -run TestDev674PriceTableTabsRequirementAndManualContracts -count=1` 因 PR/DEV 种子和手册证据缺失而失败。
- GREEN 前端：`node --test src/lib/price-table-batch.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/product-price-list-types.test.js src/lib/product-price-list-selection.test.js` 通过 93/93。
- GREEN API 回归：`go test ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1` 通过。此改动没有变更业务请求或响应格式。
- GREEN build/check：`./scripts/verify_kferp.sh frontend-build` 通过（Vite 存在既有大 chunk 提示）；`./scripts/verify_kferp.sh changed` 与 `git diff --check` 通过。

## 页面验证

- 桌面、多表、长表名、键盘左右方向键/Home/End、配置抽屉入口与配置禁用条件：待浏览器检查。
- 窄屏 Tab 行横向滚动、配置按钮固定可见、切表保存和恢复商品选择/规格/价格/样式：待浏览器检查。
- 文档/需求支持测试：`go test ./internal/interfaces/http/support -run TestDev674PriceTableTabsRequirementAndManualContracts -count=1` 通过；成本核价手册的前端手册入口仍引用单一 `OP_MANUAL_COSTING.md`。
- 浏览器截图：待补充。

## 交付

- PR：PR-674-PRICE-TABLE-EDITOR-TABS；DEV：DEV-674-NAMED-TABLE-TABS / DEV-674-DOCS-DELIVERY。
- 分支：`codex/price-table-tabs-20260929`，基于 `origin/develop@955b642667eca703f2cd71cc0acf3285882b745a`。
- Development 合并、部署和 smoke：待执行；production 不在范围内。
