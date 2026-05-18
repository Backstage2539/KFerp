# 验收记录：快速成本参数删除出成率和利润系数

## 需求
- 快速成本参数设置中不再展示或维护“生豆到熟豆转化率”。
- 快速成本参数设置中不再展示或维护商用熟豆、零售熟豆、挂耳的利润系数。

## 实现
- `/api/costing/settings` 过滤废弃快速参数，保留基础换算、生产包装、税费、物流、挂耳成本等仍可编辑参数。
- 旧参数直连保存请求返回 400，避免用户绕过页面继续修改。
- 前端成本参数分组函数同步过滤这些字段，防止旧接口数据或缓存被渲染。
- 操作手册明确出成率、商用熟豆利润率、挂耳倍率的维护入口。

## 验收
- [x] 快速成本参数分组不包含 `roast_yield_rate`。
- [x] 快速成本参数分组不包含 `retail_bean_margin_rate`、`wholesale_kg_margin_rate_*`。
- [x] 快速成本参数分组不包含 `retail_drip_multiplier`、`wholesale_drip_multiplier_*`。
- [x] `/api/costing/settings` 不返回上述废弃参数。
- [x] `POST /api/costing/settings/roast_yield_rate` 返回 400。
- [x] 操作手册已更新：`OP_MANUAL_COSTING.md`。

## 证据
- `node --test src/lib/costing-settings.test.js`
- `go test ./internal/application/costing ./internal/interfaces/http/costing -run 'TestSettingsHidesDeprecatedYieldAndMarginParameters|TestCostingSettingsAPI|TestCostingSettingsAPIFiltersDeprecatedQuickSettings' -count=1`
