# PR-718 客户履约运营台重构

状态：技术验证、开发环境及生产交付完成；Van 业务验收待确认。分支 `codex/fulfillment-operations-redesign`，基线 `origin/develop@0ebb0a0a`；后续生产授权及交付记录见文末。

## 交付范围

- 概览待办优先，库存/订单/代发/费用稳定入口；设置独立抽屉，延续商品创建器白底、浅灰、绿色主操作、紧凑表格与右侧详情。
- 服务端待办及分页使用共享筛选；订单与代发共用原单据，内部历史读取与新增能力校验分离；外部客户权限保持原范围。
- 成品现货/占用/可立即发货/已购待入库分列，复用客户资产与批次口径。费用阶段不把费用明细和其结算单累加。
- 两场景共享默认、字段继承、明确零/false客户例外、恢复默认、并发版本及操作日志。仓库/开启状态/价格授权独立，存量仅显式选择后继承，已有订单快照不变。

## 验证证据

原始输出与本地截图保存在 工作树同级 `evidence/fulfillment-operations/`（演示资料，不进入生产）。

- RED：`defaults-red.log` 缺少默认解析接口；`operations-red.log` 缺少服务器运营数据/筛选；`ui-red.log` 缺少运营辅助函数；`ui-dom-red.log` 实际页面缺少五模块导航。
- GREEN：新默认解析测试通过；隔离 PostgreSQL 的共享默认/原子设置/历史读取与131单分页验证通过；针对性前端23项通过。
- 全量 Go `scripts/verify_kferp.sh backend` 通过（未配置全局集成数据库；实际数据库在上述针对性测试中单独提供）。
- 完整 Vue 测试 1517/1517、Vite 生产构建通过；最终库存分页/焦点调整后针对性页面交互5项通过。远端发布会对最终提交重跑全部门禁。
- 实际 HTTP 默认配置并发保存一成功一冲突；旧设置、明确零值、缺记录不开启、停用能力历史、131单分页、客户隔离、费用阶段无重复、运费解析、旧入口继承冲突、审计及历史费用冻结覆盖。旧入口缺少继承保护的 RED 见 `legacy-policy-red.log`，修复后 GREEN 见 `backend-final-db.log`。
- 本地实际 Vue 组件布局覆盖 1280×720 桌面概览/库存/代发/费用/设置、620px 窄窗口；使用演示数据，未提交业务写入。原始截图同目录，设计核验记录见 `design-qa.md`。
- 单次测试/构建/本地演示不代表部署或业务验收。

## 数据与兼容

新增默认规则表为可重复初始化的单例，不覆盖任何客户记录。旧设置没有继承字段时继续使用自身规则；缺记录或未启用的客户不会自动开启或变免费。默认更新仅影响显式继承字段与后续报价；客户归属、能力、仓库、价格发布版本、订单付款和历史费用不迁移。

手册：`docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`，沿用 Vue `customerFulfillmentManual` 动态手册入口；不创建根目录手册副本。

## 发布与验收

2026-10-23 开发环境交付完成，业务验收待 Van 确认。

