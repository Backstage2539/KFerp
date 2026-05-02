# 2026-05-02 今日任务验收清单

记录时间：2026-05-02 12:13:53 +0800  
整理分支：`codex/today-acceptance-checklist-20260502`  
远端基线：`origin/develop` = `f64bb32dfe4b90d4ae9e01d6fd2dfc2a76c4d6ae`  
用途：给 Van 做今日分支验收、后续开发查历史、清理分支前复核。

## 使用规则

- 只有 `UT`、`API`、`REV` 三列证据都补齐后，需求才可以标记为完成。
- `Git 状态` 为“已进 develop”的任务，可以按本文件直接验收现网/待部署版本。
- `Git 状态` 为“待集成”或“WIP 未提交”的任务，不能按 develop 验收；必须先提交、推送、同步最新 `origin/develop`、重跑测试，再进入验收。
- 本清单是历史记录，不替代系统 UI 中 5 张表；UI 表应按相同 ID 回填。
- 下方测试命令默认在 `orderapp-remote/` 目录执行。

## 今日分支总览

| 状态 | 分支 / 工作区 | 证据 | 处理建议 |
|---|---|---|---|
| 已进 develop | `codex/sales-order-image-export-20260502` | `2c84b6f` 同时是 `origin/develop` HEAD | 进入销售单图片导出验收 |
| 已进 develop | `codex/delivery-note-outbound-20260502` 的已提交部分 | 分支 HEAD `833c8c9` 已在 `origin/develop` | 注意该工作区另有未提交出库单功能，见 WIP 行 |
| 已进 develop | `codex/production-quality-drawer-20260502` | `3851413` 已在 `origin/develop` | 进入生产质检抽屉基础验收 |
| 已进 develop | `codex/production-merge-order-links-20260502` | `49907b8` / `e3fd473` 已在 `origin/develop` | 进入合并生产单产出验收 |
| 已进 develop | `codex/stock-operation-combobox-layout-20260502` | `a1b8d78` / `ba13425` 已在 `origin/develop` | 进入库存作业输入框与布局验收 |
| 已进 develop | `codex/production-running-receipts-yield-20260502` | `79cdb75` / `4e11e29` 已在 `origin/develop` | 进入生产中投料/出品率验收 |
| 已进 develop | `codex/company-account-sales-order-layout-20260502` | `f0c644f` 已在 `origin/develop` | 进入公司公账与销售单版式验收 |
| 已进 develop | `codex/sales-order-pdf-layout-account-20260502` | `6a7064e` 已在 `origin/develop` | 进入销售单 PDF 版式验收 |
| 已进 develop | `codex/sales-order-drawer-settings-20260502` | `ddec1d0` 已在 `origin/develop` | 进入销售单抽屉设置验收 |
| 已进 develop | `codex/quality-target-specific-drawer-20260502` | `f64bb32` / `fc7ff80` / `b087781` 已在 `origin/develop` | 进入生产质检类型精修验收 |
| WIP 未提交 | `/Users/yiiiple-work/.config/superpowers/worktrees/KFerp/delivery-note-outbound-20260502` | 16 个已修改文件 + 11 个未跟踪文件 | 先完成/提交/推送出库单功能，不可当作已入 develop |
| 可稍后清理 | `codex/preserved-deploy-local-develop-20260502` | 本地保留分支，behind 14 / ahead 1，无 upstream | 确认无部署回滚价值后再清理 |

## 产品需求表 (PR)

