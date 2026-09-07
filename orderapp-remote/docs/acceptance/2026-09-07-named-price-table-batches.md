# PR-635 同版本多张命名价格表

状态：实现与本地验证完成，development 交付处理中，待 Van 验收。

范围：一个归属、商品类型、版本下多张命名价格表；整组发布、唯一默认表、独立商品规格/定价/样式、按表录单及客户下单。历史单表不迁移、不回算。

## 实现

- 每张表仍以独立 publication ID 保存快照和 PDF，`config_json.publication_batch` 冻结共同 release_id、跨版本稳定 table_key、table_name、is_default_table；DTO 暴露这些字段。
- 新增 POST `/api/costing/bean-list/draft-batches` 与 `/api/costing/bean-list/publication-batches`，公共信息 + default_table_key + tables[]。归属由认证范围解析，不能伪造 owner。
- 事务中对归属/商品类型加锁，正式版本只分配一次；所有行及审计日志一并提交。草稿不占用正式版本号。撤回/归档/恢复扩大到整个 release。
- PDF 缓存继续包含具体 publication ID；命名表新 ID 不会命中历史缓存。PDF 生成发生在提交后，单个文件失败返回已提交结果与 pdf_errors，下载接口可以重试生成。
- ERP 和员工小程序请求保存 selected_price_table_ids；客户现货订单提交具体 bean_list_publication_id。当前版本优先客户表、再公共回退；服务器校验选表范围及行价格来源，命名表即使省略选择字段也不能混表。同商品类型一张表，不同类型分别选。
- 订单 price_source_json 补充冻结表名、table_key 和 release_id，历史展示使用快照。小程序选择当前版本同组其他表不再被旧“只允许默认 publication ID”的判断拒绝。

## RED → GREEN

- 多表服务/仓储 API 测试最初因缺少类型与方法编译失败；实现后 1、2、4 张表、命名/默认约束、草稿深拷贝及整组验证通过。
- 批量 HTTP 路由最初 404；实现后权限、伪造归属覆盖、三表发布及单张 PDF 失败返回已发布结果通过。
- PostgreSQL 三表批量发布：第三张插入失败时 publication/audit 均为 0；正常三张同版本。默认第一张的测试起初取到最后一张，按冻结默认标记解析后通过。四个并发批次各自共享版本且批次间版本不冲突。
- 前端 helper 最初缺失，配置按钮位置测试起初失败；实现后名称校验、5 张表、默认唯一、草稿隔离、分组和按钮相邻通过。
- 选表服务最初缺少类型/方法；实现后默认解析、主动选非默认、同商品两表不同价、跨客户/非法 ID/重复类型/旧版本拒绝通过。
- 客户商品目录测试最初缺少字段/过滤器；实现后 227g 与 1kg 同商品按所选表过滤，分别返回对应发布 ID 及单价。

- 发布预校验：缺失最终价格的扁平行起初被接受；增加商品/正价格校验后，整组发布在事务前拒绝，定位表名。
- 手动报价绕过检查：命名表之外的商品、规格或数量档位即使传入手动单价也拒绝；正常表内手动报价沿用现有规则。
- 客户目录：起订量大于 1 的已发布规格起初未显示；目录按首个有效档位展示，提交仍按实际数量验证，RED/GREEN 通过。

## 验证证据

- `scripts/verify_kferp.sh backend`：全部 Go 单元/API测试通过。
- `scripts/verify_kferp.sh frontend`：Vue 1098 项测试及生产构建通过（构建保留已有大包提示）。
- `npm --prefix miniapp test`、`run typecheck`、`run build:mp-weixin:development`：237 项测试、类型与开发构建通过。
- 临时 PostgreSQL 16（本机 55435）运行 `TestNamedPriceTableBatchPostgres*`、`TestNamedPriceTableOrderPostgres*` 与 `TestIsCurrentDefaultOrderPublication*`：真实事务回滚、并发版本、整组生命周期、当前版本同组两张均有效、跨客户/旧版/非法 ID、混表与冻结表名通过。
- `TestMiniEmployeeOrderFormSelectsNamedSiblingAndRejectsCrossTableIDs`：GET 默认 30、切另一表 90；仅默认表规格不返回；跨客户、同类型选两表均 400。
- 本机 Chrome + Vite，使用虚构目录与拦截 API（未写业务数据）：新增/复制到三张，命名 227g/1kg/样品，设置第三张默认；两张分别设置不同品牌样式，切回及刷新后各自恢复；配置按钮唯一并位于发布左侧，无运行时异常。
- `TestNamedPriceTablePDFSeparatesTitleVersionAndSpecifications` 渲染三张 PDF。人工查看 227g/1kg 首页：表名、共同 V3.0.6、具体规格及独立价格正确，无重叠/截断。`TestNamedPublicationPDFCacheIdentityDoesNotCollideWithinVersion` 验证三张缓存互不串用。

- `TestNamedPriceTableOrderAPIUsesSelectedSnapshotAndBlocksBypass` 使用 PostgreSQL 及当前 BOM 规格身份，真实 POST `/api/order`：同商品两表分别保存 30/90 和冻结 ID/版本/名称；选表与行来源不一致、跨客户及手动价绕过表外规格均 400。
- 全量 Go 默认门禁通过。额外启用真实数据库运行 3 项旧 ERP API 用例时，旧测试数据缺少当前 BOM 配置而返回 `product_bom_spec_not_configured`；在未修改的最新 `develop` (`56ea7d3b`) 独立复现相同失败，属于既有测试夹具问题。新增命名表真实 API 使用完整 BOM 测试数据并通过，不将旧用例标作通过。

本机证据目录：`/private/tmp/kferp-pr635-evidence`。实际用户数据端到端验收由 Van 进行；本轮浏览器验证使用虚构数据。

## 手册与待验收

单一来源：`OP_MANUAL_COSTING.md`、`OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`；Vue 页面提示与手册入口同步。

Van 可在 development 用同一商品类型建立 227g/1kg/第三张表，设置不同价格并选择非首张为默认，整组发布后核对录单、员工小程序与客户现货下单的默认、切表和报价；再检查不兼容规格阻断、客户隔离、历史及复制订单，确认下载的三份 PDF。
