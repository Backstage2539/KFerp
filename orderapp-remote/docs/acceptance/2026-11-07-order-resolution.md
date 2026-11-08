# PR-749 订单处理实施与验收记录

状态：分批实施。第一批已合并 develop/main 并部署开发及正式环境；第二批货源切换与生产货权修正完成定向验证，尚未发布。第二批及第三批尚未全部交付。本文是工程验证记录，不代表 Van 已完成产品验收。

## 范围与数据边界

普通销售、留厂购货和代发订单的显式订单处理。SO-20261031-0003 仅作需求背景；本轮没有提交其领料、库存移动、生产、订单处理或退款。写入验证使用本机隔离 PostgreSQL 的测试 schema；每个 fixture 独立创建与清理。

## 已记录的失败与修正

| 验证场景 | RED | GREEN |
| --- | --- | --- |
| 待确认方案期间新增收款 | 旧方案直接完成，缺少资金去向待办 | 最终执行前重新计算；变化后整笔回滚；重新预览更新方案后补资金待办 |
| 拆分资金处置 | 同一可退金额创建多个相互独立的任务 | 一个资金待办分次登记，共用可退上限；超额拒绝 |
| 管理员策略 | 配置存取接口缺失；配置接口 404 | 配置版本校验、操作日志、指定岗位审批、待执行单按最新限制检查 |
| 策略界面 | 页面组件缺失 | 加载现有配置、提交观察到的版本；冲突不覆盖用户输入 |
| 更新待处理方案 | 接口被旧 orders.write 规则拦截；页面未加载待处理方案 | orders.resolve 权限、路径编号绑定、保存前后快照、幂等重试；新增关联项不预选 |
| 运费与商品关闭分离 | 未选费用处置也能关闭 | 必须选择保留／减免／协商，财务确认后执行；原运费保留 |
| 费用减免后的待收 | 应收总额归零但运费仍显示待收 | 运费调整单独累计，账款查询使用减免后运费 |
| 未填协商金额 | 未填写按零金额减免 | 区分未填写与明确填写零；前后端都校验 |
| 修订来源串到下一页面 | 离开页面仍保留 copy_id、amendment_id | 切换页面清理来源参数，新修订明确传入 |

## 已通过的工程验证

- Go 领域与 HTTP 定向测试：显式选择、数量金额上限、尾差、权限、路径身份、策略、更新方案。
- 隔离 PostgreSQL：共享完成生产及其他订单保持原样；关闭、重新开启、修订草稿、撤回、未开工生产取消；预览过期、操作日志失败回滚、重复提交、关闭后新增履约拦截。
- 财务隔离样本：分次退款、留存客户余额、抵扣同客户应收；超额拒绝；原现金收款保留；重新开启后重新形成应收。
- 本轮新增 Vue 运行时测试：分项权限、失败重试标识、管理员配置冲突、待处理方案加载与更新。费用与导航纯函数测试通过。
- Vue 构建在新增策略／更新方案阶段通过；后续费用界面修改后需要再次构建。

本轮中间日志：`/tmp/kferp-resolution-20261107-round2.log`、`/tmp/kferp-resolution-20261107-build.log`。这些只对应当时工作副本，最终发布须在最新合并版本重新核验。

## 2026-11-07 后续验证

- 新增 RED/GREEN：部分发货逐行对应；开票记录变化重新核实；已出账原快照保留与当前待收调整；独立未收费用减免；公司备货负责人确认及客户货权拒绝；旧取消入口绕过保护；普通订单逐批预留释放及旧写入数量上限；失败登记和安全重试；历史月结审批不提前生效、快照变化重新审批、最终重算保留更正前后记录。
- `TestOrderResolutionAmendmentCreatesReviewedOrderThroughSalesAPI`：通过实际 `/api/order` 创建关联新单、重新审核、保留原单、不继承收款；重复保存拒绝并回滚多余订单。
- `/tmp/kferp-resolution-20261107-round9-pg.log`：隔离数据库订单处理、共享生产、PR703 预订与代发回归通过（32.064s，当时版本）。后续客户货权校验需纳入最终重跑。
- `/tmp/kferp-resolution-20261107-round8-go.log`：领域／接口／销售／履约／生产／财务／权限及路由相关 Go 测试通过。
- `/tmp/kferp-resolution-20261107-round8-front.log`：相关 Vue 测试 16/16；同轮 Vite 构建通过。后续改动仍须最终验证。
- 已完成本机浏览器抽屉检查：未预选业务处置，焦点进入与返回、滚动锁定、预览、模拟提交与待办展示正常。使用内存模拟接口，未提交业务数据库。后续本机界面复验确认：逐批释放字段初始留空、输入预留数量、失败状态和失败记录可展开查看；关联跳转返回上下文另有 Vue 运行时测试。

完整履约包回归仍有历史 BOM 样本问题，不能记录为全绿。未改动工作副本的 HEAD 归档在 `/tmp/kferp-resolution-baseline-20261107` 重现了两个代表性失败：`TestSubmitCustomerDirectShipOrderCreatesERPOrder`、`TestListMiniDirectShipRequestsUsesConsistentEffectiveShipmentTimeAcrossPackages` 均缺少 `product_bom_spec_authorities`。证据 `/tmp/kferp-resolution-20261107-baseline-pg.log`。尝试补初始化后旧样本又暴露未配置默认 BOM 等问题，已撤销该无关初始化试改；未放宽生产 BOM 身份校验。

## 仍需完成，不可视为已交付

- 已收独立费用和费用经营报表候选的最终集成／发布，及后续全场景核对。
- 在制停工及余料、转供、库存分配／报损、实物与客户货权退回仍未开放。
- 剩余并发边界、生产实际执行／入库链路、阶段二和阶段三业务的完整 API 覆盖。
- 全量受影响回归的基线问题处理或明确验证边界；最终 UI、构建及代码复核。
- 后续批次集成和发布；不得把第一批交付等同于完整需求完成。

## 手册

单一源：`docs/OP_MANUAL_ORDER_RESOLUTION.md`；前端“订单处理手册”。管理员策略、重新预览、更新方案、费用与资金必须按实际开放能力验收。

## 本轮补充回归与发布边界

