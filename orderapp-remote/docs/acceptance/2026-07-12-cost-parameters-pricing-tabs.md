# PR-535 商品价格管理成本参数 Tab 验收

## 验收目标

- 商品价格管理用并排 Tab 分开价格计算模板和成本参数设置。
- 默认打开价格计算模板；模板动作不出现在成本参数 Tab。
- 成本参数继续使用既有读取、保存、操作日志和旧直达路由，不改变数据。

## 自动验证

- 前端 RED/GREEN：`node --test src/lib/price-management-tabs.test.js src/lib/settings-entry-consolidation.test.js src/lib/costing-settings.test.js src/lib/product-settings.test.js`
- API：`go test ./internal/interfaces/http/costing -run 'TestCostingSettingsAPI|TestCostingSettingsAPIFiltersDeprecatedQuickSettings' -count=1`
- 支持合同：`go test ./internal/interfaces/http/support -run 'TestDev535|TestDev531' -count=1`
- 构建与变更验证：`scripts/verify_kferp.sh changed`、`scripts/verify_kferp.sh frontend-build`。

## 浏览器验收

1. 打开开发环境 `商品 / 商品价格管理`，确认默认选中 `价格计算模板`，旁边显示 `成本参数设置`。
2. 确认价格试算、新建价格计算模板、模板列表和编辑表单只在价格计算模板 Tab 显示。
3. 切换到成本参数设置，确认参数分类、刷新和保存正常，且不显示价格模板动作。
4. 切回价格计算模板并刷新页面，确认两个 Tab 与既有数据正常；旧 `view=costingSettings` 仍可访问。

## 实现前后证据

- RED：新 Tab 契约与既有成本参数测试共 10 条，现状 8 条通过、2 条失败；缺少并排 Tab 列表和按 Tab 隔离的内容面板。
- GREEN：目标前端组合测试 163/163 通过。
- API：成本参数 GET/POST 与废弃参数过滤的 targeted Go 测试通过。
- 支持合同：PR-535、历史 PR-531、操作日志归属和手册标记测试通过。
- 构建：Vue/Vite production build、`scripts/verify_kferp.sh changed`、`git diff --check` 通过。
