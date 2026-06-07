# PR-440 商品、分组、价格模型二次修正验收记录

## 范围
- 商品档案收敛为 Item：只维护商品资料、商品分组、库存单位、整数库存、行业字段、状态、备注、BOM 使用摘要、价格摘要和客户引用。
- 删除独立客户商品新业务入口；历史客户商品只读兼容，后续迁移到商品档案客户引用。
- 分类管理改名为分组管理；分组是泛化主数据能力，不写死商品、物料或 BOM 对象。
- 商品价格管理改为价格计算模板 / Pricing Rule；新版阶梯价模板只定义档位结构。
- 商品价格表作为 Price List / Item Price 平铺价格行，按 `商品 > 子组 > 父组 > 默认` 解析阶梯价模板和 Pricing Rule。

## 本地验收项
- [x] 菜单出现 `分组管理`，不出现 `客户商品` 或 `商品分类管理`。
- [x] 商品档案出现 `客户引用`、库存单位、整数库存和价格摘要，不出现报价单位、录单单位、单位模板、商品配置模板、旧阶梯价模板或固定价格字段。
- [x] 分组管理、分组项树、使用功能、排序、启停和备注已建立 schema/API/service/UI 入口；分组不写死商品、物料或 BOM 对象类型。
- [x] 商品价格管理只展示 `价格计算模板 / Pricing Rule` 和新版阶梯价模板，不展示最终价格记录或阶梯方案。
- [x] 商品价格表页面按 Price List / Item Price 展示平铺价格行、分组选品、`商品 > 子组 > 父组 > 默认` 模板继承和发布快照固化口径。
- [x] 录单商品行显示 `报价来源：价格表 {版本}`；旧版价格表提示改为 `非最新价格表`。
- [x] 商品价格表完整生成向导落地：选择分组、勾选分组项、设置默认/父组/子组/商品行模板，并生成可编辑平铺价格行。
- [x] 发布快照固化最终价、价格单位、库存换算、分组快照、阶梯价模板来源、Pricing Rule 版本、成本来源、客户引用显示快照和人工调整标记。
- [x] 订单详情和销售单展示只读 `报价来源` 和 `生产来源` 两块追溯；BOM、工单或原料变化不自动改已发布价格表或已成交订单价。
- [x] 增加部署后 API 场景验收脚本 `scripts/scenario_acceptance.py`，覆盖 `POST_DEPLOY_ACCEPTANCE_SCENARIOS`：脚本自造客户、原料、商品、分组、Pricing Rule、阶梯价模板、客户引用、价格表发布和订单数据；运行后自动撤回价格表、失效订单、停用/废弃测试主数据，清理失败即验收失败。

## 验证命令
- RED frontend: `cd orderapp-remote/frontend-vue-shell && node --test src/lib/product-settings.test.js`
- RED support: `cd orderapp-remote && go test ./internal/interfaces/http/support -run TestDev440 -count=1`
- GREEN targeted: `cd orderapp-remote/frontend-vue-shell && node --test src/lib/product-settings.test.js`
- GREEN frontend: `cd orderapp-remote/frontend-vue-shell && node --test src/lib/order-entry.test.js src/lib/menu-ia.test.js src/lib/product-settings.test.js src/lib/product-bean-list-split.test.js src/lib/costing-bean-list-version-ui.test.js`
- GREEN support: `cd orderapp-remote && go test ./internal/interfaces/http/support -run TestDev440 -count=1`
- RED deeper frontend/API: `cd orderapp-remote/frontend-vue-shell && node --test src/lib/bean-list-pdf.test.js src/lib/order-entry.test.js`
- RED deeper backend: `cd orderapp-remote && go test ./internal/application/costing -run TestPublishBeanListRequiresPR440PriceListSnapshotMetadata -count=1`
- RED deeper backend: `cd orderapp-remote && go test ./internal/interfaces/http/sales -run TestOrderAPIDetailAllowsCustomerWorkbenchBoundOrder -count=1`
- RED deeper support: `cd orderapp-remote && go test ./internal/interfaces/http/support -run TestDev440ProductGroupPriceRemodelFrontendAndDocs -count=1`
- GREEN deeper frontend: `cd orderapp-remote/frontend-vue-shell && node --test src/lib/bean-list-pdf.test.js src/lib/order-entry.test.js`
- GREEN deeper backend: `cd orderapp-remote && go test ./internal/application/costing -run TestPublishBeanListRequiresPR440PriceListSnapshotMetadata -count=1`
- GREEN deeper backend: `cd orderapp-remote && go test ./internal/interfaces/http/sales -run TestOrderAPIDetailAllowsCustomerWorkbenchBoundOrder -count=1`
- GREEN scenario dry-run: `cd orderapp-remote && python3 scripts/scenario_acceptance.py --dry-run`
- Post-deploy scenario: `cd orderapp-remote && python3 scripts/scenario_acceptance.py --base-url <dev-app-url> --cookie '<auth-cookie>' --allow-writes`
- Broader: `cd orderapp-remote/frontend-vue-shell && npm run build`
- Broader: `cd orderapp-remote && go test ./...`
- Broader: `scripts/verify_kferp.sh changed` from repository root

## 当前状态
- 2026-06-07：PR-440 剩余三项开发切片已完成并部署到 development。Product Design 没有已保存 KFerp 设计上下文，本轮按现有 KFerp Vue 后台密集表格 + 右侧抽屉风格落地。
- 2026-06-07：部署后第一次场景脚本验证暴露旧客户价格表 public SKU 校验仍按独立客户商品模型拒绝 PR-440 公共商品档案行；已补 `TestBeanListProductScopeAllowsPR440CustomerPriceRowsForPublicProducts` 并部署修复。
- 2026-06-07：部署后第二次场景脚本验证暴露录单价格解析只读旧 `commercial_wholesale_tiers`，未读取 PR-440 `price_rows`；已补 `TestPublishedPricingMatchesPR440FlatPriceRows` 和 `TestOrderAPICreatesCommercialOrderFromPR440FlatPriceRows` 并部署修复。
- 2026-06-07：最终部署 `origin/develop=64837101570b60e9d10730cf6e9b03554eb58649` 到 development，备份 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260607151119`。部署脚本通过 Vue build、miniapp typecheck/build、Docker build 和容器内 `go test ./...`。
- 2026-06-07：部署后 `scripts/scenario_acceptance.py --allow-writes` 通过，run `PR440-SCENARIO-20260607-5HR4C6` 自造客户 `172`、原料 `48`、商品 `542`、分组 `3`、Pricing Rule `3`、阶梯价模板 `3`、客户引用 `3`、价格表发布 `59`、订单 `1526`；脚本结束后订单作废、价格表撤回、商品/客户/分组/Pricing Rule/阶梯价模板/客户引用停用、原料废弃。
- 2026-06-07：浏览器验收通过：商品档案、分组管理、商品价格管理、商品价格表、录单和销售单 `order_id=1526` 均无 `请求失败` 和 console error；销售单追溯展示报价来源 `PR440-SCENARIO-20260607-5HR4C6-PRICE`、最终价 `88/kg`、档位 `1kg+`、Pricing Rule `PR440-SCENARIO-20260607-5HR4C6-v1`，并展示生产来源。
