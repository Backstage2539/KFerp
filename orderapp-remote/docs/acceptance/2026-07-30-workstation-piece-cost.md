# PR-563-WORKSTATION-PIECE-COST acceptance evidence

## Scope

- 工位产能支持 `cost_method=time|piece`；历史/缺省方式为 `time`。
- `piece_rate` 表示每 1 个具体 SKU 销售规格件的成本，费率口径冻结为 `sales_spec_count`；新计件产能的标准产能单位固定为 `件`。
- 计划计件成本为 `计划数量 × 冻结计件费率`；实际计件成本为 `实际产出数量 × 冻结计件费率`。
- 工艺路线标准成本产能档、BOM 工序成本快照、生产计划、工单、工序卡和成本追溯保留成本方式、费率、单位和换算来源。
- 100 个“初晓拼配 227g”销售规格件能分别核对烘焙、色选和包装成本；该 SKU 的100件即100袋，包装0.5元/销售规格件时为50元。

## PR / DEV

- PR-563-WORKSTATION-PIECE-COST
- DEV-563-CAPACITY-COST-METHOD
- DEV-563-STANDARD-COST-SNAPSHOT
- DEV-563-PLAN-ACTUAL-PIECE-COST
- DEV-563-UI-AUDIT
- DEV-563-DOCS-ACCEPTANCE

## Fixed Acceptance Dataset

- 商品：初晓拼配；销售规格：227g；具体 SKU 权威换算：`1销售规格件=0.227kg`。
- 生产数量：100个销售规格件（该 SKU 即100袋）；库存数量：22.7kg；BOM 原料损耗和整体损耗在本成本公式验收中均设为0，避免把损耗误当工序成本。
- 工艺路线：烘焙 → 色选 → 包装。
- 烘焙：`time`；工位小时成本24元；标准批量10kg；标准分钟30分钟/批。计划22.7kg为3批，计划分钟90，计划成本36元。
- 色选：`time`；工位小时成本12元；标准批量25kg；标准分钟30分钟/批。计划22.7kg为1批，计划分钟30，计划成本6元。
- 包装：`piece`；标准产能单位件；计件费率0.5元/销售规格件。计划100件，计划成本50元。
- 计划工序成本合计：`36 + 6 + 50 = 92元`，API/UI 必须保留三行明细。
- 实际正例：实际分钟分别为90和30，包装实际产出100件，包装实际成本50元。
- 实际差异例：包装实际产出98件，实际包装成本49元，用于证明实际成本不读取计划100件。

## Evidence Targets

- Schema/migration：成本方式和费率字段可重复迁移；历史记录确定性读取为 `time`，历史快照不重写。
- Manufacturing API/service：合法时间/计件保存、计数单位校验、工位/工序归属校验和操作日志 meta。
- BOM/costing：发布冻结时间/计件工序快照；使用 SKU 权威换算折算计件标准成本；缺失或跨维度换算时明确阻断。
- 递归 BOM：最终销售规格计件行不进入中间件每库存单位聚合；具体成品 SKU 的价格试算按自身冻结换算加入，避免袋/盒等不同规格串用费率。
- Production：拆分和工单冻结成本方式；时间成本按批次/分钟，计件计划成本按计划数量；工位完成后计件实际成本按实际产出。
- Frontend：工位产能表单按方式显示字段；生产计划、工单、工序卡和成本追溯分列烘焙/色选/包装成本及合计。
- Audit：工位产能保存/修改/停用和生产执行状态转换可在操作日志查询；只读成本查询不产生业务日志。

## Verification Commands

- Backend targeted:
  - `go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/application/bom ./internal/infrastructure/postgres/bom ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`
- Frontend targeted:
  - `node --test src/lib/process-routes.test.js src/lib/produce-plan.test.js src/lib/production-costs.test.js src/lib/manufacturing-execution.test.js`
- Frontend build:
  - `npm run build`
- Review:
  - `scripts/verify_kferp.sh changed`
  - `git diff --check`
- Deployment smoke after implementation:
  - development API returns `cost_method/piece_rate/rate_unit` snapshots and three operation cost lines;
  - logged-in browser shows 100袋包装成本50元 and separate roast/sort/package lines;
  - production remains untouched.

## Current Evidence

- Requirements and acceptance contract drafted on 2026-07-30.
- RED/GREEN regression coverage added for cost-method validation, unit normalization, BOM/cost snapshots, loss-adjusted input versus finished sales-spec count, missing/mixed/conflicting operation splits, planned/actual piece cost, recursive-BOM boundary and operation-log metadata.
- Targeted backend packages passed:
  - `go test ./internal/infrastructure/postgres/production ./internal/infrastructure/postgres/manufacturing ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/application/costing ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/costing ./internal/interfaces/http/production ./internal/interfaces/http/support`
- Targeted frontend passed `77/77`:
  - `node --test src/lib/workstation-capacity-costing.test.js src/lib/process-routes.test.js src/lib/produce-plan.test.js src/lib/work-orders.test.js`
- Full frontend comparison: feature branch `819/826`; the 7 failures are the same workspace-context baseline failures and contain no PR-563 case.
- Vue/Vite production build passed with `396` transformed modules; generated `index-Cf3Mre-d.js` and `index-DKkhIOE4.css`.
- `scripts/verify_kferp.sh changed` and `git diff --check` passed.
- Fixed acceptance calculation passed: roast `36` + sort `6` + package `50` = `92`; package actual output `98` produces `49`.
- Temporary PostgreSQL passed `TestPR563PieceCapacityPersistsAndAuditsInPostgres`, including idempotent schema, piece capacity persistence, deactivation and `rate_unit=sales_spec_count` audit metadata.
- Isolated server Docker build passed complete `go test ./...`; Vue and miniapp typecheck/build passed before container switch.
- `origin/develop` merge/deployment commit: `a3706ce2f3680ecbaa92787803ba72e2370a9afd`; candidate/running image: `sha256:61c263bb8808cec3484d6208d1be489539d1126d6550f2ef416f002c4572cdb0`.
- Development safeguards:
  - database backup: `/opt/stacks/erp/backups/pre-pr563-20260730184251.dump`;
  - source backup: `/opt/stacks/erp/orderapp.backup.deploy-20260730184251`;
  - rollback image: `erp-orderapp:rollback-pr563-20260730184251`.
- Development smoke passed: container remains running, PostgreSQL healthy, `/app/` and `/vue-shell/` return `200`, and deployed shell references `index-Cf3Mre-d.js` / `index-DKkhIOE4.css`; protected manufacturing/process-route APIs return expected unauthenticated `401`.
- In-app logged-in browser acceptance is not claimed: navigation is blocked by development certificate `ERR_CERT_AUTHORITY_INVALID`. Production was not deployed and no real workstation, production or inventory business write was performed.