| PR ID | 产品需求 | Git 状态 | 来源 | 验收口径 | 当前结论 |
|---|---|---|---|---|---|
| PR-20260502-01 | 销售单在订单列表内以抽屉打开，并可在抽屉内打开销售单设置 | 已进 develop | `ddec1d0` | 不跳页、不丢筛选/分页；设置可维护说明、收款方式、收款码、公章；公章可调位置和大小 | 待 Van 验收 |
| PR-20260502-02 | 销售单 PDF 版式、公章比例、公账信息展示 | 已进 develop | `6a7064e` | PDF 与预览关键信息和版式一致；公章不拉伸；公账信息可展示 | 待 Van 验收 |
| PR-20260502-03 | 公账收款信息迁移到公司设置 | 已进 develop | `f0c644f` | 公司设置可维护户名、开户行、账号、税号、地址并复制；销售单设置不再出现公账字段 | 待 Van 验收 |
| PR-20260502-04 | 生产中可编辑投料、成品件数、余料和实际出品率 | 已进 develop | `e6a7ae2` / `79cdb75` | 未编辑显示 BOM 出品率；编辑后实时按成品总克重/投料计算；部分完工保留剩余 | 待 Van 验收 |
| PR-20260502-05 | 原料入库和库存作业下拉框内搜索、页面布局统一 | 已进 develop | `ba13425` / `a1b8d78` | 原料/WIP/成品选择框内可输入过滤；无独立搜索框；四个库存作业页布局对齐 | 待 Van 验收 |
| PR-20260502-06 | 合并生产单按商品合并，但保留多规格产出和订单关联 | 已进 develop | `1bbec51` / `49907b8` | 同产品多规格只生成一个运行项；完成时按规格分别入成品库存；关联订单都更新 | 待 Van 验收 |
| PR-20260502-07 | 生产质检对象从右侧抽屉选择 | 已进 develop | `b1184a5` / `3851413` / `b087781` / `f64bb32` | 可在抽屉切换工单/原料/产品并回填；按钮按类型显示且抽屉只展示当前类型对象 | 待 Van 验收 |
| PR-20260502-08 | 销售单可生成 PNG 图片版本 | 已进 develop | `2c84b6f` | “确认生成图片”生成 PNG；历史图片和最新版图片返回 `image/png`；PDF 与图片版本独立 | 待 Van 验收 |
| PR-20260502-09 | 出库单维护、预览、生成和下载 | WIP 未提交 | delivery-note 工作区改动 | 已发货订单才允许；先预览再确认生成；历史版本和最新版下载 PDF；未发货订单拒绝 | 不能验收 develop，先完成集成 |

## 开发需求表 (DEV)

| DEV ID | 对应 PR | 开发要求 | 代码证据 | 当前状态 |
|---|---|---|---|---|
| DEV-20260502-01 | PR-01 | 订单列表保留状态并打开销售单抽屉，抽屉内打开设置抽屉 | `OrdersView.vue`、`SalesOrderView.vue`、`SalesOrderSettingsView.vue` | 已进 develop |
| DEV-20260502-02 | PR-02 | 销售单 PDF 与预览统一布局，公章按比例渲染 | `internal/infrastructure/pdf/sales_order_pdf.go` | 已进 develop |
| DEV-20260502-03 | PR-03 | 公司设置保存并返回公账字段，销售单读取公司设置 | `CompanyProfileView.vue`、`internal/application/company/service.go`、`internal/interfaces/http/company/company_profile.go` | 已进 develop |
| DEV-20260502-04 | PR-04 | 生产中完成 payload 使用可编辑投料和多产出字段 | `ProduceRunningView.vue`、`frontend-vue-shell/src/lib/produce-running.js`、`internal/interfaces/http/production/production_flow_routes.go` | 已进 develop |
| DEV-20260502-05 | PR-05 | 抽象可搜索下拉并应用到库存作业页面 | `SearchableSelect.vue`、`searchable-select.js`、`MaterialReceiptsView.vue`、`StockOperationsView.vue` | 已进 develop |
| DEV-20260502-06 | PR-06 | 生产开始按商品合并需求，生产完成按 outputs 分规格落库 | `running_merge.go`、`running_repository.go`、`material_consumption.go` | 已进 develop |
| DEV-20260502-07 | PR-07 | 质检对象选择抽屉和类型过滤 | `QualityInspectionsView.vue`、`quality-inspections.js` | 已进 develop |
| DEV-20260502-08 | PR-08 | 销售单 PNG 生成、版本记录和下载接口 | `sales_order_png.go`、`sales_order_documents.go`、`sales_order_repository.go` | 已进 develop |
| DEV-20260502-09 | PR-09 | 出库单领域模型、PDF、仓储、接口、Vue 页面 | `delivery_note.go`、`delivery_note_pdf.go`、`DeliveryNoteView.vue`、`delivery_note_documents.go` | WIP 未提交 |

