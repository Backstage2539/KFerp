# PR-735 生产补充发布验收

日期：2026-10-26
基线：main `001c65b0`；功能来源：develop `692f09b8`。

## 范围

- 修复同一烘焙工序分次完成时，本次增量投料再次减历史耗料的问题。
- 整合 PR-732 生产计划分组备料、运费确认上下文、PR-734 复制订单客户与付款方式。
- 修复隔离回归发现的既有散装重量入库缺陷：无包装规格时，库存、入库单明细、批次及生产日志统一记录实际克数。
- 补齐已发布功能缺失的六个前端回归测试和开发最终点验记录；保留 main 的 BOM 回填保护及历史验收状态。

## 已完成的自动化验证

- [x] 烘焙增量耗料 TDD。真实隔离 PostgreSQL/API 的 RED 复现：同一卡 1+5kg 投料只扣 5,000g，材料及工单成本只记 500；3+3kg 第二次被“本批烘焙用料为零”拒绝。修复后两种情况均累计扣料 6,000g、入库 5,000g，成本 600；历史部分完成后按当前页面省略投料字段收尾也通过。
- [x] 新增单元测试保留克级取整。API 同时核对实物 WIP、预留耗料、成品库存、库存单、工单/事件成本和业务审计数量；原请求重试不重复记账，注入审计失败后全部回滚，移除故障后同请求可成功重试。
- [x] 相邻烘焙配方、BOM 首次耗料前刷新、无预留旧单、包装分次完成、库存货主/仓库/质检隔离及失败重试回归通过。
- [x] 前端定向回归 RED 记录为 84 通过 / 8 失败，整合后 GREEN 为 92/92；最终物料行稳定 key 修复后 Vue 全量测试 1,629/1,629 通过、无跳过。完整 Go 测试通过；这是默认无外部数据库模式，不能代替所有显式 PostgreSQL 集成开关。
- [x] 现行产品规格身份契约定向测试 5 项通过、无跳过：清理后 authority、当前规格和精确冻结来源、resolver、旧迁移路由移除及规范写入身份。
- [x] 四来源包装隔离 API 回归通过，覆盖合组分配、非零成本/审计、失败回滚与幂等；批量补料回归 10 项通过。