- 先 RED 后 GREEN：关闭订单退出待发货／缺收件信息待办；关闭后原商品计量、单价、物流重量和价格快照不可改写；正常日期编辑保留原行；旧零总额订单保留已交付未收款；零应收不推导虚假现金；同批多笔预留不重复扣减一次出库；暂停／关闭保留客户累计已入库历史。
- 追加账单收款撤销只影响最近一笔；相同撤销重放不再次扣款；两笔不同标识的并发收款仅一次成功。`/tmp/kferp-resolution-20261107-billing-reverse.log`。
- 月结共用保护覆盖旧订单收入、费用、税票、成本及财务调整写入；强锁账期在费用／税票实际写入时再次校验。`/tmp/kferp-resolution-20261107-month-lock-red.log`、`...month-lock-green.log`。
- 配置修改也与处理提交、分项执行串行，不能在按原配置校验后、最终执行前切换规则。`...policy-lock-red.log`／`...policy-lock-green.log`，补充定向回归通过（4.033s）。
- `/tmp/kferp-resolution-20261107-round13-backend.log`：完整 Go 检查通过（未配置 PostgreSQL 的集成测试按原规则跳过）。`...round13-bootstrap.log`：实际空库初始化和重复初始化通过。`...round13-pg.log`：订单处理、留厂共享生产、PR703、代发闭环、账单、客户账款、工作台、累计购货及履约编辑隔离 PostgreSQL 回归通过。
- 前端最后一次完整验证：`...round10-front.log`，1745/1745；`...round10-build.log` 构建通过。后续本轮只调整 Go 与 Markdown，页面代码未变。
- 完整应用 PostgreSQL 回归的 6 个失败与未修改 HEAD 归档一致，均为旧 BOM 物流重量／工艺路线测试样本；没有新增失败。`...round12-finance-init.log` 与 `...baseline-appmain.log`。财务包实际 PostgreSQL 回归通过。销售旧最小 schema 的 17 个失败也已在 HEAD 归档复现，不记录为全绿。
- 服务器只读准备检查：应用均运行，数据库健康，磁盘剩余约 12 GB，发布锁空闲。这是发布前记录；后续部署回执如下，真实业务单据未修改。


## 第一批发布回执与第二批验证（2026-11-07）

- 第一批：功能提交 `87aa403c7` / `b190a4c28`，开发 `c994c9b2b168975cf2c2662e791b7ddf68dc7f8e`，正式 `2d46296a3b01957c447874d37a9e5a9dfe187abe`，均已推送。使用 `KFERP_ERP_ONLY=1`，不构建或发布小程序。
- 开发备份 `/opt/stacks/erp/orderapp.backup.deploy-20261107170312-c994c9b2b168`；正式备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261107171222-2d46296a3b01`；各自同名后缀的 `kferp-orderapp-rollback:<environment>-<suffix>` 镜像保留。
- 正式容器运行、数据库健康；未登录接口 401，已登录页面、处理查询、策略、手册及需求接口 200。公开页面入口与部署文件一致（`main-CdVA-n9M.js`）。证据 `/tmp/kferp-resolution-20261107-{development,production}-smoke.json`、`...deploy-development-retry.log`、`...deploy-production.log`。
- 目标 SO-20261031-0003 仍为正常、未作废、2160 元未收款，两行各 24 袋未交付，处理记录为 0；计划 18 取消、19 完成，代发 16/17 取消。未提交真实处理、库存或资金操作。
- 只读核查发现第一批货权守卫误把 `production_plan_items.customer_id` / `work_orders.customer_id` 的需求客户当作库存货主。计划 19 的产出仓、物料、投料及入库实际货主均为公司。新增失败测试使用带客户编号的共享生产复现；修正改按仓库、物料、加工单、预留、投料和实际出入库货权判断。反向验证客户仓、客户物料、加工及客户入库仍拒绝公司备货。共享生产、其他订单、库存与预订完整快照保持一致。
- 第二批代发换货源：逐笔明确货源和完整待入库数量；同客户、冻结商品规格版本及仓库匹配；目标不足和日志失败整笔回滚；先建立新预订再释放原预订，保留历史。已有目标预订安全累计，重复和并发提交仅执行一次。更改待确认货源生成新的仓库待办，旧确认不得授权新方案。
- 第二批 API 验证覆盖预览缺选拒绝、提交、更新方案、分项确认重放；权限测试覆盖销售不能代仓库确认。Vue 运行时和隔离浏览器核对货源/数量初始留空、缺选提示、部分草稿保存及待办展示。未使用真实订单做写入测试。
- RED：`...supply-red.log`、`...supply-pg-red.log`、`...supply-existing-red.log`、`...supply-refresh-red.log`、`...supply-ui-red.log`、`...supply-draft-red.log`、`...ownership-red.log`、`...ownership-facts-red.log`、`...options-red.log`。
- GREEN：`...supply-api.log`（3.215s）、`...supply-concurrent.log`（3.462s）、`...ownership-green.log`（45.051s，全部订单处理及留厂关闭定向数据库回归）；`...options-green.log`。`...phase2-backend.log` 完整 Go 通过（数据库测试另跑），`...phase2-frontend.log` Vue 1746/1746 及构建通过；最终集成结果待追加。
- 已完成生产的取消／修订、没有已关闭数量时的重新开启，查询接口明确返回禁用原因；关闭剩余履约仍可选择。生产、货源、原因及账期不自动预选。

- 第二批完整定向数据库回归 `...phase2-pg.log`：订单处理、留厂关闭及 PR703 通过（55.346s）。随后补充混合实物／待入库预订在预览时禁用，避免形成无法完成的待办：`...partial-supply-red.log` → `...phase2-final-targeted.log`。该混合场景仍属后续范围，未宣称可执行。
- 第二批最终版本：`...phase2-final-targeted.log`（7.941s）、`...phase2-final-backend.log` 完整 Go、`...phase2-final-frontend.log` 1746/1746 与构建（1.71s）、`...phase2-changed.log` 均通过。

## 第二批正式发布确认与第三批边界

- 功能 `ae31e4a354e984559e5965d35cf96a94b9b496e5` → develop `f4d5cd6280476743d6bf932f90e8cc895f251fe5` → main `4f7f0557c990e1863e0042966fda0085e1fdbe6f`，均已推送。两环境采用 ERP-only 发布，原小程序构建产物保留。
- 开发备份 `/opt/stacks/erp/orderapp.backup.deploy-20261107173516-f4d5cd628047`；正式备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261107174727-4f7f0557c990`，对应回滚镜像均保留。
- 本机正式部署日志在 SSH 连接中断处结束；不将日志截断当作发布失败或成功。远端容器启动于 2026-11-07T09:57:16Z，镜像 `sha256:f69c817b8583f4a6806f58db9ed69708c82f828e5f9a9d6c551e8d7f4569dcc5`；当前公开静态包 `main-BCuNTg-V.js` 与服务器文件完全一致；处理仓储代码 SHA256 `0bf0fda0042798da8795ee32cdee4b855ca85fe8539fbeb582a4636e50f46ab3` 与远端 main 相同。只读认证、接口、手册及需求标记检查通过。证据 `/tmp/kferp-resolution-20261107-phase2-pre-retry-smoke.json`（核验后无需重试部署）。
- 真实 Chrome 页面只读展开 SO-20261031-0003：关闭剩余履约可选；取消／修订因已完成生产禁用；已完成共享 PP-19 的“保留公司库存”可选。已取消 PP-18、DSR-16/17 保留历史。原因、范围、生产处置、处理时点、外部入账和账期均未选择，未点击预览、保存或提交。订单正常／未作废、2160 元未收款、两行各 24 袋未交付未关闭、处理记录 0。
- 第三批已收独立费用仍为未提交的本地实现：初步验证收款本金、费用冲减和资金分次处置分别记账，原付款记录不改写；已处置收款禁止从旧账单入口再次撤销。`...paid-fee-regression.log` 定向数据库回归通过（49.805s）。独立费用更正、经营统计、失败恢复和完整界面验证尚未完成，不属于第二批正式交付。

