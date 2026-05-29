# PR-385 SKU/BOM 继承与派生自有 BOM 验收记录

## 验收范围

- SKU复制后默认记录 BOM 来源为 `inherit_current`，不复制 BOM 明细。
- BOM 配方维护展示来源 SKU 编号、名称和 BOM 版本；继承 BOM 只读。
- 点击“派生自有 BOM”后复制当前有效 BOM 到目标 SKU，并保存来源 SKU/BOM 快照。
- 成本核算、产品价格表、生产计划和客户履约下单校验按有效 BOM 读取继承或派生后的 BOM。

## 验收证据

- 单元/源码守卫：`go test ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/catalog ./internal/application/bom -count=1`
- API/集成：`go test ./internal/interfaces/http/bom ./internal/interfaces/http/catalog ./internal/interfaces/http/production ./internal/interfaces/http/costing ./internal/interfaces/http/customerfulfillment ./internal/interfaces/http/support -count=1`
- 前端：`node --test src/lib/bom.test.js src/lib/product-settings.test.js`
- 构建：`npm --prefix orderapp-remote/frontend-vue-shell run build`
- 操作手册：`OP_MANUAL_INVENTORY_MATERIALS.md`、`OP_MANUAL_COSTING.md`、`OP_MANUAL_PRODUCTION.md`
- 部署：development `origin/develop=a834c723c824fb19ce783925b8e310030244e1e4`，备份 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260529225513`。
- 浏览器验收：`https://erp.qacoohee.com/app/vue-shell?view=bom&product_id=497`
  - 初始状态：客户 `PR385验收客户0529230347` 的 SKU 497 显示 `继承：SKU-496 PR385源SKU0529230347 / BOM V001`，预期损耗率、失效、保存组件、删除、保存版本、启用版本按钮均禁用。
  - 派生操作：点击“派生自有 BOM”后页面提示“已派生为自有 BOM”，BOM 来源变为 `自有 BOM，派生自：SKU-496 PR385源SKU0529230347 / BOM V001`，组件编辑按钮恢复可用。
  - 独立编辑：通过 BOM API 将目标 SKU 497 的物料比例改为 90%，刷新页面后目标 BOM 合计比例/组件用量为 90%；来源 SKU 496 的 BOM 仍为 100%。
  - 审计证据：`audit_logs` 中目标 SKU/BOM 可查到 `inherit_current` id 3889、`copy_sku` id 3890、`derive_owned` id 3895、`product_bom_item save` id 3897；`operation_logs` 中 `/api/bom/:product_id/derive-owned` 和 `/api/bom/item/save` 均为 200。

## 验收清单

- [x] 客户 SKU 初始显示“继承：来源 SKU / BOM 版本”。
- [x] 继承 BOM 中保存预期损耗率、组件、失效和版本启用按钮不可操作。
- [x] 点击“派生自有 BOM”后，顶部显示“自有 BOM，派生自：来源 SKU 编号 + 来源 BOM 版本号”。
- [x] 修改派生 BOM 后，来源公共 BOM 不变。
- [x] 产品价格表和生产计划使用派生后的 BOM；未派生时使用继承 BOM。
- [x] 操作日志可按目标 SKU/BOM 查到继承和派生来源。
