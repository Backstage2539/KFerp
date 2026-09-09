# PR-645 物料失效库存门禁与 BOM 分类移动

2026-09-09；实现与数据清理：Codex；业务验收：Van。代码从 `origin/develop` 的 `32279ff6` 建立独立分支 `codex/material-stock-bom-move-20260909`。本次按要求修复代码并清理开发、生产数据，应用部署未执行。

## 复现与原因

- 生产物料 88“生豆-乌干达（罗）”已于 2026-09-09 21:10 失效，但主档及批次 `LEGACY-MAT-0000000088` 仍有 60500g，仓位也仍有 60500g。失效接口直接标记 `deprecated_at`，此前没有库存校验。
- 生产“红岩”在分组 165、深烘分类 890 中有 BOM 106/366。此次未代替 Van 选择或移动任何真实 BOM。源码中，已选对象从 `productionBomVisibleRows` 计算；移动模式折叠分类后可见行为空，点击目标前就显示“请先勾选生产 BOM”，不发写入请求。

## RED → GREEN

- `material_stock_deprecate_test.go`：真实 PostgreSQL + HTTP，原实现的 6 个非零库存子场景全部错误返回 200（含 60.5kg 重量、件数、批次、仓位正负抵消、库存批次镜像、客户库存）。现在全部返回 400、保留物料启用且不写成功日志；全部清零后返回 200 并记一条失效日志。
- `bom-category-selection.test.js`：原实现 3 项全部失败，复现折叠后已选为空、取消丢选、重试误报未勾选。现在按已勾选业务身份从过滤后的原始行取对象；移动期间不剪除选择，取消时等可见行恢复后再核对。当前分类/当前页全选规则保持。3 项以及共享真实 Vue SFC 的分类折叠恢复测试通过。
- `material_inventory_cleanup_test.go`：先验证预览阶段不能接受过期清单。最终验证默认预览只读、清单校验值变化拒绝、审计错误失败、最后一步删除失败时主档/批次/仓位/库存镜像/审计整笔回滚；成功仅清理失效或缺失主档对应库存，启用物料及商品库存不变，原始批次数量保留，重复执行不产生额外日志。
- 手册：`docs/OP_MANUAL_INVENTORY_MATERIALS.md`、`docs/OP_MANUAL_PRODUCTION.md`；物料 Vue 页提示已同步。需求与验收清单、PR/DEV 种子同步，PR 状态为 review，待 Van 产品验收。

## 验证命令与结果

本机 PostgreSQL 16 独立库 `kferp_material_cleanup_test`；每个测试独立 schema，测试不连接开发或生产数据库。

```sh
cd orderapp-remote
ORDERAPP_TEST_DATABASE_URL='postgres:///kferp_material_cleanup_test?host=/tmp' go test ./internal/interfaces/http/materials -run 'TestMaterialDeprecateRequiresZero|TestOrphanMaterialInventoryCleanup|TestMaterialsAPICreateCopyDeprecate' -count=1 -v
```

- 上述 3 个顶层测试及 6 个非零库存子场景通过，无跳过。
- `scripts/verify_kferp.sh frontend`：1136/1136 通过，0 跳过；Vite 构建通过。
- `scripts/verify_kferp.sh backend`：标准 Go 全包检查通过。未设置 DSN 的标准门禁不计作真实数据库验证。
- 真实数据库额外物料 API + repository 整包：41 个顶层测试通过，2 个既有失败，0 跳过。以下两项在未修改基线 `32279ff6` 的相同数据库环境同样失败，未扩大范围修复：
  - `TestMaterialsAPISemiFinishedCanManufacture`：已有制造 BOM 时禁止切回外购，旧 fixture 期望不符。
  - `TestMaterialsAPIAutoZeroesSemiFinishedPriceAndAuditsTogglePostgres`：半成品采购价必须为 0，旧 fixture 仍提交旧非零价格。
- 日志：`/tmp/kferp-pr645-final-focused.log`、`/tmp/kferp-pr645-bom-green.log`、`/tmp/kferp-pr645-frontend-green.log`、`/tmp/kferp-pr645-backend.log`、`/tmp/kferp-pr645-materials-final.jsonl`、`/tmp/kferp-pr645-material-baseline.log`。

## 两环境数据清理结果

清理使用仓库脚本 `scripts/database/cleanup_orphan_material_inventory.sql`。默认只读，显式 `apply=true` 且提供预览的 `expected_manifest` 才写入；短事务锁定相关库存表并重新校验清单。每个物料的完整原始主档、批次、仓位、库存镜像及客户库存快照写入操作日志。

清理内容为主档当前余额和批次余量归零、删除仓位及客户库存行；不删除批次历史，不改原始入库数量、成本、库存流水、订单或 BOM。主档不存在及已失效均纳入；本次两环境候选实际均为已失效主档。

| 环境 | 涉及物料 | 清理后残留 | 操作日志 | 清单校验值 |
| --- | ---: | ---: | ---: | --- |
| development | 12 | 0 | 12 | `3abbe65ecf93158bafef75dcf87787ff` |
| production | 23 | 0 | 23 | `6fd3608e7752db9b377bc6a45c840f9e` |

开发 ID：36、37、38、39、40、41、46、47、48、49、50、51。含 6 个非零余额物料和 6 个零余额仓位残留。
生产 ID：33、35、41、43、44、45、46、52、55、56、65、76、77、79、80、83、84、88、89、90、93、94、97。

执行时间：2026-09-09 22:38（Asia/Shanghai）。操作日志 action 为 `remove_orphan_inventory`，操作者 `Van/Codex PR-645`。事务后重新运行预览，两边 count 均为 0，审计条数分别为 12、23。

生产乌干达复核：主档 `onhand_g/onhand_units=0/0`；目标批次 `remaining_g/remaining_units=0/0`、状态 consumed；仓位行数 0；原始 `qty_g=60500` 保留。仓库查询数据源的当前库存已移除。公共 API 只读尝试返回 401，需要用户登录会话，因此没有把该请求当作页面验收通过；数据结果由独立数据库读取核对。

## 备份与恢复依据

两份完整数据库备份均经 `pg_dump -Fc` 成功、`pg_restore -l` 可读和 SHA-256 校验。每个目录同时保留 `preview.json`、实际执行的 `cleanup.sql`、`apply.log` 和 `result.json`。

- 开发：`/opt/stacks/erp/backups/pr645-orphan-inventory-20260909T223824/database.dump`；15723853 bytes；SHA-256 `92bcb6ac83610148dcf95d7eca3e17baadb22073d3eaa57f7ada051f0b1f92ec`。
- 生产：`/opt/stacks/erp-production/backups/pr645-orphan-inventory-20260909T223828/database.dump`；5599667 bytes；SHA-256 `011b9416c6f6e58b7bac64a492331beac5d85e4354ea906c4ac6121322271efc`。

需要恢复时可根据单个物料的审计原始快照进行有范围的恢复，或由环境负责人基于完整备份处理；本次未执行恢复或服务重启。

## 交付边界

代码合入 develop，应用未部署，故新失效门禁和 BOM 分类选择修复尚未在生产页面生效。本次库存数据修复已经在开发、生产数据库完成，刷新仓库可重新读取；真实 BOM 分类移动由 Van 在后续发布后验收。没有修改红岩 BOM 分类、版本、物料配方或商品配置。
