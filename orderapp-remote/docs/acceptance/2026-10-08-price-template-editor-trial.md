# PR-686 价格模板编辑界面试算

## 需求和验收口径

在商品价格表的平铺价格行打开“编辑价格模板”时，可选择当前表已选商品与规格，并用未保存的完整模板参数自动试算。试算只读，不写模板、价格行或已发布快照；只有点击保存才更新全局模板和按当前行为重算受影响的草稿价格行。

## 自动验证

- RED：服务端忽略 `pricing_rule_draft`；前端选品、身份去重、防抖与迟到响应隔离未实现。
- Go/API：`./scripts/verify_kferp.sh all` 中 Go 全量测试通过；包括完整草稿替换、零值/移除成本、无写入、与保存配置结果一致、模板校验复用、草稿和旧 overrides 冲突、批量接口拒绝草稿及单次 API 字段绑定。
- 前端：`./scripts/verify_kferp.sh frontend-tests`，1337 项通过；包含商品/规格候选、阶梯去重、同名不同规格、客户与 BOM 身份、250ms 自动试算、防抖、重试、切换上下文和迟到响应隔离。
- 构建与工作区：`./scripts/verify_kferp.sh frontend-build` 通过；`git diff --check` 通过。构建报告既有的大资源包提示。
- 无业务写入：试算使用既有只读成本计算路径；服务层回归断言试算前后模板数据不变。

## 手册和业务边界

- 用户操作说明：[成本核价手册](../OP_MANUAL_COSTING.md#在价格模板编辑中试算-pr-686)，并已登记在 `OPERATION_MANUALS.md`。
- 保存模板仍通过原价格模板更新 API；用户触发的模板保存保留原有操作日志。
- 固定规格价、人工覆盖和已发布快照不被编辑器试算覆盖。

## 开发交付与人工验收

- PR #188 已合入 `develop`，部署版本为 `57c5b29970c1630f644010067bb6c30bb8276300`。
- 开发环境部署于 2026-10-09 完成。服务器完整 Go 测试、Vue 测试与构建、小程序 43 个测试（267 项）、类型检查及微信小程序开发构建均通过；`erp_orderapp` 和 PostgreSQL 正在运行，PostgreSQL 健康，`https://dev.qacoohee.com/app/login` 返回 HTTP 200。
- 服务器备份：`/opt/stacks/erp/orderapp.backup.deploy-20261009004710-57c5b29970c1`；回滚镜像：`kferp-orderapp-rollback:development-20261009004710-57c5b29970c1`。
- 微信小程序产物只生成在开发环境，未上传或发布；生产环境未纳入此需求。
- 待 Van 在开发环境人工确认：编辑界面选品、未保存参数变化后的试算结果，以及保存后平铺自动价格行重算。人工业务验收仍待确认。
- 开发 PR/DEV 进度见 `ACTIVE_REQUIREMENTS.md`；PR-686 已转为 review，DEV-748 已完成。
