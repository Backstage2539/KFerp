# PR-532 生产与系统菜单归并验收

## 验收范围

- 系统设置从设置栏移动到系统栏。
- 生产配置用三个 Tab 集中工艺路线、工序、工位/设备。
- 生产成本移除常规菜单和生产顶部切换入口，但保留数据、API、工单追溯和旧路由。
- 生产计划使用精简名称，既有计划流程不变。

## 自动验证

- 前端合同：`node --test src/lib/production-system-menu-consolidation.test.js src/lib/menu-ia.test.js src/lib/production-workstation.test.js`
- 权限与交付合同：`go test ./internal/infrastructure/postgres/authz ./internal/interfaces/http/support -run 'TestDefaultViewPermissions|TestDev532' -count=1`
- 前端构建：`npm run build`

## 人工验收

1. 展开设置栏和系统栏，确认系统设置只出现在系统栏，打开后基础设置和通知设置正常。
2. 展开生产管理，确认显示生产配置和生产计划，不显示三个制造主档独立入口、生产计划旧名称或生产成本。
3. 打开生产配置，依次切换工艺路线、工序、工位/设备三个 Tab，确认页面和既有数据正常。
4. 打开生产页面顶部切换条，确认不显示成本入口。
5. 分别打开旧 `view=processTemplates`、`view=manufacturingOperations`、`view=manufacturingWorkstations`、`view=productionCosts`，确认兼容页面仍可访问。