## 第三批候选：已收独立费用（未发布）

- 已收费用以原现金收款、费用冲减、已处置资金分别保存；商品款与每笔独立费用分别计算额度。分次退款、客户余额、指定应收抵扣均要求财务明确选择，不调用支付渠道。普通已出账未收费用仍可减免，只有历史结算标记、无可核对收款的费用显示禁用原因。
- 关闭后可选择“金额冲减 → 仅调整费用”，保留商品数量、关闭状态、生产及库存；本范围必须选择先完成费用确认和资金清算。实际运单费用沿运单对应原订单；订单内商品和运费副本不重复处理，负数历史费用调整可保留但不能再减免。
- 单笔退款凭据可按商品与费用来源分别登记，同一来源同一处理单仍禁止重复凭据；HTTP 实际提交与分项确认测试核对两个独立额度。
- 原收款记录未改写，账单与客户账款分别显示原金额、冲减、原收款和资金处置。旧账单不能撤销已用于费用清算的原收款。日志失败回滚、同标识重试、两笔并发退款只完成可用余额一次。
- 经营统计纳入独立费用发生月，冲减按用户选择的记账月；排除订单内副本，未执行处理不进入收入，资金退款／余额／抵扣不重复减少收入。费用写入纳入月结同锁及强锁账期保护；历史快照仍通过已批准的更正路径保存前后证据。
- 失败证据：`...paid-fee-red.log`、`...paid-fee-green-attempt.log`、`...paid-fee-billing-red.log`、`...paid-fee-account-red.log`、`...paid-fee-reversal-option-red.log`、`...fee-only-red.log`、`...fee-only-ui-red.log`、`...fee-revenue-red.log`、`...fee-period-red.log`、`...fee-voucher-red.log`、`...negative-fee-red.log`。费用来源测试初次因样本缺少运单必填字段失败；修正样本后，在未修改 HEAD 归档重现重复副本和遗漏运单费用，证据 `...fee-sources-valid-red.log`。
- 已通过：`...fee-sources-green.log`、`...fee-period-green.log`（5.948s）、`...fee-voucher-green.log`（2.810s）；`...phase3-pg-final.log` 订单处理/留厂/PR703（87.182s）；`...phase3-backend.log` 完整 Go（数据库测试另跑）；`...phase3-frontend.log` 1748/1748 及构建（3.84s）；`...phase3-schema.log` 空库/重复初始化（16.534s）。末尾负数费用提示后，领域/HTTP/财务及页面定向复验记录在 `...phase3-final-rules.log` 与后续 final-ui/build 日志。
- 真实 Chrome 使用浏览器内隔离样本检查：初始原因/金额/去向留空；选择仅调整费用后不要求生产处置；处理时点仍须明确选；部分减免留空提示必须填写金额；原收款与已处置金额分别展示。未连接业务数据库，未提交真实业务。
- 发布回滚保护：镜像声明支持费用资金来源；若已有费用资金待办或费用独立处理记录，不允许自动回退到不能识别这些来源的旧镜像。`...rollback-red.log` → `...rollback-green.log`；ERP-only 保留小程序发布检查 `...erp-only-gate.log` 通过。
- 完整需求仍未完成：停工余料/在制品、生产转供、库存分配/报损、客户货权及实物退回、混合已入库与待入库货源切换仍未开放。不能将本批资金交付当作完整方案完成。


## 第三批集成进度与混合货源候选（2026-11-08）

- 费用功能提交 `4d6370fb5a7976c826ea06394106aeb742c89892`，develop `2b4d132e65af6a169a7e9040e60aff175fa886b6`、main `fceb9cb511717511a2a5db48d6d301c57e775146` 均已推送；main 与已验证 develop 的业务文件完全相同。
- 开发 ERP-only 部署成功：备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108001908-2b4d132e65af`；镜像 `sha256:1ff924be1b9239d2d82dda6b21598bee1fc42bb62e294effc003aaede2dc57d9`；入口 main-BLaaflQV.js 与服务文件一致，代码 SHA256 f8453d53dc8ce97473e3fdd2f7fe59f31ff98fa41ebdef64e7357d7c7bfd86d2 匹配发布提交。未登录 API 401、登录后页面/接口/手册/PR 标记 200。正式部署正在进行，不能将 main 推送当作正式运行完成。
- 后续混合货源候选使用真实留厂购货、待入库代发预订及生产入库转换作为隔离样本；新目标和已有目标两种场景均验证。保留已入库部分须明确选择，原批次及数量展示，待入库数量和目标也不预选；实物状态变化后重新预览必须重新选择。
- 实际库存、原批次货权及冻结规格、两份预留和入库历史纳入检查与版本；旧实物不重建、不释放。目标新预订与旧待入库释放原子提交；原转换数量保留。确认后原销售只关闭尚未交付数量，已交付应收保留。替代货源入库后数量与原预留正确合并，原批次不再次转换。
- RED：`/tmp/kferp-resolution-20261108-mixed-domain-red.log`、`...mixed-pg-red.log`（样本排序字段修正后有效失败）、`...mixed-ui-red.log`、`...mixed-stock-red.log`、`...mixed-preview-red.log`、`...mixed-rollback-red.log`。GREEN：`...mixed-stock-green.log`（4.749s，含目标不足、原库存不足、过期实物、日志失败回滚、重放/并发和后续入库）；`...mixed-broad-pg.log`（59.655s，追加库存守卫前的订单处理/留厂/PR703）；最终回归另见 `...mixed-final-pg.log`。
- 完整 Go：`...mixed-backend.log` 通过，未配置数据库的旧集成按规则跳过。Vue 1749/1749 通过；首次构建发现本次新增 option 闭合标签错误，已修正，`...mixed-build-green.log` 构建通过（1.86s）。真实 Chrome 隔离页面展示已入库 1 袋/待入库 2 袋和原批次，处置/货源/数量初始留空；未选已入库处置时拒绝预览。样本不连接业务数据库。
- 回滚保护增加混合货源版本声明；已有显式实物保留记录时不能回退到无法处理该记录的旧版本。`...mixed-rollback-green.log` 和 `...mixed-erp-only.log` 通过。原小程序包继续保留。
- 本候选尚未发布；在制停工、生产转供、库存重新分配/报损、客户货权及实物退回仍未开放，完整需求未完成。


## 第三批正式发布回执（2026-11-08）

- main `fceb9cb511717511a2a5db48d6d301c57e775146` 已正式运行；备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108003403-fceb9cb51171`；回滚镜像 `kferp-orderapp-rollback:production-20261108003403-fceb9cb51171`。原小程序产物保留。
- 正式镜像 `sha256:f5625e4e4dc702c19e6f4919befdf27b04b7d1f7a6bac83e0bff5e3315116621`；公开页面 main-BLaaflQV.js 与服务文件相同；仓储源码校验 f8453d53dc8ce97473e3fdd2f7fe59f31ff98fa41ebdef64e7357d7c7bfd86d2 与 main 相同。未登录页面/API 401，登录后页面、处理/策略/需求接口、手册 200，容器运行。
- 证据 `/tmp/kferp-resolution-20261108-phase3-{development,production}-smoke.json`、`...phase3-deploy-{development,production}.log`。Chrome 再次只读展开 SO-20261031-0003，关闭可选、取消/修订因已完成生产禁用，共享 PP-19 可保留公司库存，PP-18 和 DSR-16/17 取消记录保留。所有处置仍留空，未点击预览、保存或提交。
- 目标订单正常/未暂停/未取消，2160 元未收款、冲减 0，两行各 24 袋未交付未关闭，处理历史 0。没有以生产业务写入测试费用或取消功能。
- 混合货源候选补充覆盖同一代发的“全已入库预订 + 另一笔待入库预订”：前者也必须明确确认保留，不要求无意义的新货源/数量。补充 RED/GREEN：`...mixed-full-red.log`、`...mixed-full-pg-red.log`、`...mixed-full-ui-red.log` → `...mixed-full-green.log`（5.688s）、`...mixed-full-ui-green.log`（16/16）、`...mixed-full-build.log`（2.11s）。
- 已完成仓库处置后，替代货源可以在财务后续确认之前实际入库；原先要求整个库存永远不变会把后续确认卡住，`...mixed-later-receipt-red.log` 已复现。最终校验改为核对已执行的旧待入库解除、原转换数量及包裹身份，仓库执行前仍要求实物预留版本一致。最终数据库回归见 `...mixed-all-final.log`。该候选尚未合入或部署。

