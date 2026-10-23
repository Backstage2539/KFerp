# PR-725 完工入库合并与仓库名称验收记录

## 范围

完工入库按产出身份、规格、单位、货主和工单冻结目标仓分组。一次操作按来源工单顺序调用既有完工接口，每张工单继续走自己的库存、成本和操作日志链路；备注自动记入来源工单号。目标仓使用仓库 API 返回的名称展示，入库仍使用工单冻结仓库。没有增加库存写入 API，也没有改动历史单据。

## TDD 与自动化

- RED：`/tmp/pr725-go-red.log` 因 `FinishedReceiptTask` 尚未暴露产出身份与客户字段而编译失败；`/tmp/pr725-vue-red.log` 因分组 helper 尚未创建而导入失败。
- GREEN：`node --test frontend-vue-shell/src/lib/production-acceptance.test.js`，5/5 通过，覆盖同产出合组、规格/单位/货主/目标仓分组、无稳定产出身份时不按同名误合并、来源备注、质检阻塞排除、仓库名称映射和单笔失败后继续。
- GREEN：`go test ./internal/application/production -run '^TestFinishedReceipt(TasksExposeFrozenOutputAndWarehouseIdentity|DisplayQuantityUsesOutputUnit)$' -count=1` 通过。
- GREEN：`go test ./internal/interfaces/http/production -run '^TestFinishedReceiptAPIExposesOutputAndWarehouseGroupingIdentity$' -count=1` 通过。
- GREEN：`scripts/verify_kferp.sh backend` 全部 Go 包通过；`scripts/verify_kferp.sh frontend-tests` 1,569 项通过；`scripts/verify_kferp.sh frontend-build` 构建通过。
- GREEN：`scripts/verify_kferp.sh changed` 通过，包含补丁空白与冲突标记检查。

## 浏览器场景

用临时隔离浏览器 fixture（所有 API 均为本地 mock）检查 3 张工单：两张相同商品、454g 规格、同一客户和虚拟仓合成一组；目标仓不同的同规格工单保持单独一组。页面显示“客户甲成品仓”，不显示 `virtual-*` 仓库码。合并表单显示合计 15 袋及来源工单号。

模拟第一张工单成功并返回 `SE-0000000057`、第二张工单返回“本次完成件数不能超过工单剩余件数 1”后，页面显示成功 1 袋及关联单据，同时明确列出失败工单和原因；处理继续到后续来源。fixture 未连接开发或生产数据库，未写入任何业务库存。

## 技术边界与验收状态

超工单剩余数量仍由原完工接口拒绝，合并入口不会绕过该保护。单工单失败会保留失败原因并显示已完成部分；现场如遇该提示，应打开对应工单核对计划剩余量和末道报工数。

本记录证明代码、自动测试与隔离浏览器交互。生产部署、真实工单入库及 Van 业务验收不在本次交付范围。

## Development 发布与运行态核验

- PR #283 已合并到 `develop`，运行版本为 `3a2169997fba68e9ab6322adf447d7aa66ac8b64`。发布前隔离预检及正式部署脚本中的 Vue、小程序、Go 全包测试和 Docker 镜像构建均通过。
- 执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`；`erp_orderapp` 正常运行，`erp_postgres` healthy。登录页及已认证的完工入库 Vue 路由返回 HTTP 200；匿名 `/app/` 按现有行为重定向到 `/app/orders`（303），后者重定向到登录页（302）。
- 运行镜像：`sha256:b32989e9a363280fa32e0856d707de8ea26bbb8f3b3379e8a56b46f8b4751800`。部署前备份：`/opt/stacks/erp/orderapp.backup.deploy-20261023232357-3a2169997fba`；回滚镜像：`kferp-orderapp-rollback:development-20261023232357-3a2169997fba`。
- 已认证的需求进度 API 返回 PR-725 状态 `review`，DEV-858 至 DEV-861 均为 `done`。开发服务器上的生产操作手册含 PR-725 使用说明，部署 JS 包含仓库名称未加载时的安全提示。
- 该次发布不导出或上传微信小程序版本（服务器现存包仅随部署保留）；未触碰生产环境，未执行真实完工入库或其他库存写入。Van 现场业务验收仍待完成。
