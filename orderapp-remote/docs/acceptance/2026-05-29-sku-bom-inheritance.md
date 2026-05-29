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
- 浏览器验收：待本分支部署或本地服务启动后补充截图/步骤。

## 验收清单

- [ ] 客户 SKU 初始显示“继承：来源 SKU / BOM 版本”。
- [ ] 继承 BOM 中保存预期损耗率、组件、失效和版本启用按钮不可操作。
- [ ] 点击“派生自有 BOM”后，顶部显示“自有 BOM，派生自：来源 SKU 编号 + 来源 BOM 版本号”。
- [ ] 修改派生 BOM 后，来源公共 BOM 不变。
- [ ] 产品价格表和生产计划使用派生后的 BOM；未派生时使用继承 BOM。
- [ ] 操作日志可按目标 SKU/BOM 查到继承和派生来源。
