# PR-589 价格试算具体销售规格验收记录

## 范围

- 需求：`PR-589-PRICING-TRIAL-PRODUCT-SPECS`
- 分支：`codex/pricing-trial-product-specs-20260810`
- 环境：完成自动化与构建门禁后只合入 `develop` 并部署 development；`main` 和 production 不在范围内。
- 验收方式：自动化完成后由 Van 在 development 人工验收；Van 于 2026-08-10 确认验收完成。
- 数据边界：价格试算和规格选择均为只读，不新增操作日志类型，不回算或修改已发布价格表、历史订单和历史价格快照。

## DEV 合同

### DEV-589-TRIAL-SPEC-CANDIDATES

- 价格试算先按 PR-588 选择启用主商品，第二层“销售规格”只列当前父商品 active 且派生状态为空或 `active` 的具体子 SKU；`template_disabled` 和 `template_removed` 均排除。
- 候选按 SKU 身份去重，不按销售单位去重；同单位的不同 SKU 保持两项。全局单位字典和仅因可换算而兼容的单位不得加入候选。
- 默认选中父商品 `default_sku_id` 对应的有效 SKU。没有有效子规格时与价格表一致回退有效主商品自身及其权威销售单位。

### DEV-589-CONCRETE-SKU-TRIAL

- 单次试算沿用现有 `product_id + quote_unit` 请求，不新增 API 字段：`product_id` 是所选具体 SKU ID，`quote_unit` 是该 SKU 权威销售单位。
- 页面不再提供可独立选择的全局销售单位。切换主商品或客户时清空旧商品和规格；切换销售规格时保留新规格，切换价格模板时保留当前主商品和规格；上述切换均立即作废在途旧试算并清空 BOM、工艺路线和旧结果，避免旧响应串入新上下文。
- 后端继续沿用现有具体 SKU 产品读取、单位换算及 SKU → 父商品 BOM 回退。本需求不扩展为新的防伪或跨父归属校验。
- 同一具体 SKU、价格计算模板、客户、BOM、工艺路线和临时参数下，试算报价单位、BOM/工序成本口径和最终单价应与商品价格表同一具体规格一致。

### DEV-589-DOCS-DEVELOPMENT-DELIVERY

- 同步根目录与 `orderapp-remote/docs` 的需求、验收、成本操作手册、PR/DEV 种子、支持合同和本验收记录。
- 完成前端定向/全量测试、costing API、PR-589 支持合同和 Vite build 后，合入最新 `develop` 并最轻量部署 development。
- 自动化与部署完成前保持 `doing`；产品验收继续由 `REV-589-PRICING-TRIAL-PRODUCT-SPECS` 跟踪。

## TDD / API / 构建证据

- RED：前端定向合同首次报告规格 helper 不存在、旧 payload 仍提交主商品 `58` 而非 454g 具体 SKU `560`、页面缺少“销售规格”合同；PR-589 支持合同首次报告 `req_store.go` 缺少 PR/DEV 种子。
- GREEN：`node --test src/lib/product-settings.test.js` 191/191；`node --test src/api/*.test.js src/lib/*.test.js` 925/925。
- GREEN：`go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing -count=1` 通过；`go test ./internal/interfaces/http/support -count=1` 通过。
- GREEN：`npm run build` 通过（Vite 8.0.10，397 modules）；`scripts/verify_kferp.sh changed` 通过。
- 独立终审发现并修复 `template_disabled` 规格混入、无有效子规格回退、异步旧响应回填和失效类型筛选无法回到全部四个边界，复审后定向与支持合同均 GREEN。
- development 部署：功能提交 `d3ae9d29` 已通过合并提交 `4341a8eb9683f37c2d4fe6bf4978cbfdbb703816` 合入 `develop`，并使用 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development` 发布。
- 部署门禁：服务器 Vue 925/925、小程序 195/195、类型检查/development 构建、完整 Go 测试、镜像内 Go 测试、容器启动均通过；脚本内置外部 smoke 返回 HTTP 200。
- 部署备份：`/opt/stacks/erp/orderapp.backup.deploy-20260810004102-4341a8eb9683`；回滚镜像：`kferp-orderapp-rollback:development-20260810004102-4341a8eb9683`。

## 操作手册

- `docs/OP_MANUAL_COSTING.md` 已补充入口、角色、前置条件、主商品/具体规格选择、默认规格、历史回退、结果对账、异常处理、只读日志边界和历史快照边界。
- 原 PR-456/PR-585 的任意报价单位说明已按 PR-589 收敛为“由所选具体 SKU 自动确定权威销售单位”。

## Van 人工验收

- [x] 选择具有多个规格的主商品，确认只显示本商品 active 且派生状态为空或 `active` 的具体 SKU，已停用、已移除、其他商品规格和全局单位不出现。
- [x] 确认同为“盒”的两个不同 SKU 分别显示，默认选中 `default_sku_id`；切换主商品后旧规格、BOM、路线和结果不残留。
- [x] 选择无有效子规格的历史商品，确认与价格表一致回退有效主商品自身；主商品也缺少有效销售单位时明确提示维护商品档案。
- [x] 对同一具体规格使用与商品价格表相同条件，核对报价单位、BOM/工序成本和最终单价一致；切换同单位兄弟 SKU 后结果不串用。
- [x] 确认仅浏览和试算不写操作日志，不改变已发布价格表、历史订单和历史价格快照。
- 验收结论：Van accepted 2026-08-10。

## 交付状态

- `DEV-589-TRIAL-SPEC-CANDIDATES`：done。
- `DEV-589-CONCRETE-SKU-TRIAL`：done。
- `DEV-589-DOCS-DEVELOPMENT-DELIVERY`：done，自动化、构建、合入和 development 部署已完成。
- `REV-589-PRICING-TRIAL-PRODUCT-SPECS`：done，Van accepted 2026-08-10。
