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

- 桌面 1470px：两张表同一行显示，当前表突出且带“默认”标记；配置按钮固定在 Tab 行右侧，表数量、统一版本和复制入口位于下方。配置入口只有一个，点击打开原配置抽屉后关闭，未修改字段。
- 窄屏 390px：长表名截断，Tab 区域可横向滚动，配置按钮仍完整可见；页面内容宽度未超出视口。
- 单表与多表：开发环境实测一张表和两张表状态；双表间用方向键来回切换后，选品、规格、价格和展示样式分别恢复。标准表恢复 2 个商品/3 个规格/12 行价格；一件代发表恢复 2 个商品/2 个规格/4 行价格。未编辑或发布价格。
- 快捷键：浏览器确认 ArrowLeft/ArrowRight 可切换；Home/End 有键盘处理实现，但此次浏览器实测未能确认其选中行为。发布中禁用条件由现有逻辑/自动化覆盖，未在浏览器启动发布流程。
- 新增、复制、改名和删除后的列表同步沿用同一响应式表集合；此次未在开发环境写入真实价格表变更，留给 Van 验收。
- 文档/需求支持测试：`go test ./internal/interfaces/http/support -run TestDev674PriceTableTabsRequirementAndManualContracts -count=1` 通过。成本核价手册的 Vue 手册入口仍使用唯一的 `OP_MANUAL_COSTING.md`。

## 交付

- PR：PR-674-PRICE-TABLE-EDITOR-TABS；DEV：DEV-674-NAMED-TABLE-TABS / DEV-674-DOCS-DELIVERY。
- 分支：`codex/price-table-tabs-20260929`，基于 `origin/develop@7d12711d`；最终开发提交 `c53fe2ee7b29498d74246c8f20b615de13cea17b` 已合并至 `develop`。
- 最终本地验证：96 个聚焦 Node 测试通过；成本核价/销售/支持 API 聚焦 Go 回归通过；`npm run build`、`./scripts/verify_kferp.sh changed`、`git diff --check` 通过。
- Development：`./deploy_orderapp.sh development` 全门禁通过并部署上述提交；远端 Node/Vue、Mini Program 264 项/42 文件、TypeScript、mp-weixin build、全量 Go 测试与二进制构建通过；`https://dev.qacoohee.com/app/login` 返回 HTTP 200。部署前备份 `/opt/stacks/erp/orderapp.backup.deploy-20260929233908-c53fe2ee7b29`，回滚镜像 `kferp-orderapp-rollback:development-20260929233908-c53fe2ee7b29`。
- Development 小程序产物已同步到 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，未上传或提交微信审核/发布。Production 未操作。Van 业务验收待进行。
