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

本记录证明代码、自动测试、隔离浏览器交互及下列环境发布核验。真实工单入库及 Van 业务验收仍待现场完成。

## 环境发布与只读核验（2026-10-24）

- 开发环境：PR #283 合入 develop，部署版本 `3a2169997fba68e9ab6322adf447d7aa66ac8b64`，运行状态和认证页面检查通过。
- 生产环境：PR #285 从最新 main 基线集成，发布提交 `7883f1aa3beb0f5ab76646758f8b88075484d168`；候选预检与正式发布均通过。正式命令为 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production`。
- 正式门禁：Vue 1,569/1,569、Vue 生产构建、小程序 432 项测试/类型检查/生产编译与 24 页清单、服务器 Go 全包测试、镜像内 Go 全包测试和应用编译均通过。`pdfjs-dist` 在 Node 20 下有引擎警告，未阻断测试或构建。
- 发布前发现根盘满、PostgreSQL 恢复因无空间失败。按 Van 授权清理 Docker/Go/npm/系统软件包缓存及七天以前的归档系统日志；保留数据库、业务数据、操作日志、源码备份及回滚镜像。数据库恢复 healthy，完工列表 API 恢复 HTTP 200。正式发布前可用空间约 6.0 GB，发布后再清理构建缓存，最终约 5.8 GB 可用。
- 运行态：`erp_prod_orderapp` running、重启计数 0；`erp_prod_postgres` healthy。外网 `/app/login` 返回 200，匿名 `/app/` 返回 401，认证完工页面及列表/仓库/PR/DEV API 返回 200。
- 生产运行镜像：`sha256:cc4e9622ae2f4490d974660783df17504a221111beb5593c3398d040d961542f`。
- 实际 HTTP 提供的 `ProductionAcceptanceView-jlh6KW8o.js` 与运行容器文件 SHA-256 相同：`43549e991e496e3c94e01248875e529395c5cd40690a247401a2e9dec048d197`；包含合并核对、自动来源备注等文案。运行容器生产手册包含合并入库说明。
- 生产只读列表共 10 张任务，按页面状态和身份分组为 10 组，仓库名称全部解析成功。当前生产数据没有多工单同组样本，因此生产核验未执行真实合并提交；多工单合组及部分失败继续由前述隔离 fixture 和自动测试覆盖。
- 发布前后按工单排序的 `work_order_id / reported_qty / received_qty / available_qty / receipt_status / target_warehouse` 投影完全一致，SHA-256 为 `26933b427df9bafc8402494e1a63c0bdffc2736277f876533871cd028e9be304`。
- READ ONLY 数据库快照一致：完工入库单 2 张、最大 ID 50；明细 2 条、`qty_g` 合计 99,500、`qty_units` 合计 0；完工 POST 操作日志 10 条、成功 2 条、最大 ID 240633。GET 访问日志可能正常增加，不作为业务写入判断依据。
- 进度 API：PR-725 为 review；DEV-858 至 DEV-861 均为 done，保留 Van 业务验收状态。
- 原始部署日志和只读快照留在执行机 `/private/tmp/pr725-production-evidence-20261024/`；本文件保留可持久复核的摘要。

### 回滚锚点

- 旧源码：`/opt/stacks/erp-production/orderapp.backup.deploy-20261024084152-7883f1aa3beb`。
- 回滚镜像：`kferp-orderapp-rollback:production-20261024084152-7883f1aa3beb`，已验证仍指向原运行镜像 `sha256:d06374e799f84e54c7659660adbb783ece87a65cc08ebed3ab351b80a9be4142`。
- 本次未上传、审核或发布微信小程序，也未执行真实完工入库。
