# PR-653 生产计划自动生成领料建议

实施日期：2026-09-11～2026-09-12。分支 `codex/production-auto-picking-20260911`，从 `origin/develop@1ef28cdb` 独立开发，并合入已验证的统一草稿流程 `4ac85149`。本需求原暂用 PR-652，发现并行需求占号后改为 PR-653；早期日志保留原文件名。

## 行为与兼容

新计划 `picking_version=1`；预览/草稿只读分配，来源记录支持多仓。组件档案决定货主及单位，WIP 优先，其余允许仓按排序、编号，批次 FIFO。手工意图先保留有效数量，剩余自动补齐。提交事务锁批次并重新计算，失败保留草稿；库存物理位置不改变。

“去领料”复用库存单据，绑定冻结来源/货主/规格/批次，支持部分领取和幂等。库存移动与批次预留同事务迁移，同一批次在多仓分别记账；撤销未耗用领料反向恢复。物料与商品组件都须进入 WIP 后耗用/开工。已提交来源只读；新计划领料单显示部分领料说明并锁批次。

旧草稿刷新升级；已提交旧计划保持版本0。冻结BOM、数量、目标仓、成本及旧执行分支保留。沿用上游分配、部分入库和取消释放；备货不推进无关订单。

## 自动验证

| 验证 | 证据/结果 |
| --- | --- |
| RED：自动建议 | `/private/tmp/pr652-red-api.log`：缺少 WIP6000 + 跨仓15000 的分配 |
| RED：部分领料 | `/private/tmp/pr652-red-partial.log`：旧完整来源数量规则拒绝部分领取 |
| RED：真实Vue渲染 | `/private/tmp/pr652-red-ui.log`：3项组件渲染失败，随后实现 |
| RED：商品批次/非WIP耗用 | `/private/tmp/pr652-red-typed.log`、`pr652-red-consumption.log`、`pr653-batches-red.log` |
| 完整标准门禁 | `/private/tmp/pr653-final-gates2.log`：Go全包、Vue1193/1193、Vite、差异检查通过 |
| 隔离PostgreSQL/API | `/private/tmp/pr653-api-verified2.log`：全部定向测试通过，无数据库跳过 |
| 更多现有回归 | `/private/tmp/pr652-relevant-api.log`：正式BOM、冻结规格、多层依赖、旧工单执行、在途/部分入库等通过 |
| 纯分配规则 | `auto_picking_test.go`：共享库存预算、手工优先、WIP优先、仓库排序和计件缺料 |

定向真实数据库用例包含：

- 21kg = WIP6kg + 原料仓10kg + 备用仓5kg；草稿不产生预留，提交生成三条同批次不同仓预留且不转仓。
- 领4kg后不能开工；相同请求只产生一次领料；撤销后库存及预留恢复；再领齐后允许开工；禁止从非WIP耗用，WIP耗用更新实际批次消耗。
- 手工固定原料仓15kg，两次刷新保持WIP6kg；供应移除时提交失败点明物料并保留草稿，无残余预留。
- 多货主共享WIP，排除其他货主及hold批次；预览不建计划。
- 商品组件按冻结规格选批次，库存/上游产出均先转WIP；领料草稿恢复保留货主、BOM规格/变体、批次和编辑锁；成本及消耗批次可追溯。
- 现有并发提交、在途配额、部分入库提前包装、最终少产、取消释放和重复请求测试通过。

### 扩展套件的历史失败

真实数据库全量生产API套件并非全绿。基线 `1ef28cdb` 记录14个既有失败（`/private/tmp/pr652-baseline-api.log`）；合入 `4ac85149` 后撤销夹具修复了1个，剩余13个均在旧基线已有，主要为历史需求换算、旧父BOM或最小WIP夹具。完整扫描 `/private/tmp/pr653-production-suite.log` 最初另发现1个本次测试辅助函数在已齐套时仍请求领料，移除多余领料步骤后该完整多层生命周期单测通过（`pr653-material-receipt.log`），并已加入最终定向套件。

基础设施可选数据库套件与旧基线同样存在18个夹具失败（`pr652-repository-tests.log` 对比 `pr652-baseline-repository.log`，缺少owner字段等），没有把跳过或基线失败计作通过。标准 Go 门禁默认不启用这些可选数据库夹具。

## 页面核对

使用实际 Vue、真实生产/库存API及本地独立PostgreSQL schema；仅登录/导航测试壳和无关资料接口使用占位响应。业务写操作全部在隔离数据，不修改开发环境已有订单/计划。手工保存后刷新保留来源；390px宽度=页面宽度，无横向溢出。提交后来源冻结并显示去领料；物料、两个来源仓10kg/5kg、WIP目标和批次自动带入库存单据。浏览器点击“提交并过账”生成SE-0000000001（15kg、2行），回到计划显示WIP21kg、待领0、现场已齐套。SQL验证两来源仓余量0，WIP库存与预留均21000g；create/submit及领料操作日志已留存于 `/private/tmp/pr653-browser-ledger-evidence.txt`。

截图目录 `/private/tmp/kferp-pr653-acceptance/`：

- `01-preparation-sources.png`：最终实际备料明细。
- `02-manual-restored.png`：保存并刷新后的手工意图。
- `03-narrow-preparation.png`：390px备料及来源明细。
- `04-submitted-awaiting-picking.png`：计划已提交，仍待领料。
- `05-picking-document.png`：冻结来源的领料单。
- `06-wip-ready.png`：领料后WIP齐套。

页面检查中同步修复了备货品种显示为0、已提交仍提示保存草稿、库存单据旧超额领料提示与新规则冲突的问题。自动化/隔离页面验证不代替 Van 业务验收。

## 手册与交付

单一来源手册 `docs/OP_MANUAL_PRODUCTION.md` 与 `docs/OP_MANUAL_STOCK.md` 更新；Vue“备料说明”跳转现有生产手册。PR/DEV种子和ACTIVE记录已维护。

目标development，production及微信发布不在范围。开发发布、回滚、只读检查和实际线上截图将在发布后补录。Van验收待进行。