烘焙修复已独立提交 `ebe0542b1640a018125a9024b66b74dddbf8e055`。通过 [GitHub PR #306](https://github.com/Backstage2539/KFerp/pull/306) 同步到 develop，功能提交 `456fdb88a5a98ff308b0fc5cc250a9b4ce4de5c3`，合并提交 `e6653f0aa3608d6afee7f321ab9420542139957c`。远端已核对，合并树与已测试功能树一致；该 develop 回合同样通过完整 Go、changed 和隔离 PostgreSQL 定向验证，**未部署 development**。物料 key 修复经 [PR #307](https://github.com/Backstage2539/KFerp/pull/307) 同步 develop `17c18c823`；重量入库及备料状态修复经 [PR #308](https://github.com/Backstage2539/KFerp/pull/308) 同步至 develop `dd603e05083cecbcaab76d506cf983b9fb9c6525`，定向数据库API、Vue1629、Vite及Go门禁通过，仍未部署development。

## 浏览器验收与证据边界

- [x] 本地真实 Vue/API/独立数据库的烘焙两批已点击完成。账务核对记录累计入库 4,994g，三项配料分别扣 4,200g、900g、900g；仅接入一次完整 BOM，产生两条已过账完成事件和四张已过账库存单。首轮 fixture 的物料批次与工时单价为零，只证明零成本计算内部一致；非零成本由上述 API 回归覆盖。
- [x] 隔离包装组按四来源 10/6/4/1 袋合计 21 袋，浏览器先完成 6 袋，再完成剩余 15 袋；第二次来源分配为 4/6/4/1 袋，页面显示整组完成且活动任务为零。保留 6 袋和 15 袋预览/完成截图。
- [x] 第二轮独立 fixture 完成真实页面路径：70% 配方阻断 → 进入 BOM → 发布完整 V002 → 关闭详情抽屉并返回原工位 → 两批各 3kg 投料、2.497kg 产出。最终工单完成、队列为零，材料成本合计 435 元，入库批次合计 4,994g；业务审计含发布1、配方采纳1、完成2，正式 HTTP 操作日志含发布200一次、完成200两次。工序费为零，不能称为非零人工费验证。首轮缺 HTTP 日志中间件的限制保留，第二轮独立补证见 `bom-return-browser-ledger.json`。
- [x] 最终实际计划页面确认备料紧邻对应安排，来源明细可展开。新增稳定物料 key、汇总保留服务端备料状态两组 RED/GREEN；零数量字段不能把“待生产入库”误标为“现场已齐套”。最终截图 `production-plan-grouped-preparation-final.png`。
- Van 最终业务验收及新建“下单→发货”全链路不在本次局部技术验收完成范围内。

## 宽范围回归的未通过项

- 扩展真实数据库 HTTP production 整包初次结果为 262 pass / 19 fail / 5 skip（含子测试，跳过项为显式开启的浏览器夹具）。19项失败均在 main `001c65b0` 定向重现，未发现本候选新增失败；其中无包装规格重量入库是真实缺陷，已修复为 `bbdaa134`。强化 kg/g、非零成本、库存/入库单/批次/生产日志/幂等断言后，相关21个顶层、39个含子测试全部通过、无跳过。其余18项基线失败保持原断言并记录，不能宣称 HTTP 整包全绿。详见 [既有数据库回归失败与重量入库修复](2026-10-26-existing-production-http-tests.md)。生产只读查询零克入库明细影响记录为0，本轮未修正历史数据。
- 包装部分完成后的外层备料摘要可能误报已完成部分的缺口，旧零投料卡再次补料预览也会被数量待核对规则阻止；这是已有边界。本轮6+15完成、账务和真实缺料拒绝均通过，未把部分完成后的补料路径声明为通过。详见 [包装部分完成边界](2026-10-26-packaging-partial-material-known-issue.md)。
- 显式开启旧规格迁移集成开关后，两项原 fixture 缺 authority；临时补齐 fixture 后，旧迁移 cutover 与现行默认 BOM 实际规格移除的并发断言均未通过。探索补丁已撤回，现行并发窗口作为既有独立风险保留，不能用顺序身份校验通过替代。详情见 [既有 BOM 切换并发风险核查](2026-10-26-existing-bom-switch-race.md)。

## 发布状态与本机证据

- [x] 最终 Vite 生产构建通过；保留构建日志。
- [x] 已取得生产只读发布前快照 `production-before.json`，含应用、PostgreSQL 和 35 张表的核对信息。
- [x] 服务器预检及正式 Docker 构建通过；Vue 1,629、小程序432、类型检查和24页构建通过。设置 `KFERP_SKIP_MINIAPP_EXPORT=1`，未导出覆盖本地微信目录。
- [x] [PR #309](https://github.com/Backstage2539/KFerp/pull/309) 已合并；候选树与已验证 f21c41e6 一致。生产于21:14部署 `ac663d9a505a67095f36945e8eda9001c87269d7`，合并包含已验证 develop `dd603e05`。
- [x] 21:19只读核验：未登录页面/API401、登录页200；认证后工位/计划页面、API、生产/销售手册200；订单复制响应包含客户名称和付款方式。三个前端脚本的服务器/容器/公网哈希一致，两个修复后端文件与部署源码哈希一致。
- [x] 发布前21:03至发布后21:20的35表快照：除访问日志增加42条GET外，其他34表行数与内容哈希一致。应用重启0，数据库健康且未重启，磁盘余10GB。
- [x] 回滚源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261026210439-ac663d9a505a`；镜像 `kferp-orderapp-rollback:production-20261026210439-ac663d9a505a`，镜像身份与上一运行版本一致。
- 运行镜像 `sha256:63199fe1d0487a3038e89b4e1e7d63361616d2d6b02360de7f19ccadf7a7676e`。随后仅将 PR-735 更新为 review、DEV-PR735-03 更新为 done，并记录两条审计及一条维护操作日志；未改工单、订单、库存。
- 发布后用户指出单张旧包装按钮及留厂入库转代发分配阻断，另由 [PR-736 补充修复](2026-10-26-frozen-receipt-allocation.md) 跟踪；本次技术发布不等同于现场业务全部验收。

证据保存在工作树同级 `/Users/yiiiple-work/.codex/worktrees/production-followup-release/evidence/`：

- `logs/kferp-followup-roasting-{red,green,regression}.log`：耗料缺陷及隔离回归。
- `logs/kferp-followup-develop-{changed,backend,targeted}.log`：PR #306 合并前门禁。
- `logs/kferp-followup-{backend-all,frontend-all,vue-red,vue-green}.log`：本轮 Go/Vue 验证；`logs/kferp-followup-frontend-final.log` 与 `logs/kferp-followup-vite-final.log` 为最终前端全量与 Vite 构建结果。
- `logs/kferp-followup-{current-identity-contract,spec-migration-explicit-integration,spec-migration-fixture-integration,production-repository-tests}.log`：身份验证通过及失败边界。
- `logs/http-production-full-observed-partial.jsonl`：正在运行的 HTTP 整包日志快照，不是最终结果。
- `production-before-deploy.json`、`production-after.json`、`production-snapshot-comparison.json`：正式发布前后35表快照；`production-operation-interval.json`证明该区间新增42条GET。
- `roasting-browser-ledger.json` 及烘焙/BOM 截图：首轮浏览器结果和已记录限制。
- `packaging-six-preview.png`、`packaging-six-complete.png`、`packaging-fifteen-preview.png`、`packaging-all-complete.png`：包装 6+15 浏览器证据；`packaging-isolated-api.log`、`packaging-four-source-cost-audit.log`：隔离 API 证据。

## 数据边界

所有业务写入验收使用本地独立测试数据库。生产业务检查只读取运行状态和业务快照，不领料、不完工、不入库、不修正库存。发布后管理性写入仅同步上述两条需求进度并记录审计。微信上传、审核与发布不属于本次自动发布。
