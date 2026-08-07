# PR-581 小程序代发、生产工单、库存与账单闭环验收

## 交付边界

- 分支：`codex/mini-customer-fulfillment-closed-loop-20260806`
- 基线：最新 `origin/develop`
- 环境：仅 development；不合入 `main`，不部署 production，不提交微信正式审核。
- 验收数据：通过正常 ERP/API 创建带 `DEV-E2E` 标识的客户仓、库存、BOM、路线、工单和费用模板；保留至 Van 确认后再清理。

## RED 证据

- miniapp：旧生产入口仍为“代加工”、费用中心混入订单、代发仍有批次入口、缺少客户闭环组件和 API；定向 Vitest 首轮按预期失败。
- 后端：各子模块在实现前补充 FIFO/幂等/事务零写入、多商品 BOM 汇总、客户隔离、费用幂等与权限测试。

## GREEN 证据

- miniapp：25 个测试文件、173 项用例全部通过，覆盖入口、闭环工作区、共享解析、商品搜索、重复 SKU 合并、缺料阻断、库存批次、账单明细和能力隐藏；类型检查通过。
- ERP Vue：881 项全量测试通过，Vite 构建通过；财务菜单与视图权限覆盖“费用账单 `finance.read/write`”和“模板维护 `settings.write`”的独立入口。
- Go：`go test ./...`、`scripts/verify_kferp.sh changed`、`scripts/verify_kferp.sh backend` 及 PR-581 专项 verifier 通过。
- 真实 PostgreSQL：代发 FIFO/跨仓/并发/幂等/历史库存，生产 BOM/客户与工厂物料/WIP/预留/完工退料，账单实耗/成本/并发确认等包全部通过。
- development mp-weixin 构建通过，13 个页面、52 个清单文件完整。
- 独立合同审计发现的账单菜单/权限分层问题已完成 RED→GREEN；终审无开放 P0/P1。
- develop 合并、development 部署、正常 API 联调和微信开发者工具导入在发布阶段记录；不涉及 main、production 或微信正式审核。

## 人工验收矩阵

- [ ] 指定开发账号粘贴地址并提交跨仓 FIFO 发货，发货中心看到多个包裹和物流。
- [ ] 多商品生产申请可预览 BOM，库存足够成功进入 ERP 待排产，不足整单拦截。
- [ ] ERP 排产/下达/执行/完工后，小程序工单状态和客户成品仓库存同步。
- [ ] 库存批次详情和“提交生产工单”预填正确。
- [ ] ERP 按费用模板生成并推送真实完工工单账单，小程序显示费用明细且客户自有物料未收费。
- [ ] 未开 `product_order` 的代加工客户个人中心不显示工厂商品入口。
