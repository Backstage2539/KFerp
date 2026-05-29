# 2026-05-29 客户账户、模板范围和仓库绑定修正验收记录

## 范围
- ERP 顶部“客户账户”选择器展示已开通门户/工作台的客户，不再要求外部账号、密码或能力模板。
- 未绑定能力模板的客户可进入空客户视角；客户侧服务页返回空页面元数据，提交动作仍由能力校验拒绝。
- 仓库绑定客户入口移动到 仓库库存 → 当前仓库 → 仓库设置 右侧抽屉。
- 客户账户库存只显示公共仓库和当前客户绑定仓库，不显示其他客户绑定仓库。
- 商品配置模板、阶梯价模板按当前客户上下文过滤；复制公共模板后，客户列表隐藏已派生的公共原模板。
- 商品配置模板里的阶梯价模板下拉允许选择公共模板和当前客户自己的模板。

## 验收项
- [ ] 新增 `karen`，只打开“开通客户门户/工作台”，不建账号、不选能力模板；ERP 顶部“客户账户”下拉能选到 `karen`。
- [ ] `karen` 未设置外部账号密码时不能登录小程序；未绑定能力模板时服务页为空，提交代发/代加工/结算动作被拒绝且不写业务数据。
- [ ] 仓库库存选择一个仓库，点击“仓库设置”打开右侧抽屉，绑定 `karen` 后保存；切到其他客户账户时看不到该仓库，工厂总览仍可见。
- [ ] 在 `karen` SKU设置中，阶梯价模板列表不显示凯哥客户模板；复制公共阶梯价模板后，公共原模板在 `karen` 模板列表中隐藏，只显示客户副本。
- [ ] 在 `karen` SKU设置中，商品配置模板列表不显示凯哥客户模板；复制公共商品配置模板后，公共原模板在 `karen` 模板列表中隐藏，只显示客户副本。
- [ ] 编辑 `karen` 商品配置模板时，阶梯价模板下拉可选择公共模板和 `karen` 自己的模板，不显示其他客户模板。

## 自动化证据
- 2026-05-29：`go test ./internal/application/customerportal ./internal/interfaces/http/customerfulfillment ./internal/infrastructure/postgres/stock ./internal/infrastructure/postgres/customerfulfillment -count=1` 通过。
- 2026-05-29：`go test ./... -count=1` 通过。
- 2026-05-29：`node --test src/lib/customer-management-source.test.js src/lib/product-settings.test.js` 通过。
- 2026-05-29：`node --test src/lib/*.test.js src/api/*.test.js` 通过，408/408。
- 2026-05-29：`npm run build` 通过。
- 2026-05-29：浏览器 mock 数据验收通过：仓库设置抽屉可打开，客户库存只显示公共仓和 `karen` 专属仓，隐藏 `凯哥` 仓；`karen` SKU 设置只显示 `karen`/未派生公共模板，隐藏已派生公共模板和 `凯哥` 模板。

## 手册更新
- `docs/REQUIREMENTS.md`
- `docs/ACCEPTANCE_TESTS.md`
- `docs/OP_MANUAL_CUSTOMER_PORTAL.md`
- `docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `docs/OP_MANUAL_WORKSPACE_MODE.md`
- `docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `docs/OP_MANUAL_COSTING.md`
