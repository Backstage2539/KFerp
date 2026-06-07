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
- 2026-06-07：本地 PR-440 剩余三项开发切片已完成。Product Design 没有已保存 KFerp 设计上下文，本轮按现有 KFerp Vue 后台密集表格 + 右侧抽屉风格落地；后续状态为完整 verification、合并 develop、部署 development，并运行场景脚本和浏览器/API 验收。
