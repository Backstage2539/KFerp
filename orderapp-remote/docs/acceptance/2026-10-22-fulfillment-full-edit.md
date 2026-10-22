# PR-714 完整代发订单编辑验收

状态：生产5086dfe6已发布，回归、独立审查和生产只读核验完成；Van业务验收待完成。

## 原问题与生产基线

- 生产db427e4d，订单列表首次打开库存代发117出现Vue空节点_assign/_vei错误；外层隐藏工作流Teleport目标且子组件先渲染空白新单表单，关闭/保存异常。
- 上一版仅收件编辑分支移除了商品编辑、费用和完整操作，不满足业务需求。
- 117两项227g商品各4袋，预占来源购买115（SO-20261022-0002）待入库权益；商品和快照未丢失，尚无实物出库。
- 已保存服务费300分、预估运费1010分，物流2160g+包装500g=2660g，原模板1/V2；预估不计当前运费应收。

## 已确认规则

- 可选该客户全部有效现货/待入库权益，而非仅原购货单。
- 地址/规格/数量变化使用原单模板版本与原计费设置；原数据读取不回算。
- 真实生产115/117只读，保存测试使用隔离数据库。

## TDD与验证

- HTTP新路由RED：尚未注册，GET/POST/PATCH为404，预期权限401/403；日志outputs/fulfillment-full-edit-20261022/api-routes-red.log。
- 真实 Vue OrdersView + OrderEntryView DOM RED：空 Teleport 目标/异步默认空行复现；修复后覆盖加载、关闭、保存、失败保留输入、重试、重开、增删/换规格/数量、无库存原价。
- 隔离 Chrome 从真实列表组件打开完整编辑，首次409保留地址/数量，重试成功后关闭重开保留已存值，console error为空（local-browser-evidence.md）。
- API权限与严格JSON：匿名/无权限/客户渠道/客户范围拒绝，客户和代发类型只能从原单确定，manual服务费必须有客户维护权限；旧地址入口复用新事务。
- 运费真实PG RED/GREEN：原模板/政策/物流重量，2160+500=2660g、1010分预估/300分服务费；现模板版本和BOM重量变更不改原快照，同输入复用小程序费用服务。
- 服务费调整理由、已结算保护、历史实收和凭证；历史grand_total为0的兼容、零额已付单增加应付转未付、gross120/net115/discount5仅改地址完整保留也有PG回归；真实ConfirmImport与编辑竞争共享锁，实际出账1234分后拒绝旧预览保存。
- 数量0历史行保留FK历史，但从当前订单明细、商业单据、发货导出、生产需求排除；重量冻结仍拒绝负数量；完整发货状态不被已移除明细误判。
- 独立审查修复：审计包含地址/日期/备注前后值；停用新下单入口不影响旧单读取；地址或地区变化必须一致；同规格换制造版本使用新明细ID保留历史。
- 全量Go（合入076fbb82最新收款修复后）通过；前端1496项及生产构建通过，实际父子DOM包含地址A→B→A、清空规格阻止保存和本单原占用展示。
- 真实PG HTTP权限/原客户地址兼容、普通订单收款/运费回归通过（api-payment-final-pg.log）；PR703三项原夹具缺物流重量在建原单前失败，补齐权威454g/1000g/V2 454g后原断言全部通过，未放宽业务校验。
- 事务后段审计故障注入确认商品/权益/费用/地址/单据缓存全部回滚；生产收货转换后旧编辑预览失效，取消后旧草稿不能复活。
- 活跃代发明细唯一索引最小迁移：仅qty>0参与唯一；已移除历史行保留且同规格新制造版本使用新ID。重复迁移与活跃重复拒绝均有PG验证。
- 所有日志保存在 outputs/fulfillment-full-edit-20261022/，含RED/GREEN与隔离浏览器证据。完整代发/费用及PR703、orderbeans、production相关36项PG通过；最后净货款合同5项再次通过，104.80原净货款在context/preview/save和前端显示一致。生产只读验证与部署回执见下文。

## 合并后检查与测试边界

- 发布候选5086dfe6（feature79af2304、develop79af2304）合入main后全量Go通过；实际PG完整代发及销售仓储专项通过，精确新API/旧收件兼容/收款API测试通过（main-full-go.log、main-pg.log、main-api-exact-pg.log）。
- 扩展PG名称匹配触发6项旧测试失败：两项旧付款夹具缺canonical BOM配置、两项履约列表夹具返回空、确认测试缺order_freight_snapshots、预付款夹具缺orders.delivery_mode。独立导出发布前main1ab75187逐项复跑，6项错误完全相同（baseline-1ab75187-six-legacy-tests.log）；属于已有测试边界，不计为本次通过，也未放宽或跳过这些原断言。
- 独立合并审查确认原main收款保护及普通订单前端代码未被覆盖；生产既有BOM保护保留，仅ACTIVE_REQUIREMENTS记录冲突，已保留双方记录。

## 交付

- 分支codex/fulfillment-full-edit-20261022，DEV-835/836/837/838。
- 2026-10-22生产已部署5086dfe63c46a039328be258fff5f261f8d86daa；feature79af2304已push、合入develop，再merge main。部署脚本退出0，应用容器running/restart_count0，登录HTTP200、匿名保护API401。
- 服务器门禁：Vue1496/1496、mini425/425、typecheck与24页小程序产物核验、全量Go及镜像内Go测试均通过。未上传/发布微信版本。
- 运行镜像sha256:8c71a4c997fcaa668fcd052f7854a487b7d96bfca07a9a5101a8c4e1efd7fd57；公开主JS、服务器dist和容器dist字节一致，SHA256 0d280c36859b670c2155f45bf4c8c86a6de2180b49d08087af67e650e3b7cc26。两份在线操作手册HTTP200且含新流程。
- 回滚源码：/opt/stacks/erp-production/orderapp.backup.deploy-20261022194805-5086dfe63c46；回滚镜像：kferp-orderapp-rollback:production-20261022194805-5086dfe63c46。
- 原订单115/117的edit_revision和全部items与发布前一致，全部运费模板内容一致。117可编辑，规格19/68各4袋，原模板1/V2、物流2160g+包装500g=2660g、预估1010分、服务费300分、pending_actual且当前运费应收0，实际运费待账单。
- 生产Chrome新标签页仅筛选SO-20261022-000117，从真实订单列表首次打开，完整收件输入、两行商品编辑、费用/订单信息正常；刷新、关闭、重开实测正常，两个保存按钮enabled，console error为空。没有点击生产保存、核算、取消或增删商品；全部写入/库存测试使用隔离数据。
- 证据：production-deploy.log、production-verification-summary.json、runtime-identity.json、production-items.png、production-fees.png、production-browser-evidence.md。原始订单快照仅本地outputs保留，未提交仓库。
- PR714与DEV835–838已出现在生产需求界面；产品需求保持review由Van验收，交付记录review待业务验收。Van业务验收仍待完成。
- 发布后的验收回执为文档提交；运行代码身份以5086dfe6为准。
