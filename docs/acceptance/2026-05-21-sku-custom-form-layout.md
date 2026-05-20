# 2026-05-21 SKU设置客户专属 SKU 创建区布局

## 范围
- PR-298-SKU-CUSTOM-FORM-LAYOUT：SKU设置选择履约客户后，“客户专属 SKU”创建区不再卡在左侧窄列，改为横向全宽工作区。
- DEV-298-SKU-CUSTOM-FORM-LAYOUT：Vue/Vite 的 `ProductSettingsView.vue` 调整面板 grid 和表单列数，桌面 4 列、中等屏幕 2 列、小屏幕 1 列。

## 验收点
- “客户专属 SKU”面板跨越 SKU 设置左右两列，不再让右侧大面积空白。
- 基础产品、绑定熟豆、专属 SKU 名称、备注、复制选项和创建按钮都在面板内排布，不溢出边界。
- 公共 SKU 场景的“新增公共产品”布局不受影响。

## 证据
- TDD RED：`node --test src/lib/product-settings.test.js` 曾失败于 `SKU settings renders the customer-only SKU form as a full-width workspace`，证明旧 CSS 未满足全宽布局要求。
- TDD GREEN：`node --test src/lib/product-settings.test.js` 通过 21/21。
- 前端单元测试：`node --test src/lib/*.test.js src/api/*.test.js` 通过 241/241；合入最新 `origin/develop` 后复跑仍通过。
- API 级测试：`go test ./internal/interfaces/http/catalog -run ProductSettings -count=1 -v` 通过产品设置 API 全部用例。
- 全量 Go 测试：`go test ./...` 通过；合入最新 `origin/develop` 后复跑仍通过。
- 构建：`npm run build` 通过；合入最新 `origin/develop` 后复跑仍通过，仅保留 Vite 既有 chunk size warning。
- 空白检查：`git diff --check` 通过。
- 浏览器验证：本地 mock Vue shell 打开 `/vue-shell/?view=productSettings`，选择“岩师傅”后检测 `.custom-product-panel` 与 `.settings-grid` 等宽，桌面宽度下 `formColumns=227px 227px 227px 227px`、`overflowing=[]`；1000px viewport 下 `formColumns=324px 324px`、`overflowing=0`；800px viewport 下 `overflowing=0`。