## 单元测试表 (UT)

| UT ID | 对应 DEV | 最小必跑测试 | 证据文件 | 当前记录 |
|---|---|---|---|---|
| UT-20260502-01 | DEV-01 | `go test ./internal/interfaces/http/support -run TestDev125SalesOrderDrawerSettings` | `dev_125_sales_order_drawer_settings_test.go` | 待复跑并回填日志 |
| UT-20260502-02 | DEV-02 | `go test ./internal/infrastructure/pdf -run SalesOrder` | `sales_order_pdf_test.go` | 待复跑并回填日志 |
| UT-20260502-03 | DEV-03 | `go test ./internal/application/company ./internal/infrastructure/postgres/company` | `service_test.go`、`repository` 相关测试 | 待复跑并回填日志 |
| UT-20260502-04 | DEV-04 | `node --test frontend-vue-shell/src/lib/produce-running.test.js` | `produce-running.test.js` | 待复跑并回填日志 |
| UT-20260502-05 | DEV-05 | `node --test frontend-vue-shell/src/lib/searchable-select.test.js frontend-vue-shell/src/lib/material-receipts.test.js` | `searchable-select.test.js`、`material-receipts.test.js` | 待复跑并回填日志 |
| UT-20260502-06 | DEV-06 | `go test ./internal/infrastructure/postgres/production -run Merge` | `running_merge_test.go` | 待复跑并回填日志 |
| UT-20260502-07 | DEV-07 | `node --test frontend-vue-shell/src/lib/quality-inspections.test.js` | `quality-inspections.test.js` | 待复跑并回填日志 |
| UT-20260502-08 | DEV-08 | `node --test frontend-vue-shell/src/lib/sales-order.test.js` 与 `go test ./internal/infrastructure/pdf -run Image` | `sales-order.test.js`、`sales_order_pdf_test.go` | 待复跑并回填日志 |
| UT-20260502-09 | DEV-09 | `go test ./internal/domain/sales -run DeliveryNote` 与 `node --test frontend-vue-shell/src/lib/delivery-note.test.js` | WIP 未跟踪测试文件 | 待提交后复跑 |

## API 测试表 (API)

| API ID | 对应 DEV | 最小必跑测试 | 证据文件 | 当前记录 |
|---|---|---|---|---|
| API-20260502-01 | DEV-01 | `go test ./internal/interfaces/http/support -run TestDev125SalesOrderDrawerSettings` | `dev_125_sales_order_drawer_settings_test.go` | 待复跑并回填日志 |
| API-20260502-02 | DEV-02 | `go test ./internal/interfaces/http/sales -run SalesOrder` | `sales_order_api_test.go`、`sales_order_settings.go` 相关测试 | 待复跑并回填日志 |
| API-20260502-03 | DEV-03 | `go test ./internal/interfaces/http/company -run CompanyProfile` | `company_profile_api_test.go` | 待复跑并回填日志 |
| API-20260502-04 | DEV-04 | `go test ./internal/interfaces/http/production -run 'ProduceFinish|ProductionFlow'` | `production_flow_api_test.go` | 待复跑并回填日志 |
| API-20260502-05 | DEV-05 | `go test ./internal/interfaces/http/support -run TestDev129StockOperationCombobox` | `dev_129_stock_operation_combobox_test.go` | 待复跑并回填日志 |
| API-20260502-06 | DEV-06 | `go test ./internal/interfaces/http/production -run 'ProduceStartAPIMerges|ProduceFinishAPIMultiSpec'` | `production_flow_api_test.go` | 待复跑并回填日志 |
| API-20260502-07 | DEV-07 | `go test ./internal/interfaces/http/support -run 'TestDev129QualityDrawer|TestDev131QualityDrawer'` | `dev_129_quality_drawer_test.go`、`dev_131_quality_drawer_test.go` | 待复跑并回填日志 |
| API-20260502-08 | DEV-08 | `go test ./internal/interfaces/http/sales -run SalesOrder` 与 `go test ./internal/interfaces/http/support -run TestDev132SalesOrderImageExport` | `sales_order_api_test.go`、`dev_132_sales_order_image_export_test.go` | 待复跑并回填日志 |
| API-20260502-09 | DEV-09 | `go test ./internal/interfaces/http/sales -run DeliveryNote` 与 `go test ./internal/interfaces/http/support -run TestDev132DeliveryNoteOutbound` | WIP 未跟踪测试文件 | 待提交后复跑 |

