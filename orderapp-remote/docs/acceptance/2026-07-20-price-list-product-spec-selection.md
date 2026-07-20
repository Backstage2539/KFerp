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

- [x] 行为实现先合并为 `f3b040c0431c17a3e9f9fba5a2a9bc2d31f3b8b9`；首次浏览器加载发现 `parent_product` 重复别名后，RED 回归测试复现 `SQLSTATE 42712`，修正版本合并为 `96478bd19c57dae40776969d6b4159a2563a8ea0` 并重新部署。行为版本备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260720193327`。
- [x] `erp_orderapp` 正常运行，`erp_postgres` healthy，`erp_docconvert` 正常运行；容器构建再次执行完整 `go test ./...` 并通过。认证后的 `/app/`、`/app/api/product-settings`、`/api/req/product?limit=1` 均返回 200，未认证 API 返回 401。
- [x] 默认规格启动迁移在开发库连续运行两次。两次均为 533 个父商品、无效 `default_sku_id` 为 0、`is_default_sku` 投影不一致为 0、不是“恰好一个有效默认规格”的父商品为 0；第二次启动后 API 在第 7 秒恢复 200。
- [x] 开发浏览器中商品价格表从 386 个 SKU 正常加载为父商品选品；分类首次选择只带默认规格，咖啡豆显示 `42 款 / 42 规格`。追加“白月光瑰夏”的 454g 和 1Kg 后变为 `42 款 / 44 规格`，分别生成 `个227g`、`个454g`、`个1Kg` 的独立阶梯和价格行，页面控制台无错误。
- [x] “初晓”（父商品 619）权威默认规格为 SKU 990 `227g袋装`，价格表父商品行自动选中该规格，另一个有效 `100g袋装` 未被自动选中。当前开发主数据没有“初晓”的有效磅和 1Kg SKU，故未为了验收改写规格模板；磅/1Kg 多规格逻辑由前端、API、发布和订单定向测试覆盖，实时磅单位行也验证为按件数显示。
- [x] 浏览器仅改变未保存的本地选品状态，没有点击保存、发布、撤回或生成 PDF；开发环境现有 V3.0.18 未重新发布。
- [ ] 未在开发主数据上实际切换默认规格，因此本轮没有制造新的默认规格操作日志；原子 API、跨商品/停用/失效拒绝及操作日志写入由 Catalog API 测试覆盖，留待 Van 需要时做业务写入验收。
- [x] 生产环境未部署、未写入、未切换入口；现有价格表未自动重新发布。
