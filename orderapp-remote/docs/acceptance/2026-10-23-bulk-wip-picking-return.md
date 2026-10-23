# PR-722 整袋领料到 WIP 与返回工位

## 范围和生产只读基线

2026-10-23。实现分支 codex/bulk-wip-picking-return-20261023，自 origin/develop 8091b262；原 ccd2 工作区已有修改保持。DEV-851 兼容快照与精确撤销，DEV-852 超领及库存保护，DEV-853 返回和草稿保护，DEV-854 验证、手册、生产交付。Van 业务验收独立。

生产运行 6b3e67af。SE-0000000037 / work order 19 / job card 21 为草稿，4 行均为 60000g；只读基线哈希 e520c42ad38ec4c740b61670c0e30976cebbfa11bf1ebf510af66153300e62b7（包含工单、工序卡、当天排班）。不代用户提交该单。

## 原因与实现

自动领料路径在数量校验、预留移动两处把实际数量限制为未领预留，与旧批量领料提示矛盾。实际库存移动与预留转移需分离，剩余 WIP 保持同货主自由库存，BOM/消耗/产量不改变。

返回入口原先只在无可领明细提示块内显示，工位来源又被预览请求覆盖为 work_order。修复返回链接位置、范围/任务上下文及未保存修改的三选项处理。导航上下文仅保留前端内存。

内部 nullable picking_reservation_snapshot 保存本行实际预留移动。NULL 为旧单；新单即使只转移自由库存也保存 version/moves 空记录；事务内精确撤销。兼容提交仍保留旧上限，再由后续提交放开数量，兼容镜像为最低安全回滚版本。

## RED / GREEN

- HTTP/PostgreSQL RED：/tmp/kferp-pr722-api-red.log；真实自动领料计划预留 15.57kg，60kg 草稿保存重载正常，但提交返回 400 待领预留已变化；重复行与并发整袋领取同样复现。
- 兼容仓储 RED /tmp/pr722-compat-red.log → GREEN /tmp/pr722-compat-green.log；兼容既有自动领料 API /tmp/pr722-compat-api.log。独立 main 基线全 Go /tmp/kferp-pr722-compat-main-backend.log 与 PostgreSQL /tmp/kferp-pr722-compat-main-pg.log 通过。
- 批量领料 HTTP/PostgreSQL GREEN /tmp/kferp-pr722-api-green.log；同工单与不同工单共享批次并发重复三次 /tmp/kferp-pr722-api-concurrency-green.log。覆盖草稿重载、60kg提交、幂等提交、自由库存补领、非倒序撤销、kg/g/件、质量冻结、货主/来源身份、其他预留、审计失败原子回滚。
- 审查发现混合动作锁顺序风险；真实 HTTP + PostgreSQL 锁屏障复现 SQLSTATE 40P01（/tmp/kferp-pr722-api-lock-order-old-red.log），统一 draft/create-submit/submit/cancel 的工单→物料→批次加锁顺序后 GREEN（/tmp/kferp-pr722-api-lock-order-green.log）。
- Vue runtime RED /tmp/kferp-pr722-frontend-red.log → 12 新用例 GREEN /tmp/kferp-pr722-frontend-green.log；完整前端 1,557 项通过 /tmp/kferp-pr722-frontend-all.log，构建 /tmp/kferp-pr722-frontend-final-build.log。
- 真实 PostgreSQL 全 stock 包有与本任务无关的既有商品 BOM fixture 失败，8091b262 overlay 同样复现（/tmp/pr722-stock-baseline-existing-failures.log）；本任务新旧领料、撤销、并发用例单独运行通过，不把跳过数据库的全 Go 检查当作这些数据库用例已通过。
- 最终真实 PostgreSQL API：11 个新增顶层用例及4个现有 AutoPicking 回归全部通过（24.125s，/tmp/kferp-pr722-api-final-green.log），包含客户货主60kg闭环和商品组件新批次/预留/审计/撤销闭环。客户撤销流水缺 owner 的 RED /tmp/kferp-pr722-api-owner-ledger-red.log；最终两笔流水均保留货主。兼容镜像同样回补此项后才作为最低安全回滚。
- 完整 Go 包检查 /tmp/kferp-pr722-final-backend.log 通过；审查的锁顺序 finding 已修复复审。
- 所有业务写入测试在本机隔离 schema 或隔离浏览器数据中进行。

## 手册与页面验证

单一来源手册：docs/OP_MANUAL_STOCK.md、docs/OP_MANUAL_PRODUCTION.md 的 PR-722 节；Vue 页面的数量提示同步更新。本地隔离接口下使用实际 StockOperationsView/StockEntriesView/WorkstationView，浏览器已验证：

- 全厂工位进入领料，改为60kg；继续编辑保持输入，保存草稿并返回保留 all/工位/工单/任务21，重进草稿仍60kg。
- 提交成功留在单据页，手动返回后重新显示 WIP 齐套并定位原任务；任务离队时自动打开只读工单详情。
- 保存失败显示错误，三选项弹窗和60kg输入保留；放弃返回保留 mine 范围，不提交。
- 390px 窄屏抽屉可操作；提交失败后两行60kg均保留，返回原我的工位。
- 预览失败仍同时显示页面及抽屉返回入口；直接进入无 return_navigation 时无虚假入口。
- 截图 /tmp/kferp-pr722-ui-proof/save-failure.png；仅隔离 UI mock 保存/提交，生产零业务写入。

## 集成与发布

兼容提交80c38c05793377731375abfd1dc685d162f742ae，经开发PR #269合入74b1947a；生产 cherry-pick606fbbdc经PR #270合入8a86ee97。兼容生产构建正在运行。最终 develop/main PR、镜像、生产发布和只读核对后续补记。发布前检查共享锁及磁盘，保护数据库、数据卷、源码与镜像回滚依据。
