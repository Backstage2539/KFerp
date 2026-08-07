# PR-581 小程序代发、生产工单、库存与账单闭环验收

## 交付边界

- 分支：`codex/mini-customer-fulfillment-closed-loop-20260806`
- 基线：最新 `origin/develop`
- 环境：仅 development；不合入 `main`，不部署 production，不提交微信正式审核。
- 验收数据：通过正常 ERP/API 创建 `DEV-E2E-PR581-20260807` 客户仓、库存、BOM、路线、工单和费用模板；保留至 Van 确认后再清理。

## RED 证据

- miniapp：旧生产入口仍为“代加工”、费用中心混入订单、代发仍有批次入口、缺少客户闭环组件和 API；定向 Vitest 首轮按预期失败。
- 后端：各子模块在实现前补充 FIFO/幂等/事务零写入、多商品 BOM 汇总、客户隔离、费用幂等与权限测试。

## GREEN 证据

- miniapp：25 个测试文件、173 项用例全部通过，覆盖入口、闭环工作区、共享解析、商品搜索、重复 SKU 合并、缺料阻断、库存批次、账单明细和能力隐藏；类型检查通过。
- ERP Vue：881 项全量测试通过，Vite 构建通过；财务菜单与视图权限覆盖“费用账单 `finance.read/write`”和“模板维护 `settings.write`”的独立入口。
- Go：`go test ./...`、`scripts/verify_kferp.sh changed`、`scripts/verify_kferp.sh backend` 及 PR-581 专项 verifier 通过。
- 真实 PostgreSQL：代发 FIFO/跨仓/并发/幂等/历史库存，生产 BOM/客户与工厂物料/WIP/预留/完工退料，账单实耗/成本/并发确认等包全部通过。
- development mp-weixin 构建通过，13 个页面、52 个清单文件完整。
- 远端发布前检查通过；Go、ERP Vue 881 项、miniapp 173 项及对应构建均为 GREEN。
- 独立合同审计发现的账单菜单/权限分层问题已完成 RED→GREEN；终审无开放 P0/P1。
- 开发环境接口冒烟通过：8 条新增路由返回预期鉴权结果，共享 `POST /api/customer-recipient/parse` 返回单个 JSON 文档；应用容器健康。

## 版本与开发部署证据

- 功能提交：`6d31cebc`；首次 develop 合并：`0f32b5e7`。
- 首次部署在旧数据库结构升级时暴露建索引顺序问题；schema hotfix `5772daa9` 合并为 `32dac811`，authentication hotfix `633d1573` 合并为 `b6bf1670`；真实完工联调暴露的客户冻结仓默认值问题由 hotfix `37def50a` 修复，并经 PR `#17` 合入最终 develop/deployment commit `b6431f01`。
- development 最终源码备份：`/opt/stacks/erp/orderapp.backup.deploy-20260807151012-b6431f013acf`；回滚镜像：`kferp-orderapp-rollback:development-20260807151012-b6431f013acf`。
- 最新固定 development 小程序包复核 52 个清单文件完整；微信开发者工具已重新打开最新包，控制台为 0 错误、0 警告，首页、发货中心、生产、库存批次、账单明细和个人中心入口隐藏视觉检查通过。
- 未合入 `main`，未部署 production，未执行微信上传、审核或发布。

## DEV-E2E 状态

- Codex 业务 DEV-E2E 只通过正常 ERP/API 执行，全部通过：
  - 地址解析成功；跨仓 FIFO 发货生成两个包裹，重复提交保持幂等，取消后正确释放预留。
  - 生产请求 `3` 进入计划 `86`、真实工单 `42`/`43` 和工序卡 `65`/`66`，在客户冻结仓完成；本次完工新增 227g 规格 4 件、454g 规格 3 件，完工后 454g 客户库存合计 5 件。
  - 中央库存批次 `FP-48` 展示生产日期和入库时间；不可追溯旧批次明确展示历史批次标志。
  - 账单 `CPB-19-00000001` 关联 2 张工单、包含 6 行、合计 `91.00`；客户自有物料费用为 `0`，重复生成保持幂等。
  - 本次地址、发货、生产、库存和账单业务写入对应操作日志齐全，无缺口。
- 以下勾选代表 Codex API DEV-E2E / 微信开发者工具视觉验收通过；Van 的最终业务确认仍待。

## Codex DEV-E2E / 视觉验收矩阵

- [x] 指定开发账号粘贴地址并提交跨仓 FIFO 发货，发货中心看到两个包裹和物流；幂等与取消释放通过。
- [x] 多商品生产申请可预览 BOM，库存足够成功进入 ERP 待排产，不足整单拦截。
- [x] ERP 排产/下达/执行/完工后，小程序工单状态和客户冻结仓成品库存同步。
- [x] 库存批次详情和“提交生产工单”预填正确；`FP-48` 日期可追溯，旧批次显示历史标志。
- [x] ERP 按费用模板生成并推送真实完工工单账单，小程序显示 2 工单、6 行、`91.00` 明细，客户自有物料未收费且生成幂等。
- [x] 未开 `product_order` 的代加工客户个人中心不显示工厂商品入口。

## 待确认与未执行范围

- 上述结果为 Codex 在 development 的 API DEV-E2E 与视觉验收；Van 业务确认仍待。
- 测试数据 `DEV-E2E-PR581-20260807` 保留至 Van 确认后再清理。
- 未合入 `main`，未部署 production，未执行微信上传、审核或发布。
