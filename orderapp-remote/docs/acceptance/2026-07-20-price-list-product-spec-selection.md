# PR-541 商品价格表按商品选择销售规格验收

## 范围

- 商品档案为父商品维护权威默认规格，并通过正式 API 和操作日志切换。
- 商品价格表按父商品聚合，默认选择默认规格并允许同时选择多个具体规格。
- 新阶梯模板按销售规格件数解释；发布、PDF 和订单冻结并匹配具体 SKU。
- 历史发布、历史 PDF、订单及财务快照不迁移、不重写；只部署开发环境。

## RED（实现前）

- `go test ./internal/interfaces/http/support -run TestDev541PriceListProductSpecSelectionContracts -count=1` 失败：缺少 PR-541/DEV/REV 种子，以及 `default_sku_id`、`product_spec_selections`、`sales_spec_count` 和三份手册合同。
- Catalog 默认规格迁移/API、价格表父商品与规格选择、发布快照及订单具体 SKU 匹配的定向 RED 证据由对应 DEV 项补充。

## GREEN（实现后）

- Catalog：增加父商品 `default_sku_id`、回填优先级、唯一兼容投影、模板同步回退和 `PUT /api/product-settings/products/{parent_id}/default-sku`。Catalog service/repository/API 定向测试通过；`default_sku_backfill_test.go` 会在有 PostgreSQL DSN 时真实运行两次回填并校验优先级和幂等性。
- 选品/草稿/计价/PDF：父商品聚合、默认规格、多规格勾选、分类不覆盖手工选择、草稿默认规格变更提示、规格级固定价隔离、开放末档和历史单位兼容的前端定向测试通过。
- 发布/订单：后端拒绝跨商品、停用、伪造规格快照、显式空选择和未定价规格；新价格行按 `sales_spec_count` 处理阶梯、总价和每件优惠，零售/商用发布不串版本，手动价的数量口径可随提交和编辑回显往返。历史缺少新标记的行仍走旧重量逻辑。
- 全量后端：`go test ./... -count=1` 通过。PR-541 前端定向集为 384/384；前端全量 `node --test src/lib/*.test.js` 为 734/740，6 条失败与干净 `origin/develop` 相同，均是既有工作区/客户上下文合同；PR-541 新增与受影响测试全绿。`npm run build` 通过（401 modules）。
- 独立复核发现的开放末档、父商品 self-SKU、快照伪造、空选择绕过、件数总价/折扣、零售发布加载、跨发布回退和手动价口径丢失均在集成前修复并增加回归测试。

## 开发环境验收

- 待补：部署提交、应用备份、容器及 API 冒烟。
- 待补：商品档案默认规格操作日志，以及“初晓”默认规格、多规格价格行、预览和发布链路。
- 生产环境未部署、未写入、未切换入口；现有价格表不会自动重新发布。
