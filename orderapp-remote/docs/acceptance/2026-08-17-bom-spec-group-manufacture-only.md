# PR-600 BOM 规格组、统一配方模式与半成品制造专供

## 范围

- 保持普通 BOM 的物料/商品类型化产出，不恢复包装 BOM、半成品 BOM 或 `bom_kind`。
- 商品 BOM 以 BOM 专属规格组承载多个完整规格配方；新业务身份为父商品 + BOM 规格。
- 逐商品提供迁入、校验和切换工具，不自动生成配方，不自动切换现有商品。
- 交付范围为 `develop` 与 development only；`main`、production 和人工配方录入不在范围。

## RED 证据

- `go test ./internal/interfaces/http/support -run TestDev600BomSpecGroupManufactureOnlyContracts -count=1`：缺少 PR-600 跟踪种子时失败。
- `node --test src/lib/bom.test.js`：旧页面仍含“有损耗的配方 / 无损耗的配方”，缺少规格组入口和响应式表单合同，2 个用例失败。
- `go test ./internal/application/bom -run 'TestUpdateProductionBomDraft(Applies|Rejects)' -count=1`：旧服务仍接受损耗配方和无损耗配方中的比例/固定混用。
- `go test ./internal/application/productspecmigration -count=1`：逐商品迁移状态、readiness、cutover 和规格身份解析合同尚不存在，编译失败。
- 半成品 schema 旧实现会在存在未完成采购单、采购收货或普通入库时直接清零主档采购价；真实 PostgreSQL 三个场景均错误返回成功。采购收货三段事务与半成品切换并发时，旧实现允许切换穿过已提交库存段，存在部分库存风险。
- 十规格模板、商品 BOM 规格身份、逐商品迁移、订单库存、生产、代发和代加工分别先以不存在字段、旧子 SKU 回退、规格库存串用或 `spec_g=0` 被跳过的失败用例固定 RED。
- 同一商品的两个 BOM 规格成本最初被汇总到父商品；规格组件也没有按 `component_bom_spec_id` 递归。真实 PostgreSQL RED 明确表现为指定规格没有独立成本节点。
- 默认 BOM 发布新版本最初会让旧版本行 ID 失效；旧实现也未阻止删除仍有库存或订单引用的规格。并发逐商品 cutover 最初可穿过旧业务写事务。

## GREEN 证据（最终稳定树）

- BOM Vue 单列表、统一配方模式、规格模板/规格组入口和四列/两列/单列表单定向测试已通过；Vite 构建通过。
- 真实 PostgreSQL 已覆盖十规格模板复制、模板修改不回改已复制 BOM、第十规格失败整组回滚、同 BOM 规格身份跨版本稳定、不同 BOM 同名规格隔离、规格单位锁、类型化循环和默认版本删除规格门禁。
- 半成品真实 PostgreSQL 已覆盖采购价同事务清零与双字段审计、未完成采购/收货/普通入库迁移阻断、采购收货与半成品切换串行化、普通入库拒绝、库存调整审计和递归制造成本。
- 逐商品迁移真实 PostgreSQL 已覆盖准备、readiness、原子切换、幂等重试、旧子 SKU 墓碑、库存/预留/订单/计划/工单/客户履约阻断、并发写门禁和历史快照不改写。
- 订单、价格、库存、生产和客户履约定向真实 PostgreSQL 已覆盖父商品 + BOM 规格身份、规格数量 1:1、同规格跨版本库存沿用、不同规格库存隔离及 `100 袋 = 22.7kg 主料 + 100 个袋材`。
- `scripts/verify_kferp.sh all` 在最终稳定树 exit 0：Go 全量、Vue 1013/1013 和 Vite 6594 modules / 2.04s 全部通过；`go test ./... -run '^$' -count=1` 与 `git diff --check` 通过。
- 小程序最终稳定树 34 files / 217 tests 通过，`vue-tsc --noEmit` 通过，development `mp-weixin` 构建通过且只固化开发 API。
- 最终真实 PostgreSQL 串行回归通过 app bootstrap、BOM、catalog、materials、purchase、stock、inventory、production、productspecmigration、costing、sales、customerportal 与 customerfulfillment；production HTTP 73.778s、customerportal 39.645s、customerfulfillment 34.437s，销售新旧库存闭环定向 8/8。
- 最后补入的商品 BOM 模板强制门禁、规格新增/删除/重套模板，以及成品调拨/盘点服务端解析当前规格版本，已纳入上述 Go/Vue/Vite/真实 PostgreSQL 最终回归。
- 独立最终只读发布门禁确认 0 P0 / 0 P1，P2 无新增；复验模板来源、主投入、默认切换、条码、并发锁序、商品组件规格单位和 legacy 兼容均通过。

## 不可变验收合同

- 开启损耗后所有组件为物料比例行；未开启损耗时比例和固定不能混用。
- 半成品主档采购价为 0，不能采购、收货或普通入库；库存调整仍审计。
- 一个模板版本复制成一个 BOM 专属规格组；模板变化不回改已有 BOM，规格组整组原子发布。
- 同一 BOM 同键同单位规格身份稳定，不同 BOM 的同名规格相互隔离。
- 默认 BOM 规格决定新价格、订单与库存候选；新商品规格数量与库存数量 1:1。
- 迁移工具不猜配方；旧业务占用未清零时不得 cutover，历史快照不改写。

## Development 交付

- Feature branches：`codex/pr600-bom-spec-groups@8a33c695` 与最终表单对齐修复 `codex/pr600-bom-alignment-release-fix@3839b332`；自动化验证、独立发布门禁和远端 development 预检均完成。
- `develop` merge：已完成；功能发布合并提交 `c80a46ea`。
- development deployment：已完成；发布内容包含 `develop@c80a46ea`，固定开发小程序包已同步。
- Backup：`/opt/stacks/erp/backups/pr600-pre-bom-spec-groups-20260816T222807Z-2309674f.dump` 已在临时库完整恢复并核对聚合签名。
- HTTP / browser smoke：development 登录、BOM 页面和规格模板接口通过；桌面与窄屏几何验收由本次发布记录覆盖。
- 商品自动 cutover：禁止。
- `main` / production：不操作。

## Van 人工验收

状态：pending。以下项目保留给 Van 在 development 环境人工确认，自动门禁与浏览器只读检查不替代业务验收。

- [ ] 在 BOM 页面建立十规格模板，复制到商品 BOM 并逐规格核对配方。
- [ ] 检查损耗比例与固定用量互斥提示。
- [ ] 检查半成品隐藏采购价且采购/入库候选中不可见。
- [ ] 检查桌面和窄屏 BOM 表单对齐、无横向溢出。
- [ ] 选择一个无未结业务的测试商品执行 preparing → ready → cutover。