- 混合货源最终候选：`...mixed-all-final.log` 领域/HTTP/订单处理与留厂数据库回归通过（51.389s）；`...mixed-final-backend.log` 完整 Go 通过；`...mixed-final-frontend.log` 1750/1750 与构建（2.30s）通过。第三批正式数据库健康、Caddy/应用运行、费用版本标签为 1，近五分钟无 panic/fatal/ERROR；磁盘剩余 5.5 GB，证据 `...phase3-service-check.log`。

### 混合货源发布回执（2026-11-08）

- 已推送功能 `498bf7ade`，develop `3c7ec9d83ba7720bab87cb24e300e59ec45d214b`，main `ad60158c5eb34aca5eb5a31a9a78fcfd6cc83d7b`。合并前后检查没有其他工作流的新改动。
- ERP-only 开发和正式部署成功，小程序包保留；开发备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108005120-3c7ec9d83ba7`，正式备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108010433-ad60158c5eb3`，正式回滚锚点 `kferp-orderapp-rollback:production-20261108010433-ad60158c5eb3`。
- 开发镜像 `b394f77609cb`，正式镜像 `1699fb6ae4eb`。入口 `main-Dqs9Wi-s.js`，公开页面与服务器静态文件匹配；仓库源码校验 `4656af6bc9302576d2b64cb0bc63abdde9e616be4828971012466ab11113a793` 一致。
- 正式未认证页面/API 401，认证页面/处理接口/手册/配置/需求接口 200。Chrome 再次打开目标订单处理抽屉，正常状态、原数量、共享已完成生产、已取消代发和未选择的业务选项均符合预期。未预览、保存或提交真实订单。
- 只读目标 SO-20261031-0003：2160 元未收款，关闭/退回量 0，处理历史 0。完整方案及现场产品验收仍未完成。
- 日志 `/tmp/kferp-resolution-20261108-phase4-deploy-{development,production}.log`；接口与版本核查 `/tmp/kferp-resolution-20261108-phase4-{development,production}-smoke.json`。

### 公司成品转配候选（2026-11-08）

- 明确选择生产原批次、普通销售目标商品行及整数库存数量；原订单已有预留时，另行明确释放数量。仓库确认与最终订单关闭、原预留释放、目标预留及日志原子保存。全部齐备才标记库存待发货，不自动出库、转仓或退款。
- 核对真实库存、批次、仓库、公司货权、冻结商品规格、目标客户、已有预留、共享需求和本单继续履约量；源批次或目标变化后重新预览，仓库确认不能沿用。跨计划总分配不得超过原单本次关闭数量。
- 普通库存转仓可用量现在扣除销售批次未发货预留；多笔分配先按订单合计，再扣一次出库量，避免重复扣减或遗漏。旧版本回滚遇到新成品分配记录时拒绝不兼容镜像。
- RED：`/tmp/kferp-resolution-20261108-company-allocation-{red,pg-red}.log`；`company-{stock-guards,multiplan,partial,release,target-identity,ready,reapproval,shared-unit,existing-identity,ui,rollback}-red.log`。初次数据库样本使用不存在字段的错误已纠正，PG RED 记录为缺少批次与可用量的业务失败。
- GREEN：最终公司分配与相关订单处理／留厂／PR703 数据库回归 `company-final-pg.log`（125.199s）；转仓数据库回归 `company-stock-transfer-regression.log`（22.550s）；全 Go `company-final-backend.log`；Vue 1751/1751 及构建 3.73s `company-frontend.log`；changed、旧版回滚和 ERP-only 保留包检查均通过。
- 真实数据库用例包含共享已完成生产、部分目标齐备、已有原预留释放、API 显式选择与重复提交、并发仅一次执行、批次／单位／目标身份／库存变更拒绝、日志失败回滚、变化后再次仓库确认、其他客户货权异常拒绝。原批次实物与共享生产/其他订单快照保持不变。
- Chrome 隔离模拟页面观察：分配项最初为空；添加后批次与目标为未选、数量为空；选择批次才列出明确目标，填写后布局正常。页面写入仅模拟，真实 API 写入证据来自隔离 PostgreSQL。临时页面已移出仓库，样本在 `/tmp/kferp-order-resolution-company-preview-20261108.html`。
- 当前能力仅开放可核对的按件公司成品到普通销售需求，不推断箱件／重量转换，不自动接管目标已有生产。停工余料、生产转供、客户货权退回、实物退货及报损仍待完成；PR／DEV 保持进行中。