- 功能分支 `codex/fulfillment-operations-redesign`，已推送功能提交 `730631b33d4c93a55219749c77ddc801d8a43ca3`；GitHub PR #250 合入 develop，部署提交 `13a185db1af16a881f53697fc57e7d2832a14ba7`。
- 使用独立 `deploy-checkout` 从最新 origin/develop 发布，保留原共享 develop 工作目录的其他未提交修改。命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`，退出0。
- 服务器顺序完成 Vue1517、miniapp426、类型检查、Vue/小程序构建、24页面验证、主机与镜像内全量 Go 测试。未导出/上传微信开发包。
- 仅重启 `erp_orderapp`；运行中、重启计数0，`erp_postgres` healthy；镜像 `sha256:81c21498c35c995dd829bf39c141d6b5e98fec1a01ab743cf7c2e9cdf55041b6`。
- 公网开发登录页200；匿名根路径303、受保护默认配置API401；认证工作台、默认配置GET、手册均200。PR-718与DEV-718-01至04均已进入review，未代替Van验收。
- 首次默认读取为V0、两个场景，实际商业费率尚待管理员维护；本次不修改任何客户收费配置。
- 服务器源文件、运行容器与公网资源 `CustomerFulfillmentView-Dn2Mov5X.js` 的 SHA256 相同：`7802917e4c9400dc07d13376f0564e8795b0e90f6b248e1bb03514921574520e`；运行包RELEASE_INFO指向上述部署提交。
- 源码回滚点 `/opt/stacks/erp/orderapp.backup.deploy-20261023023741-13a185db1af1`；回滚镜像 `kferp-orderapp-rollback:development-20261023023741-13a185db1af1`。
- 发布前磁盘余量2.8G，编译后清理可重新生成的主机Go缓存及未使用构建缓存；未清理数据库、业务文件、源备份或带标签的回滚镜像。
- 原始交付日志：`deploy-development.log`、`runtime-smoke.log`、`runtime-container.log`，见工作树同级evidence目录。

入口：https://dev.qacoohee.com/app/vue-shell?view=customerFulfillment 。生产发布、生产业务数据修改、小程序微信上传与Van业务验收均未执行。本次后续收据提交仅更新交付记录，开发环境应用仍运行上述部署提交。


## 生产发布授权与集成

2026-10-23 Van 明确要求“合入main，部署生产环境”。发布分支 `codex/fulfillment-operations-production` 从 `main@d5edc418` 合入已验证的 `develop@470018f9`；main 独有的历史数据保护及生产交付记录保留。合并无冲突，业务改动限定于上述46文件。合并后重新运行相关隔离数据库/API及Vue回归，并由生产发布脚本重跑全量门禁。当前为发布前记录，实际部署版本与回滚点将在完成后补录。

生产候选合并后验证：隔离 PostgreSQL/API 三项通过（6.4秒，无跳过）；生产 BOM 启动保护测试通过；相关 Go 应用/接口/订单/支持包通过；Vue 交互与样式22项通过；changed及合并暂存区检查通过。日志为 `production-integration-{db,go,vue}.log` 和 `production-bom-guard.log`。


## 生产交付结果（2026-10-23）

- 发布候选 `54020ecf334f345ef9472e0f63ac7b6bc45c9bfe` 已推送；GitHub PR #252 合入 main，运行提交 `5f218156324b183e2e3ba315608304ebcb79dc84`。独立 deploy-checkout 的 main 与 origin/main 完全一致后发布。
- `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production` 退出0。服务器 Vue1517、miniapp426、类型检查、两端构建、24页面验证、主机及镜像全量 Go 门禁全部通过。GitHub connector 的候选 PR workflow 查询返回空列表；不把该结果作为 CI 通过证据。
- 仅生产应用切换；`erp_prod_orderapp` running/restarts0，`erp_prod_postgres` healthy。镜像 `sha256:8e941ffe70ba47997ff41651a23d3fba72d94b49d4817cf20815a9226ff74b36`。
- 公网登录页200，匿名根路径与默认配置API均401；当前运行凭据认证后工作台、共享默认、PR/DEV、手册均200。首次检查脚本沿用静态.env认证返回401，改为运行容器的当前认证信息后通过；未修改认证配置。凭据未写入输出或交付文件。
- 只读核对用户指定客户：概览正常，待备货、待发货、待确认、待收款、缺收件信息五类服务端统计均与对应订单列表总数一致。检查仅发GET，未提交客户设置、订单、库存或费用写入；原始响应中的业务数据未保存在交付日志。
- 全局默认为revision0、两场景；本次未指定商业费率。存量客户配置与历史费用按原规则保留。PR-718与四个DEV均为review，Van业务验收待确认。
- 运行包 RELEASE_INFO 指向 `5f218156324b183e2e3ba315608304ebcb79dc84`。服务器源、容器、公网 `CustomerFulfillmentView-Dn2Mov5X.js` 的 SHA256 一致：`7802917e4c9400dc07d13376f0564e8795b0e90f6b248e1bb03514921574520e`。
- 源码回滚点 `/opt/stacks/erp-production/orderapp.backup.deploy-20261023095037-5f218156324b`；回滚镜像 `kferp-orderapp-rollback:production-20261023095037-5f218156324b`（旧镜像 `sha256:abbbb5b5d97455e7b47482f777d05e0b3385e8540e567a6437c5da45afb30b57`）。均已核对存在。
- 发布前及完成后仅清理可重新生成的Docker构建、npm、Go、dnf缓存；最终磁盘余量约2.4GB。生产/开发应用均running/restarts0，数据库、业务文件、源码备份及回滚镜像保留。
- 原始证据：同级 `evidence/fulfillment-operations/deploy-production.log`、`production-integration-{db,go,vue}.log`、`production-bom-guard.log`、`runtime-production-smoke.log`、`runtime-production-container.log`、`production-cache-cleanup.log`。

生产入口：https://erp.qacoohee.com/app/vue-shell?view=customerFulfillment 。本次没有微信上传/审核/发布；技术发布与Van业务验收分开记录。发布后的收据提交仅更新本文件与ACTIVE_REQUIREMENTS，不改变上述运行版本。
