# PR-458 分组模板驱动业务列表整理

## Scope
- PR-458-GROUP-TEMPLATE-BUSINESS-LISTING：商品档案、生产 BOM、仓库库存统一选择 `分组模板` 后，业务列表按模板完整大类/小类树和 `未分类` 自动整理展示。
- 分组模板仍只在 `系统设置 / 分组模板` 维护模板名、大类、小类；模板内不维护商品、BOM、仓库对象。
- 商品、BOM、仓库移动分类继续写入 `business_group_assignments`，不新增表、不新增后端 API。

## Acceptance Checklist
- [ ] 商品档案选择 `商品分组` 后，列表出现模板下全部大类/小类标题；`咖啡熟豆`、`挂耳咖啡` 等空大类也显示，未归类商品进入 `未分类`。
- [ ] 商品档案没有分类过滤 Tab，商品表格没有独立 `分类` 列；分类归属只通过分组标题表达。
- [ ] 商品档案可勾选商品，通过 `移动到分类` 移动到 `未分类`、大类或小类，保存覆盖旧归类并写操作日志。
- [ ] 生产 BOM 选择分组模板后，BOM 列表按模板完整大类/小类和 `未分类` 展示，空分类也显示。
- [ ] 生产 BOM 没有 `全部分类 / 未分类 / 分类项` 过滤 Tab；状态、搜索、批量失效和批量移动继续可用。
- [ ] 仓库库存不再显示 `普通仓库`、`客户仓库` 固定分段；仓库按库存分组模板的大类/小类和 `未分类` 整理。
- [ ] 仓库行可勾选，使用同一套 `移动到分类` 控件批量移动仓库；仓库类型和客户绑定只作为行内或抽屉信息保留。
- [ ] 商品档案、生产 BOM、仓库库存三处页面共用 `BusinessGroupControls` 和 `business-grouping` helper。

## Verification
- RED frontend：`node --test orderapp-remote/frontend-vue-shell/src/lib/business-grouping.test.js orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/bom.test.js orderapp-remote/frontend-vue-shell/src/lib/materials-ui.test.js` 在实现前失败，因为共享 helper/control、模板空分类显示、商品/BOM 去 Tab 和仓库去固定分段 marker 缺失。
- GREEN frontend：`node --test orderapp-remote/frontend-vue-shell/src/lib/business-grouping.test.js orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/bom.test.js orderapp-remote/frontend-vue-shell/src/lib/materials-ui.test.js` 通过 151/151。
- 待补充：support contract、Vue build、浏览器验收和 development deploy 证据。