### 公司成品转配历史版本补查（2026-11-08）

- 首次集成 develop `7cf45b430956f5dfaf31fd19ede158a1eee4740c` 并部署开发；备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108013512-7cf45b430956`，镜像 `1d0917839f07`，页面 `main-DOSGBqsA.js`。开发 API、页面、手册、需求标记与静态文件核查通过；该候选未发布正式环境。
- 补查复现订单和原批次都冻结 V1、BOM 已发布 V2 时分配失败：`company-historical-red.log` 为 `bom_variant_not_current`（此前样本遗漏归档旧版本的唯一键错误已修正）。新增精确冻结订单行与公司批次链，核对仓库、库存单位、货权、品质、实物可用量、已有预留和目标剩余需求；不把历史身份改成当前版本。
- 增加反向数据库用例：错订单行、错批次、错仓库、超需求、克数混入、零数量、留厂/代发/暂停/关闭目标、其他客户货权、原预留占满、目标已齐备、单位不符、质检冻结及实物不足。API 和共享生产保护复用本批实际接口回归；最终结果见后续回执。

- 补查最终隔离回归通过：`company-historical-final-pg.log`，订单处理／留厂／PR703／历史发货 HTTP 为 143.338s，BOM 守卫为 2.806s；完整 Go `company-historical-backend.log` 通过。扩大通配回归时，旧代发目录样本缺少 `product_bom_spec_authorities`；未改动的 develop 发布基线同一用例复现，证据 `company-historical-baseline-fixture.log`，未声称该扩大集合全绿。


### 公司成品转配正式发布回执（2026-11-08）

- 功能提交 `b6362964b`，最新 develop 合并后功能头 `5ef2403a004d7ebeaa2458cff5f784b3a6be6280`，develop `430f061ad677835a004651ad9e422168c70feb73`，main `c0d0650e798da1b8fcc2e3f114d7b8fcde5bf53f` 均已推送；main/develop 业务文件一致，没有混入隔离中的退货实现。
- 追加身份定向回归 `company-historical-final-identity.log`（公司分配与历史场景 53.232s，BOM 守卫 1.134s）；合并后定向 PG 7.593s/2.704s、完整 Go、Vue 构建和 changed 均通过。原成品转配 Vue 1751/1751 保持有效。
- 开发发布连接回执中断，未盲目重试；只读确认源码、运行镜像、静态文件与服务已更新成功。开发备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108015345-430f061ad677`，镜像 `e043d326559b`。构建缓存清理释放约 22 GB，数据库、业务文件和回滚锚点保留。
- 正式 ERP-only 发布成功：备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108124311-c0d0650e798d`；回滚镜像 `kferp-orderapp-rollback:production-20261108124311-c0d0650e798d`；运行镜像 `sha256:e0ca30c6f9ae485053b3c0f2fc4e5e107ad93ac613acf7ba79c9a50bf4c180b6`。小程序原包保留。
- 正式未登录页面/API 401，认证页面、处理接口、策略、需求与手册 200；入口 `main-DOSGBqsA.js` 与服务文件一致。仓储源码 SHA256 `097cc1ea1648640a5109532006773d9cdb02681076e596feece5afb89764cfef`，BOM 守卫 SHA256 `4682c421e774a03d1e3895636041568c32b328a879804b8108ff1fcc4c2327d2` 与发布提交一致；服务正常。
- Chrome 刷新并只读打开目标订单处理抽屉，原因、范围、时点、账期与处置初始留空。保留公司库存可选；原红岩成品批次 99 可核对 24 袋，但当前没有匹配的目标订单，分配选项给出禁用原因；未选择替代目标或提交业务。SO-20261031-0003 仍为正常、2160 元未收款，原数量和已取消代发/共享完成生产保留，处理历史 0。
- 证据：`/tmp/kferp-resolution-20261108-phase5-fixed-development-smoke.json`、`phase5-production-smoke.json`、`phase5-production-service.log`、`phase5-deploy-production.log`。完整订单处理方案及 Van 现场验收仍未完成。


## 2026-11-08 原批次实物退货候选

- 范围：普通销售和客户货权库存代发的真实出库批次退货；明确商品款保留／按量冲减、原批次、数量、货权、仓库和验收状态。留厂购货货权退回、生产停工／转供及报损仍未开放，完整 PR 保持 doing。
- 原始业务保护：保留原发货、包裹、分配、扣减与批次余量；新建退回批次，继承原冻结 BOM 和存储成本。新版订单实际分配旧版批次时，必须存在精确商品行、批次、仓库和数量的原分配证明，不替换版本。
- 仓库与财务分项：仓库填实际开放日期、验收凭据及说明，独立入库；财务确认商品款，必要处置完成后形成应收调整；退款／余额／抵扣继续按原现金来源上限处理。配置审批时，仓库入库前必须先批准。
- TDD RED/GREEN：`/tmp/kferp-resolution-20261108-return-{accounting,rounding,selection,sources,posting,history,ownership,dropship,policy,ui,pending-ui,refresh-api,provenance,rollback,allocated-history,approval}-*.log`。补充成本核对直接通过，未虚报为 RED。
- 隔离数据库：真实原出库证明、公司／客户货权、部分退回与关闭尾差、独立入库及信用、已收／未收、分次退款和客户余额、超额拒绝、重复／并发收货、开放账期、审批、历史版本、源数据过期、日志失败回滚及失败重试、不可改写退回记录、已入库方案刷新、Schema 重复初始化。
- API：真实 HTTP GET／preview／submit／steps／refresh，覆盖必选项缺失、旧预览、收货凭据缺失、已入库继续财务、改选已执行实物拒绝；沿用岗位权限矩阵。证据 `return-refresh-api-green.log`。
- 回归：`return-final-pg.log` 138.195s；库存转仓与身份守卫 `return-stock-identity-pg.log` 4.634s／0.946s；审批补充 `return-approval-green.log` 6.655s。完整 Go（无外部数据库）及 Vue 1753/1753、构建通过；数据库范围之外没有宣称全量数据库测试通过。
- 界面：Chrome 本机隔离 Vue 页验证初始不预选；选原批次后仍须填写数量／货权／仓库／验收；预览保留应收显示冲减 0；模拟仓库收货后已收批次只读，财务仍待办。截图 `/tmp/kferp-resolution-20261108-return-ui.png`。此页面验证不连接任何正式或开发数据库。
- 手册：`docs/OP_MANUAL_ORDER_RESOLUTION.md`“按原出库批次退货”；原有 Vue 手册入口继续读取单一 Markdown 来源。PR／DEV 种子已同步，Van 验收待进行。
- 发布保护：新镜像标记 `io.kferp.order-resolution-return-aware=1`；数据库已有退货处理记录时拒绝回滚到不支持分项退货的旧镜像。回滚脚本 RED/GREEN 已通过。
- 当前状态：候选验证完成，待推送、集成及 ERP 发布。生产业务单据未写入。

## 2026-11-08 第六批发布回执

- 功能 3734e1cbb → develop 42a65173993c5ee579267e74ee58765c121b45e3 → main 471dc50464a7fd504d2372f43d67f378b4f69e3a，均已推送。干净发布目录只包含已验证的原批次退货；客户货权退回仍在功能分支，未夹带发布。
- 开发备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108135341-42a65173993c`，镜像 0fc05ae45bed85869e6188f3a3b483764c2c11a615ebd6806a6f0cd8847f35d6；正式备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108140550-471dc50464a7`，镜像 076d98aa736fadf80ba922c9e9850185d85f4d868d3e0ea1c7969370f74a2185。
- 两环境 ERP-only 发布，保留原小程序产物，未上传或发布小程序。正式回滚镜像 `kferp-orderapp-rollback:production-20261108140550-471dc50464a7`。
- 两环境未认证接口 401，认证 shell/API/手册/PR 200，main-4HPfsK_v.js 与部署文件一致；核心源码及历史身份校验哈希与干净 main 相同；正式 orderapp 运行、数据库健康、无启动错误。
- 正式浏览器只读打开目标订单处理抽屉：关闭剩余履约可选；共享已完成生产和两笔已取消代发明确列出；原因、范围、处置、时点、账期均未预选。未保存草稿、预览或提交真实处理单。
- 目标 SO-20261031-0003 保持正常、未暂停、2160 元未收款、冲减 0、处理历史 0。此回执为部署与只读运行验证，不替代 Van 的业务验收。
- 证据 `/tmp/kferp-resolution-20261108-return-{deploy-development,deploy-production,development-service,production-service}.log` 及对应 `*-smoke.json`。

## 2026-11-08 客户货权退回候选验证

- 仅原留厂购货实际收货记录可退；校验订单行、客户货权、冻结规格、原批次/仓库/流水、可用实物及预留。共享批次按原收货权益限制数量，普通转仓仍不能绕过客户货权保护。
- 用户明确选择货权退回、数量、公司接收仓及验收处置。仓库按开放账期日期/凭据过账：原客户批次和库存减少、新公司批次和库存增加；保留原入库、共享生产与原成本。商品款和资金分别确认。
- 原已入库量保留，货权退回单列；净购货/净入库扣除已退回，待入库承诺不因退回重新产生。全部已入库货权退回后，原权益显示已释放历史；已有仓库事实时 API 重新预览和刷新不重复移动库存，不能修改已执行选项。
- RED → GREEN：原收货来源缺失、无明确货权选择、未开放执行、历史 BOM 身份拒绝、无过账记录却修改退回量、未单列已退回读模型、可删除出库流水、普通库存接口伪造专用来源、旧镜像回滚、页面缺少禁用原因、全部退回仍要求处置。日志 `/tmp/kferp-resolution-20261108-ownership-{sources,domain,posting,guards,history,ledger,source,rollback,ui,released}-*.log`。
- 隔离 PostgreSQL 订单处理/留厂/PR703/历史发货回归通过（191.284s）；最终货权相关测试通过（25.002s），含实际 HTTP GET/preview/submit/steps/refresh、重复提交、并发仅一次、共享批次被预留、来源改变、缺失出库凭据、仓库变更、日志失败回滚、审批前阻断、关闭账期拒绝后重试及 schema 重复初始化。库存/历史规格相关包 7.261s/0.938s 通过。
- 完整无数据库后端验证通过；全前端首次 1754/1755，旧断言需包含新增的已退回数；修正该断言后定向 23/23 及 Vite 构建通过，合并后再运行完整前端。未将无数据库后端验证称为完整数据库验证。
- 浏览器隔离样本通过：业务选项均初始留空，选择原收货仍不预填数量/货权/仓库；明确退 2 袋、公司成品仓、待检隔离、按原款冲减，预览列出 60 元及库存/财务分项。截图 `/tmp/kferp-resolution-20261108-ownership-ui.png`；使用 mock 页面，无真实业务写入。
- 源手册 `orderapp-remote/docs/OP_MANUAL_ORDER_RESOLUTION.md` 已同步，ERP 处理表单与库存/代发录单说明同步。需求 PR-749 与 DEV-PR749-* 继续 doing；停工余料、生产转供、报损仍未交付，Van 验收待进行。

- 合并最新 origin/develop 后：完整无数据库 Go、前端 1755/1755 与 Vite 构建通过；原出库退货/客户货权退回/库存代发/不可改写记录的隔离 PostgreSQL 回归 72.449s 通过，changed、diff-check 与回滚保护通过。证据 `/tmp/kferp-resolution-20261108-ownership-merged-{backend,frontend,build,pg}.log`。

## 第七批正式回执（2026-11-08）

- 功能 57fd6c209；最终候选 770dd8767；develop a65c5d890ff1193cbfe6214b3233e63093cccc3a；main dfedfd86175901ff556b6f2d248485af207417b0，全部已推送。
- 开发：备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108142955-a65c5d890ff1`，镜像 3c4b26ab846d，源码与页面核对通过。
- 正式：备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108144236-dfedfd861759`，回滚镜像 `kferp-orderapp-rollback:production-20261108144236-dfedfd861759`；运行镜像 e74ced3a9491c835196c53bee22cc563cc049e82d87aa94925486d6987247ad3，入口 main-DZrtLBCZ.js。
- 证据 `/tmp/kferp-resolution-20261108-ownership-deploy-{development,production}.log`、`ownership-{development,production}-smoke.json`、`ownership-production-service.log`。认证 API 401/200、页面/手册/PR/配置 200、源文件哈希、静态资源一致性通过；数据库健康，ERP 服务运行。
- Chrome 刷新正式订单列表并只读打开目标抽屉：SO-20261031-0003 正常，2160 元未收，历史 0；共享 PP19 完成、PP18/两笔代发取消；可关闭剩余履约，全部原因/数量/处置/账期均未选。未提交业务预览、保存或变更。保留原小程序包，未构建或发布小程序。

## 第八批停工候选（尚未发布）

- RED：没有逐工单余料选项、缺少仓库确认、预览无实际余料行；旧确认沿用、已停工计划新建工单绕过、旧版本回滚均先复现。
- 当前隔离 GREEN：保留扣料/在制品/批次/成本；明确保留或只释放未耗用预留；生产与仓库分别确认；数量/货权/预留变化拒绝，日志失败原子回滚，重复及并发只执行一次；刷新变化后的方案重新要求两个岗位确认；重新开启销售不恢复停工工单。
- 补充 RED → GREEN：停工计划遮住重开后的剩余需求；重复初始化尝试回填停工历史；其他工单通过来源关联继续消耗已停工在制品/余料；计重产出成本误显示为零。分别修复需求筛选、跳过停工历史回填、校验新旧两端来源、按原单位成本计算，定向回归通过。
- 完整无数据库 Go 验证通过；全前端 1757/1757、Vite 构建通过；隔离 PostgreSQL 订单处理/留厂/PR703/历史发货 177.077s、生产需求 0.955s 通过；changed 与旧版本回滚保护通过。证据 `/tmp/kferp-resolution-20261108-stop-release-{backend,frontend,pg}.log`，最终停工定向 `/tmp/kferp-resolution-20261108-stop-final-target.log`。未声称完整 PostgreSQL 全库回归。
- 浏览器 mock 样本通过：四种业务处置均不预选，未选时拒绝预览；明确原位保留余料/在制品、释放未耗用预留、保留原产出后，显示订单冲减与生产/仓库两个确认；仓库确认仍需明确选择。截图 `/tmp/kferp-resolution-20261108-stop-ui.png`；真实订单未写入。工单跳转及返回来源用实际组件运行测试验证。
- 未完成：开发/正式发布；指定需求转供及批准报损仍待实施。完整 PR/DEV 保持进行中。

- 合并最新 origin/develop（无源码差异）后，完整无数据库 Go 再次通过；停工/API/重开需求隔离数据库 14.872s / 1.255s 通过，changed 通过。前端相对已通过的 1757 项和构建无差异。

## 第八批停工正式回执（2026-11-08）

- 候选 90d92003a，develop f9b9a06a7aaa51893ec6cf62ed3d0329b1aaed4d，main 5d52568126ffb84c5ee2f9944f56790f2d7b9528 均已推送；发布源来自干净 main，未带入隔离中的报损改动。
- 开发备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108151607-f9b9a06a7aaa`，镜像 11efbf996ac132dd8fe4cbc2f3b0ef07f3d4f3c06dbfa2c5667cc7616ed05a0a。
- 首次正式构建中断，先只读确认旧服务和发布锁，再从同一 main 重试成功。正式备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108153839-5d52568126ff`，回滚镜像 `kferp-orderapp-rollback:production-20261108153839-5d52568126ff`，运行镜像 379b8b6fbe42e906e91144443e429ce29cca96b4a1d51959e0af37631bb34bd6。
- 两环境 ERP-only 发布保留原小程序产物。静态入口 main-BSVuaiOb.js，认证前 API 401、认证 shell/API/手册/策略/PR 200；核心源码 SHA256 与干净 main 相同，正式服务运行且数据库健康。
- Chrome 只读打开当前正式订单抽屉，确认关闭剩余履约可选、取消须保留已完成共享生产；所有原因、范围、处置及账期留空。SO-20261031-0003 仍正常、未暂停、2160 元未收、冲减 0、历史 0。未提交业务预览、草稿、库存或订单操作。
- 证据 `/tmp/kferp-resolution-20261108-stop-deploy-{development,production-retry}.log`、`stop-{development,production}-smoke.json`、`stop-production-service.log`。完整需求和现场业务验收仍未完成。

## 第九批公司成品批准报损候选

- 明确原批次、数量、原因和成本方式；共享生产保护其他需求/预留；销售提交、财务批准、仓库确认分别按权限，最终扣库、原成本单列／成对重分类、订单调整与日志原子提交。所有钱货选项留空，管理员可停用成本方式，不能跳过事实校验。
- 原生产成本已被现有经营报表计入，因此“保留原成本”不再加费用；“转列损失”在选定账期形成主成本 -30/费用 +30 的成对调整，原 10 月成本 50 保留、11 月总成本净变化 0。以真实财务服务 DraftReport 验证。报损不能早于原批次和原成本日期。
- RED → GREEN：缺少原成本证明、缺少明确选项、历史 BOM 报损拒绝、普通库存接口伪造报损来源、旧财务批准可重复沿用、原批次成本可改写、处理记录没有实际报损凭据、管理员配置丢失报损选项、质检拒收批次不能批准报损、同月提前记报损、旧版本回滚未保护。证据 `/tmp/kferp-resolution-20261108-scrap-{domain,posting,history,source,refresh,immutable,evidence,policy,held,date,rollback}-red.log`（个别 RED 文件名以此前记录为准）。
- 隔离 PostgreSQL 定向最终 16.389s 通过：实际 HTTP 各岗位 403/200、两岗位确认、重复/并发一次、共享其他订单保留、实物不足、来源/成本/预留过期、账期关闭、日志失败全回滚、原成本和调整不可改写、历史 BOM、冻结批次余量仍冻结、重新初始化、重开不恢复损失，含财务报表核对。日志 `scrap-final-target2.log`；原批次持久凭证包含单位/数量/成本/日期/批次。
- 完整无数据库 Go 通过；全前端 1758/1758、Vite 构建通过；changed、diff-check、回滚保护通过。扩大数据库回归中订单处理等用例完成，唯一失败为已有代发目录样本缺 `product_bom_spec_authorities`，在干净 main 同一测试复现（`scrap-pg.log` / `scrap-baseline-fixture.log`），不声称完整数据库全绿；生产 1.442s、库存转仓 5.503s、规格守卫 0.743s 通过。
- Chrome mock 页面完整操作通过：初始无选项，漏选拒绝；明确批次/3 袋/成本方式/原因/账期后预览财务和仓库待办，两岗位各自再选并填凭据，最终显示已报损 FP-TEST-301、3 袋、原成本 30 元及日期；未使用真实订单或库存测试。
- 操作手册和 PR/DEV 进度同步；候选待集成和发布，生产转供仍未完成，完整 PR 继续 doing。

- 合并最新 origin/develop 后没有额外业务文件变化；完整无数据库 Go、changed 通过；报损/成品分配/停工隔离数据库回归 48.986s 通过。前端相对已通过的 1758 项及构建无变化。证据 `scrap-merged-{backend,changed,pg}.log`。


### 第九批正式发布回执

- 功能 c961fe10b，最终验证 cf59da51e，develop bc46c69cbae006851cf01aeacb9401ca4bdd5732，main f9cb1b3ee509d08c385a6e5d99e619a4bdbc7324 已推送；仅发布已验证报损，未带入生产转供候选。
- 开发备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108155706-bc46c69cbae0`，镜像 d9c7f061186c4648664f665d0a5bd8ba4d9d4341da16bcf7a4550ae195be37d6；正式备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108161117-f9cb1b3ee509`，回滚镜像 `kferp-orderapp-rollback:production-20261108161117-f9cb1b3ee509`，运行镜像 2c98ad5274a667fb067b4a11c43ab93c150f00242378141cb8ccc5ee8a5d8d53。
- 两环境 ERP-only，保留原小程序包。正式服务运行、Postgres healthy，未登录 401、登录页面/接口/手册/PR 200；公开 main-cP5bzBb9.js 与部署文件一致，核心源码散列与干净 main 一致。
- Chrome 刷新正式订单列表并打开目标订单处理，初始业务选项仍留空；关闭剩余履约可用、已完成共享生产及两个已取消代发列出。只读 API：SO-20261031-0003 正常、2160 元未收款、冲减 0、处理历史 0；没有提交预览、草稿、处置或库存移动。
- 证据 `/tmp/kferp-resolution-20261108-scrap-{deploy-development,deploy-production,production-service}.log`、`scrap-{development,production}-smoke.json`。


## 第十批未完成生产转供候选

- 范围：明确非共享、单工单、公司货权、无成品产出生产的目标商品行及完整数量。要求原订单关闭量足够、目标已接单且冻结商品/规格/单位/仓库与完整剩余数量匹配；客户专属标识、加工说明、已有生产与上下游依赖须先核对，不自动拆分。普通客户名称快照保留，不误判为专属要求。
- 原工单、工艺/配方、投料、余料、在制品、预留及成本保留；独立记录原单/目标/数量及完整快照，仅切换有效需求关联。生产负责人明确确认并提供依据；原单重开不撤销转供。目标编号、客户、规格、数量、仓库及专属加工信息不可直接改写；订单日期仍可修改。
- RED → GREEN：原能力禁用、缺少目标数量、目标剩余数量不完整、原目标可改写、缺少实际凭据、旧版本回滚无保护、重复追加目标生产、转供冻结配方/工艺可改写、普通客户快照误挡、未审核或专属目标误接收。证据 `/tmp/kferp-resolution-20261108-reassign-{red,domain-red,target-remainder-red,guard-red,proof-red,rollback-red,duplicate-red,frozen-red,target-identity-red}.log`。
- 真实 PostgreSQL HTTP：GET/preview/submit/step，销售与财务不能代替生产岗位确认，缺少凭据拒绝，重复返回原结果，历史返回原单→目标单凭据；同步操作日志。真实工位接口“开工→转供→完工”通过：原开工投入消耗一次、完工日志归目标单、目标进入生产完成、成品库存 100 袋、原成本记录一笔；未使用生产业务数据。
- 过期目标数量/仓库/状态、新增目标生产、WIP 变化与审计失败均整体回滚；重复/并发确认仅一笔转供，重新初始化保持正常，旧审批刷新后 superseded，源单重开不会夺回生产；目标已承接的同一需求不能重复追加生产。最终身份/历史目标回归 17.334s、实际完工接口 2.254s 通过（`reassign-identity.log`、`reassign-final-production.log`）。
- 完整无数据库 Go、Vue 1759/1759、Vite build、changed、回滚保护通过；订单处理全流程 PostgreSQL 回归 212.939s，生产接口 8.132s 通过（最后冻结快照与目标确认细化后将在集成前重新运行相关回归）。
- Chrome 隔离页面从全部空选项到漏选拒绝，再明确目标/3袋/账期、生产岗位确认凭据，最终显示 SO-TEST-SOURCE → SO-TEST-TARGET 及 3 袋；截图 `/tmp/kferp-resolution-20261108-reassign-browser.png`。未连接业务数据库。
- 已更新源手册与 Vue 表单说明、PR/DEV；候选待合并与发布，产品验收仍由 Van 完成。复杂共享/已部分产出或客户专属生产不自动改供；页面给出先拆分核对的限制。

- 集成前最终检查：合并最新 origin/develop 后，完整无数据库 Go、changed、兼容回滚通过；订单处理/留厂关闭 PostgreSQL 全回归 189.244s，真实生产接口 3.093s 通过。历史 BOM 冻结转供另测 1.741s 通过；前端代码相对已通过的 1759 项及构建无变化。证据 `reassign-merged-{backend,changed,rollback,pg}.log` 与 `reassign-historical.log`。最后冻结保护的启动问题来自旧测试样本产出字段未回填，按正常生产初始化完善样本后重启、完工与历史保护均通过，未放宽生产守卫。


### 第十批开发与正式发布回执

- 验证提交 54c7be4542ec56f1f4b69c080121ebe2d254f66c 已推送；干净 develop 03e5e0091b3e5a79ee9616b7bf6b9ff8e7767d1f、main 2657ca8e2e9f1b15762b7c8155315a39c7efc1a4 已推送并完成两环境发布；发布前确认源与已验证分支一致，没有带入其他改动。
- 开发备份 `/opt/stacks/erp/orderapp.backup.deploy-20261108163305-03e5e0091b3e`，回滚镜像 `kferp-orderapp-rollback:development-20261108163305-03e5e0091b3e`，运行镜像 830b21b373f821fcbbb36bbe8d1fa818c3a22213a283bc394994ce3286301956。
- 正式备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20261108164456-2657ca8e2e9f`，回滚镜像 `kferp-orderapp-rollback:production-20261108164456-2657ca8e2e9f`，运行镜像 6b32575caf79348bb6391017638736132681e431c16c2f1669de3dda13a6c32c。两次 ERP-only 均保留原小程序包。
- 正式服务运行、Postgres healthy；匿名页面及 API 401，登录页面/订单处理/策略/手册/PR 200；main-YdwMQe4H.js 与服务器文件一致，repository/生产转供/停工/报损/退货/货权/库存及身份守卫源码散列与干净 main 一致。开发 API 认证及同等功能核对通过。
- Chrome 刷新正式订单列表并只读打开目标处理抽屉，观察正常状态与所有业务选项初始为空。只读 API 与第九批发布后相比，目标订单状态、数量、已关闭量、已退回量及账款完全一致，历史仍为 0；未 POST 真实预览/草稿/处置，无真实领料、取消、退款或库存移动。
- 证据 `/tmp/kferp-resolution-20261108-reassign-{deploy-development,deploy-production,production-service}.log`、`reassign-{development,production}-smoke.json`；隔离与正式页面截图保存在 `/Users/yiiiple-work/.codex/visualizations/2026/11/04/01a1a5fe-9461-76a2-958f-8b00b47606d5/order-resolution-acceptance/`。
- 本回执在部署后补入仓库，仅更新验收和需求进度文档，不重新构建已核对的服务；生产运行版本仍为 2657ca8e2。实施与发布已完成，Van 现场业务验收待进行；复杂共享、部分产出或客户专属工单须先拆分核对，未验证场景不自动处置。
