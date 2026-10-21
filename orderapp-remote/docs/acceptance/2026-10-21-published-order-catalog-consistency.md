# PR-707 发布价格表到录单目录一致性

日期：2026-10-21。DEV-810 / DEV-811 / DEV-812；Van 产品验收待确认。

## 业务规则

员工在网页或小程序手动选择对应已发布价格表。客户类型和订单类型不自动改选商用/零售表，也不删除表内有价商品。每个实际发布分类选择一张表；选表确定商品、BOM 规格、数量档位和价格。客户自助入口的具体版本/用途绑定继续严格隔离。

## 生产复现与原因

生产只读快照来自 main973d5481 的 `/app/api/order/form`。用户提供复现客户黄艳，编号487，默认订单类型为零售。

原员工接口根据默认订单类型推断 retail，再按旧 product_kind 限制表类型。生产当前非生豆表为 commercial，因此已发布的熟豆、速溶和自定义商品被二次删除，只剩5个生豆商品。网页、小程序读取和提交各自拼接目录，重复的业务规则容易继续漂移。

同一生产快照回放修复后得到43个商品、46个BOM规格：按旧product_kind计为roasted31、green_bean5、instant_coffee4、generic3；实际分类以发布表为准，不由该旧字段推断。未修改生产商品、客户、价格表、订单或配置。

## 架构与兼容

- `application/sales/order_catalog.go` 负责统一商品/规格投影，员工两端及小程序提交复用；HTTP 层保留薄兼容适配。
- 网页新增只读 `/api/order/catalog`，与员工小程序目录共用 `ResolveOrderCatalog`；沿用录单权限、客户范围及 no-store。目录读取不增加业务写操作。
- 分类来自实际所选发布表，支持新的商品分类。commercial/retail 保留当前版本供手动选择，不按客户类型筛选。
- 读目录时仅跟随同一归属、分类、表类型和 table_key 的新版。移除命名表不能静默替换为另一表；保存旧版本仍严格拒绝。
- 保存事务读取条目发布表的实际 list_type，避免缺少客户端来源信息时再次按零售订单路由。归属、当前版本、所选表和精确发布规格校验保持；历史冻结换算和样品例外继续走原守卫。
- 网页等新目录到达后才校验切表明细；请求失效或失败时关闭候选，不拿全商品或其他客户响应补齐。发布换版保留已有明细，要求核对。
- 无数据库迁移、业务数据修复或新增业务写路径。原发布及保存订单操作日志继续生效。

## RED / GREEN

证据目录：工作区 `outputs/pr707-catalog-consistency/`（私有生产快照不提交仓库）。

| 行为 | RED 证据 | GREEN 验证 |
| --- | --- | --- |
| 零售客户仍可选 commercial 表内熟豆与自定义商品 | red.log，仅生豆 | application/sales、mini_employee_catalog_consistency_test.go |
| 两种报价表供手选 | manual-table-red.log，仅保留零售选项 | price_table_selection_test.go |
| 网页共享目录 | erp-api-red.log，404 | order_catalog_api_test.go；真实数据库API |
| 新发布同名表刷新、移除表禁止替换 | publish-refresh-red.log、removed-table-red.log | order_catalog_consistency_test.go |
| 新商品分类 | mini-red.log、web-red.log | orderCatalogCategories.test.ts、order-catalog.test.js |
| 切表等待新目录并保留合法规格 | vue-table-switch-red.log，误标失效 | 真实 Vue setup/runtime 测试及浏览器操作 |
| 零售订单按手选商用表保存 | submit-routing-red.log，缺少价格 | named_price_table_order_api_test.go，事务保存及冻结来源断言 |
| 目录权限与录单权限一致 | catalog-permission-red.log | support/order_catalog_permission_test.go |

验证结果：

- Vue shell 全量 Node 测试1444项通过；Vite生产构建通过。
- 小程序64个测试文件、372项通过；vue-tsc和production构建通过。
- `go test ./...` 通过（需要数据库环境变量的旧集成测试按既有约定跳过）。
- 本地隔离 PostgreSQL16：真实发布 writer → order-form 查询 → 统一目录，覆盖默认表、非默认表、零售报价表、任意分类、草稿不生效、再次发布、同名表跟随、旧价提交拒绝、撤回、表外规格和跨表来源拒绝；包含发布操作日志断言。日志 postgres-lifecycle.log / postgres-guards-green.log。
- 真实订单API：零售客户选商用客户表读目录并保存，确认冻结表名、版本和单价；跨表、其他客户、表外BOM规格/手动价绕过拒绝。日志 postgres-api-green.log。
- 浏览器使用隔离数据挂载实际 OrderEntryView：零售客户选择新分类商品，标准表38，手动切零售表后保留袋装规格和数量1，价格变为48。截图 manual-price-table-browser.png；不提交真实业务订单。
- 尝试扩大运行旧数据库套件发现既有夹具漂移（orders缺列、旧列索引断言等）；在未修改的c591b765基线复现并保留 postgres-baseline-full.log。未将这些旧用例宣称为通过，也未扩大本次修复。与本需求相关的数据库生命周期、命名表保存和精确规格守卫均单独通过。

## 手册

`docs/OP_MANUAL_ORDER_SALES.md`、`docs/OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`、`docs/OP_MANUAL_COSTING.md`；网页录单内置手册同步。仍只维护orderapp-remote/docs单一来源。

## 交付边界

已完成实现和上述验证，正在按本轮生产部署授权整合develop/main。实际运行提交、回滚镜像、健康检查、需求表及目录只读核对、小程序RELEASE_INFO/PAGE_FILE_MANIFEST以发布回执为准。代码实现、服务器运行、小程序包、微信上传/审核/正式发布及Van业务验收分别记录。

## 历史冻结兼容补充

最后核对发现，新增的目录刷新可能把历史冻结换算/原样品行的旧发布ID当成当前选表，并在换版或手动切表时将其误标失效。真实Vue运行时回归先RED（frozen-catalog-red.log），修复后覆盖以下边界：

- 仅供历史冻结行使用的旧表不阻塞当前目录读取；仍在当前候选内的表可继续手选。
- 冻结换算、已存样品原行在刷新、切表及重新核价时保留原规格、数量、价格、来源和计价方式。
- 普通行和历史行共同使用旧表时，普通行仍必须按当前发布校验；不能借冻结行跳过校验。
- 复制新订单不属于历史冻结编辑，继续使用当前已发布表。

日志：frozen-catalog-green.log及frozen-catalog-vue-all.log。后端已有的可信原行/冻结快照守卫不变，无业务数据修复。

中间生产版本be6f46bc实际只读核对：黄艳43商品/46规格，发布分类为烘焙咖啡豆27、咖啡挂耳3、速溶咖啡5、咖啡生豆5、好玩的产品3；所有阶梯均在所选发布ID内、价格大于0。完整生产健康和HTTP证据在production-runtime-be6f46bc.json。最终交付将包含上述历史兼容补充，最终运行与包提交以发布回执为准。
