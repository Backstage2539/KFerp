# PR-736 旧规格入库与单张包装操作

2026-10-26，承接 PR-735 发布后的现场反馈。Van 最终业务验收待完成。

- 生产只读核对：工单 `WO-PP-0000000012-0000000060` 冻结 `product 7 / spec 20 / variant 559 / BOM 409`，各项相互匹配；当前 BOM 为 553。该工单旧完工记录为14袋，本次不修改实际产出。
- `包装 / 曜石2.0`（工单末尾58，工序卡16）的 `product 58 / spec 68 / variant 1166 / BOM 552` 也为匹配的历史冻结身份。
- PostgreSQL 错误语句证明，两条路径均在入库后转换代发预订、写入 `order_stock_batch_allocations` 时被当前版本校验阻断。原有入库保护并未覆盖这一步；不是本次合并丢失旧工单规格。前一张工单在18:49已有同接口失败记录。
- 工位页面源码在开发、生产、远端 develop 中相同；相关工作区没有未提交的页面改动。单张已开工旧包装卡未匹配简化条件，且只有一张来源卡时不会显示包装组，因而回退到旧动作。页面渲染回归已复现。

## 修复和验证

- [x] 包装、分装、装盒、装袋的单卡统一“工序完成 / 查看工单”；保留服务端完成条件校验，补料使用对应计划备料入口。
- [x] 两类批次分配只有在精确订单行和实际批次都匹配商品、规格、历史版本，且批次编号、规格克数和货主匹配时，才沿用归档版本。新订单及无来源、错来源仍受当前版本规则保护。
- [x] 隔离 PostgreSQL/HTTP RED：旧完工直接入库与单张旧包装完成均重现 `bom_variant_not_current`；GREEN 两条路径均成功，保留原版本。
- [x] 重试原请求不重复入库、耗料或分配。第二张分配表故障使库存、耗料、成本、权益与第一张分配表一起回滚，移除故障后原请求可重试。直接入库不再次耗料。
- [x] 错订单、订单行、批次、批次编号、规格克数及货主共12种分配负例，在两条 API 场景均拒绝；入库成本为非零并落在对应成本表与库存单。
- [x] 页面实际 Vue 渲染测试 RED/GREEN，现行数据库规格保护及相邻入库回归通过。
- [x] 完整 Go（默认模式）、Vue 1,630/1,630、Vite 和 changed 检查通过；显式 PostgreSQL 相邻入库、现行规格保护及留厂预订转实物库存回归通过。
- [x] 首次生产发布后，针对四条仍失败的新版预留记录完成第二轮 PostgreSQL/API RED/GREEN：历史来源批次与来源留厂权益保持 V1，目标预留订单行采用同规格 V2。仅当有效收货、权益、来源工单/订单、未转换预留、目标订单行、物理批次和货主全部匹配时放行；现行规格保护和12类错配负例继续拒绝。
- [x] 最终候选 `78560416ddb53ec5576ed18a201d51f02a8dd9ab` 合入 develop 并部署开发环境。Vue 1,630、Vite、miniapp 432、类型检查、24页构建、全量 Go 测试和镜像内 Go 测试通过；开发登录页 HTTP 200、数据库健康、DEV-PR736-03 行已创建。
- [x] 首次正式服务器构建的 Vue 1,630、小程序432、类型检查/24页构建、Go及镜像内Go验证通过；首次生产部署退出0，运行核验通过。
- [ ] Van 现场业务验收。

## 正式发布证据

- 功能提交 `1acd44526dba9b6f0dae4f95813a821d9daec67f` 已推送并合入 develop；保留 main 既有保护后合入 `fc96f6720f67a474e2ba6893e0f27ab5163534dd`。2026-10-26 21:58 生产运行核验确认此版本，后续仅补发布文档，不再部署应用。
- 运行镜像 `sha256:a4190fab62059f5bf52e9dd22bfa347f3ed40a609a492bcad11d505468846cdc`；重启计数0，PostgreSQL健康且未重启，磁盘余8.9GB。
- 数据库函数已包含两类批次分配的精确冻结身份及货主保护；函数哈希 `94f4cd814b6925abcfa7d6bc02074e49463ebb106d089ac2b4fe1187dd27def8`。四个关键源文件与最终候选哈希一致；工位、计划及订单脚本的服务器/容器/公网哈希一致。
- 未登录页面/API返回401，登录页200；认证后工位/计划页面及API、生产/销售手册200，订单复制包含客户名称和付款方式。生产包装台只读页面确认末尾54、58、61三张旧任务均显示“工序完成 / 查看工单”，不再显示物料操作、报异常或呼叫补料；截图 `production-packaging-after-live-fix.png`。
- 21:47:01至21:59:12的35表快照，除访问日志增加69条GET外，其他34表行数和内容哈希一致；该区间无POST或其他写入操作日志。PR-735/736均为review，关联开发项为done；Van验收不自动标记完成。
- 回滚源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261026214738-fc96f6720f67`；回滚镜像 `kferp-orderapp-rollback:production-20261026214738-fc96f6720f67`，身份为前一版 `sha256:63199fe1d0487a3038e89b4e1e7d63361616d2d6b02360de7f19ccadf7a7676e`，已确认存在。
- 首次发布时未部署开发环境；各轮均未覆盖本地微信构建目录，也未上传、审核或发布微信小程序。两张真实工单尚未由本会话提交完成/入库；隔离API成功和生产规则生效不替代现场业务确认。

## 第二补丁：新版本预留与历史批次

- 生产日志中四个失败请求的错误均发生在向 `order_stock_batch_allocations` 写入分配时。旧来源冻结 variant 为 `559/558/1166/1167`，目标预留订单行已随 BOM 重发变为 `1185/1186/1205/1206`；商品、实际规格、克数和货主相同，来源收货量与预留转换量仍为0。
- 本次放行沿精确来源关系校验历史批次：订单/订单行/请求、仍开放的留厂预留、权益收货、来源工单与订单、实际批次编号、规格克数和货主必须组成完整关联；不放宽普通新订单的当前规格校验。
- 最新生产只读核对确认四张目标工单仍为原来的待完成或部分完成状态，22:21:46 CST 后无 POST/PUT/PATCH/DELETE 请求。另有工序卡12在22:03:56由现场成功完成，审计记录与之前快照差异相符；本次发布没有完成或入库用户的目标工单。
- 生产候选合入、部署和运行证据待补。微信小程序上传/审核/发布及 Van 现场验收均尚未完成。

运行证据：`production-live-followup-deploy.log`、`production-live-followup-smoke.json`、`production-live-followup-guard.json`、`production-live-followup-snapshot-comparison.json`、`production-live-followup-operation-interval.json`、`production-live-followup-rollback.txt`。

证据目录：`/Users/yiiiple-work/.codex/worktrees/production-followup-release/evidence/`，包括 `production-followup-issues-readonly.json`、两个 `production-*-error-context.txt`、`live-packaging-ui-{red,green}.log`、`live-frozen-receipt-{red,green,regression}.log`、`live-identity-guard-regression.log`。

生产业务记录仅进行只读诊断；未替用户点击确认完成或入库，未修改现场工单、历史数量或库存。原发布记录中的其他既有失败边界不因此关闭。