## 需求审核表 (REV)

| REV ID | 对应 PR | Van 验收动作 | 通过标准 | 状态 |
|---|---|---|---|---|
| REV-20260502-01 | PR-01 | 在订单列表筛选后打开销售单，再打开设置抽屉并保存 | 返回订单列表状态不丢，预览刷新后设置生效 | 待验收 |
| REV-20260502-02 | PR-02 | 用含公章、收款码、公账、多行说明的订单生成 PDF | 公章不变形，版式紧凑，信息完整 | 待验收 |
| REV-20260502-03 | PR-03 | 在公司设置维护公账并复制，再打开销售单 | 公司设置字段保存成功，销售单设置无公账输入项 | 待验收 |
| REV-20260502-04 | PR-04 | 在生产中修改投料/成品/余料，执行部分完工 | 出品率实时变化，库存和剩余工单符合页面数值 | 待验收 |
| REV-20260502-05 | PR-05 | 原料入库、WIP、成品转仓分别输入名称/编号过滤 | 候选项正确过滤，包装物料不进原料入库，布局不乱 | 待验收 |
| REV-20260502-06 | PR-06 | 选择同产品不同规格订单开始生产并完工 | 一个运行项含多规格 outputs，完工后各规格库存和订单状态正确 | 待验收 |
| REV-20260502-07 | PR-07 | 进入生产质检，分别选择工单/原料批次/产品批次 | 按当前类型显示“选择工单/选择原料批次/选择产品批次”，抽屉只展示对应对象并回填 | 待验收 |
| REV-20260502-08 | PR-08 | 生成销售单 PNG，再下载历史图片和最新版图片 | 响应为 `image/png`，PDF 最新版不被图片生成覆盖 | 待验收 |
| REV-20260502-09 | PR-09 | 已发货订单打开出库单，预览、确认生成、下载历史版本 | WIP 未提交，暂不验收 | 阻塞 |

## 阻塞项和下一步

1. `delivery-note-outbound-20260502`：当前有未提交改动，至少包含出库单页面、领域模型、PDF、仓储、接口和测试；必须先完成、提交、推送，再按 PR-09 验收。
2. 已进 develop 的 8 个需求建议按 REV 顺序逐项打勾；打勾时把测试命令输出、接口响应或截图链接补进 UI 表。
3. 验收完成后再清理已合入分支；不要删除 `delivery-note-outbound`，它仍有工作未完成。

## 本次整理的验证证据

- 已执行 `git fetch origin`。
- 已确认 `origin/develop` 最新三条：`f64bb32 Merge quality target specific drawer`、`fc7ff80 Merge remote-tracking branch 'origin/develop' into codex/quality-target-specific-drawer-20260502`、`2c84b6f feat: support sales order image export`。
- 已确认当前整理分支从 `origin/develop` 创建，不直接修改 `develop`。
- 已确认 `memory/` 被 `.gitignore` 忽略；本文件是需要进 git 的历史记录。
