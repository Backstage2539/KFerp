# PR-531 设置入口归并验收

## 验收范围

- `设置 / 公司设置` 同页维护公司资料与共享公章资产。
- `商品 / 商品价格管理` 只维护价格计算模板；成本参数设置已由 PR-535 移除。
- 主菜单移除独立公章、成本参数和代加工模板入口。
- 旧 `outsourceSettings` 路由和既有数据/API 保持兼容；旧 `costingSettings` 页面不再提供。

## 自动验证

- 前端合同：`node --test src/lib/settings-entry-consolidation.test.js src/lib/menu-ia.test.js src/lib/product-settings.test.js`
- 后端审计与交付合同：`go test ./internal/interfaces/http/support -run 'TestDev531|TestAuditReadable' -count=1`
- 前端构建：`npm run build`

## 人工验收

1. 打开 `view=companyProfile`，确认页面显示公司资料和公章设置，公章的上传、选择、去背景操作可用。
2. 打开 `view=productPriceManagement`，确认只显示价格计算模板，不显示成本参数设置或对应 Tab。
3. 展开主菜单，确认不显示独立 `公章设置`、`成本参数设置`、`代加工模板设置`。
4. 打开旧 `view=costingSettings`，确认不再提供设置页面；`view=outsourceSettings` 仍可访问且既有数据不变。
5. 选择公章后，到操作日志确认菜单归属显示 `设置 / 公司设置 / 公章设置`。
