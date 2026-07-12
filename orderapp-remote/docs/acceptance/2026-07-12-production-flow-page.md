# PR-533 生产流程页面归并验收

## 验收范围

- 生产流程用五个 Tab 集中生产计划、生产工单、工序卡、生产质检和生产验收。
- 生产侧栏与顶部切换条移除上述独立入口。
- 生产手册位于生产管理菜单最后。
- 旧直达路由、数据、API 和业务规则保持兼容。

## 自动验证

- 前端合同：`node --test src/lib/production-flow-page.test.js src/lib/menu-ia.test.js src/lib/production-workstation.test.js`
- 权限与交付合同：`go test ./internal/infrastructure/postgres/authz ./internal/interfaces/http/support -run 'TestDefaultViewPermissions|TestDev533' -count=1`
- 前端构建：`npm run build`

## 人工验收

1. 展开生产管理，确认显示生产流程，未显示生产计划、生产工单、工序卡、生产质检、生产验收独立入口，且生产手册位于最底部。
2. 打开生产流程，依次切换五个 Tab，确认各页面读取既有数据且页面内不重复显示生产模块顶部导航。
3. 打开生产视图或生产中，确认顶部切换条显示生产流程，不再分别显示计划、工单、工序卡、质检。
4. 分别打开旧 `view=producePlan`、`view=workOrders`、`view=jobCards`、`view=qualityInspections`、`view=productionAcceptance`，确认旧页面仍可访问。
