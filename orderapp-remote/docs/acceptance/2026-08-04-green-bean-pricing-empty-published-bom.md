# PR-577 生豆价格试算空发布 BOM 诊断与旧回填保护验收记录

## 当前状态

- 产品需求：`review`，等待 Van 业务验收。
- 开发需求：实现、完整回归、真实 PostgreSQL 临时 schema 验证、第二轮独立复核、develop 合并、development 部署及只读冒烟均已完成。
- Van 业务验收：`todo`。
- production 仅做只读诊断；没有保存、发布、归档或绑定任何真实 BOM，也没有重算价格表。

## 现场只读复现与根因

- 商品：`萨其姆-生豆（SKU-000911）`；价格计算模板：`生豆计算模板-麻袋`。
- 当前 production BOM 主表为 `BOM-000911`。默认绑定指向 published `V001`，但该版本组件数为 0；同一 BOM 的 `V002` 是 draft，组件数为 1。
- 旧逻辑只读取 active production BOM 的 published 版本，因此读取空 V001 后标准制造成本为 0；它不会读取 V002 草稿，这个安全边界本身正确，但缺少了对“空发布版本旁已有非空草稿”的明确诊断。
- V001 是旧兼容绑定修复生成的空壳。旧回填/绑定条件允许仅凭空 `product_bom` 或 `bom_versions` 壳建立 published V001 与绑定，是该异常数据能形成的代码原因。
- production 商品当前形态及 V002 损耗口径仍需业务人员在正式发布前核对。本修复不自动纠正商品档案或发布 V002，避免把 0 元变成另一种错误价格。

## 修复合同

- 试算选中组件数为 0 的 published 版本、同 BOM 存在有组件 draft、且没有正数成本覆盖或可独立计价的正数工序成本时，单次试算返回业务错误，批量试算把同一错误写入对应行。
- 错误明确包含当前 published 版本、非空 draft 版本、`没有组件`、`草稿未发布` 和 `生产管理 → 生产 BOM` 入口。
- 仅读取草稿版本号和“存在组件”的布尔事实；不读取草稿组件成本，不自动发布或绑定，不回退历史 ProductInput 汇总成本。
- schema 回填、PR-403 绑定修复和特殊属性冲突拆分都要求组件来源；空旧壳只可成为 draft 且不建立默认绑定，有组件来源仍正常迁移并发布。PR-403 repair 在显式事务中先取得 schema 级 advisory transaction lock，再通过 `RETURNING` 明确串联新主表、版本、组件和绑定，一次调用内原子完成；重复或并发请求不会复制组件。
- 显式 `inherit_version` 继续绑定同一 BOM 中有组件的历史版本，不因历史版本已归档而静默切换到最新版；空版本、缺失版本或跨 BOM 异常引用不建立绑定。

## TDD RED 证据

- 应用层新增单次/批量回归后，旧实现均返回 `final_unit_price=0` 和笼统的“暂无可试算标准制造成本”，没有指出 V001/V002 状态。
- PostgreSQL 成本选项合同最初没有查询 `production_bom_version_items` 组件数和同 BOM 非空草稿元数据。
- BOM 修复回归最初证明 PR-403 候选仍接受空 `product_bom`，schema 回填仍会把空 legacy 版本创建为 published 并绑定。
- 控制测试 `TestPricingRuleTrialIgnoresLegacySummaryCostWithoutOutputBomDetails` 在 RED 阶段已通过，证明修复没有依赖恢复不可追溯历史汇总成本。

## GREEN 与数据库级证据

- 定向 Go：`go test ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/infrastructure/postgres/bom ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1` 通过。
- 完整 Go：`go test ./... -count=1` 通过。
- HTTP/API：`TestPricingRuleTrialAPIRejectsEmptyPublishedBomWithNonEmptyDraft` 返回 400，并包含 V001 无组件及 V002 草稿发布指引。
- 在 development PostgreSQL 的独立临时 schema 中验证 schema 回填四类来源：空 `product_bom`、有组件 `product_bom`、空 legacy version、有组件 legacy version。空来源得到 draft/无绑定；有组件来源得到 published/一条组件/有绑定；断言后回滚，没有持久化测试数据。
- 新增条件 PostgreSQL 集成测试并在 development 数据库执行：`TestRepairLegacyProductionBomBindingsPostgresOnce` 证明新 BOM、published V001、组件和绑定在一次调用中完整形成，空来源不迁移，已有非空发布版本只补绑定，重复及两个并发 repair 后组件/绑定仍各一份；`TestPricingRuleTrialProductionOptionsPostgresEmptyPublishedWithDraft` 证明真实查询和 pgx Scan 返回 `V001/component_count=0/V002 draft nonempty`。两个临时 schema 均在测试结束删除。
- production 等价只读查询确认 `BOM-000911 / V001 / component_count=0 / V002 draft nonempty`，没有执行写入。
- Vue/Vite build、`go test ./... -count=1`、`scripts/verify_kferp.sh changed/backend`、完整支持合同和 `git diff --check` 已通过。
- 首轮独立复核发现 data-modifying CTE 快照/并发幂等、原始计件费率误放行、负数覆盖错误被遮蔽、特殊属性复制空版本和固定历史版本回落风险；均补回归并修复。第二轮复核确认无阻断项；残余仅为同 schema repair 短暂串行及管理员恰在试算时发布版本可能出现一次过时提示，重试即可且不会返回错误价格。
- 功能提交 `07f374c8` / `28d4ff45` 已推送，合入 `develop` 的应用提交为 `a16d5d6ad85a997e067b47d525eeeb3f7ebecd0c`。development 发布过程再次通过 Vue 871 项、小程序 152 项、类型检查、两端构建、完整 Go 测试和 Docker 构建。
- development 发布后 `erp_orderapp` 正常运行，外部登录 `https://dev.qacoohee.com/app/login` 返回 HTTP 200；部署源码包含 `pricingRuleTrialRejectEmptyPublishedBom` 和 `pg_advisory_xact_lock` 标记。源码备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260804005441-a16d5d6ad85a`，回滚镜像为 `kferp-orderapp-rollback:development-20260804005441-a16d5d6ad85a`。

## 历史、数据与操作日志边界

- 不新增数据库字段或数据迁移，不自动修复、归档或发布 production 现有版本。
- 历史 BOM、已发布价格表、PDF、订单和财务快照不重算、不回写。
- 价格试算和诊断是只读，不新增操作日志类型。管理员后续保存/发布 BOM 继续沿用现有操作日志。
- 明确的正数临时基础成本和合法正数工序成本不会被空组件诊断误阻断；0 元价格表发布仍按既有规则拒绝。

## Van 验收清单

- [ ] 在 development 构造 published V001 无组件、draft V002 有组件的数据，单次和批量价格试算均显示精确发布指引，不显示可用 0 元结果。
- [ ] 发布有组件的 V002 后重新试算，按该版本真实物料/工序成本得到正数，并能追溯到 V002。
- [ ] 只保留空旧壳时，兼容回填不创建 published 版本和默认绑定；添加真实组件来源后迁移仍成功。
- [ ] production 正式处理 SKU-000911 前，核对商品形态、分类、产出单位、V002 组件和整体/原料损耗，再由有权限人员发布；发布日志可追溯。

## 部署边界

- development：已部署应用提交 `a16d5d6ad85a997e067b47d525eeeb3f7ebecd0c`，自动测试和只读冒烟通过。
- production：未部署，未执行业务写入。
