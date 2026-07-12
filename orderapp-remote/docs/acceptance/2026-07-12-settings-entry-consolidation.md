# PR-531 设置入口归并验收

## 验收范围

- `设置 / 公司设置` 同页维护公司资料与共享公章资产。
- `商品 / 商品价格管理` 同页维护成本参数与价格计算模板。
- 主菜单移除独立公章、成本参数和代加工模板入口。
- 旧直达路由和既有数据/API 保持兼容。

## 自动验证

- 前端合同：`node --test src/lib/settings-entry-consolidation.test.js src/lib/menu-ia.test.js src/lib/product-settings.test.js`
- 后端审计与交付合同：`go test ./internal/interfaces/http/support -run 'TestDev531|TestAuditReadable' -count=1`
- 前端构建：`npm run build`

## 人工验收

1. 打开 `view=companyProfile`，确认页面显示公司资料和公章设置，公章的上传、选择、去背景操作可用。
2. 打开 `view=productPriceManagement`，确认 `价格计算模板` 和 `成本参数设置` 两个 Tab 并排显示；切换到成本参数设置后保存，刷新数据仍在。
3. 展开主菜单，确认不显示独立 `公章设置`、`成本参数设置`、`代加工模板设置`。
4. 分别打开旧 `view=costingSettings`、`view=outsourceSettings`，确认旧链接仍可访问且既有数据不变。
5. 修改成本参数或选择公章后，到操作日志确认菜单归属分别显示 `商品 / 商品价格管理 / 成本参数设置` 和 `设置 / 公司设置 / 公章设置`。
