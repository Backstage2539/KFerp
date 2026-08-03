# PR-573 小程序按客户当前价格表录单与生产前订单编辑验收记录

## 范围

- development 员工小程序新建、恢复草稿和编辑订单时，商品与具体规格只来自所选客户当前默认已发布价格表。
- 订单详情顶部进入编辑；编辑态固定原客户为只读，支持商品、规格、数量、成交价、运费、优惠、收件资料和订单备注。
- 只有未进入生产计划、未生成工单、未开始生产执行且未发货的正常订单可修改；后端在保存事务中重新检查状态，防止页面打开后的并发推进造成越界修改。
- 合法编辑写操作日志；编辑成功后旧销售单/发货单版本只保留历史追溯，不再作为当前订单内容。
- 只合并 `develop` 并部署 development，生成 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`；production ERP 和正式小程序包不部署。

## DEV 对照

- `DEV-573-CUSTOMER-PUBLISHED-CATALOG`：已实现并通过应用层、API 和保存事务校验。
- `DEV-573-PREPRODUCTION-ORDER-EDIT`：已实现并通过后端与小程序自动化验证。
- `DEV-573-CONCURRENT-STATE-AUDIT`：已实现订单版本、锁序、状态复核、审计和单据失效验证。
- `DEV-573-DOCS-DEVELOPMENT-DELIVERY`：需求、验收和操作手册已同步；development 集成、部署、冒烟与固定包证据已补齐。

## TDD RED 证据

- [x] 后端商品目录 RED：新增严格目录测试后，旧实现因缺少 `FilterOrderProductsForDefaultPublications`、客户参数和保存时当前发布版本校验而失败。
- [x] 后端编辑 RED：新增 PUT、状态门禁、`edit_revision`、生产/发货锁序、before/after 审计和单据失效测试后，旧实现分别出现路由/契约缺失与失败断言。
- [x] 小程序 RED：目录重载、编辑详情、阶梯自动价、草稿重校验、目录竞态、行优惠和应收口径分轮先失败再修复；主要 RED 摘要为 6 failed / 43 passed、5 failed / 32 passed、4 failed / 49 passed、2 failed / 38 passed、3 failed / 38 passed。
- RED 命令：后端 `go test ./internal/application/sales ./internal/infrastructure/postgres/sales ./internal/infrastructure/postgres/production ./internal/interfaces/http/customerportal ./internal/interfaces/http/sales -count=1`；小程序 `npm test -- --run src/api/customerPortal.test.ts src/utils/employeeOrder.test.ts src/utils/employeeOrderPage.test.ts src/utils/employeeOrderDetailPage.test.ts`。

## GREEN 证据

- [x] 共享商品目录与员工小程序 API 定向测试通过；后端对当前默认发布范围外商品/规格和伪造价格表版本拒绝提交。
- [x] 编辑 API、销售应用层和 PostgreSQL 定向测试通过；状态冲突不会产生订单头、明细、费用或单据的部分写入。
- [x] 权限测试通过：普通销售只可编辑本人范围，管理员沿用现有全范围，缺少 `orders.write` 为 403，越权订单为 404，状态冲突为 409。
- [x] 小程序 Vitest、类型检查、development 构建通过；编辑页字段、只读客户、目录加载门禁、价格与优惠口径、保存后回到详情均有自动化覆盖。
- [x] `scripts/verify_kferp.sh changed`、`git diff --check`、PR/DEV 支持合同和受影响完整套件通过。
- GREEN（2026-08-02 23:44 CST）：Go `go test ./... -count=1` 全包通过；受影响包 `go vet` 通过；小程序 21 files / 147 tests、`vue-tsc --noEmit`、development build 通过；Vue shell 854 tests 和 Vite build 通过；后端与前端独立只读复核均无开放 P0-P2。

## 价格表目录验收

- 开发数据前提：记录测试客户、当前默认已发布价格表及其包含的商品/规格；不在证据中保留无关客户隐私。
- “红岩”复现：商品档案保持启用，但当前默认已发布价格表不包含“红岩”时，ERP 与小程序新建订单均不可选；小程序搜索“红岩”无结果。
- 历史/归档/非默认发布：只在这些版本中的商品和规格不出现在新建或编辑候选，直接提交 ID 被拒绝且订单不变。
- 客户切换与草稿：切换客户和恢复草稿分别按对应客户重新加载目录，加载中不能使用旧客户候选，过期商品行必须修正。
- 证据：严格目录/保存校验自动化、客户切换与草稿竞态 Vitest 已通过；development 未登录 order-form 返回 401，证明新接口继续受鉴权保护；“红岩”与 ERP 同客户目录一致性保留给 Van 使用登录态人工验收，不在发布过程中写入测试业务数据。

## 编辑状态与并发矩阵

- [x] 未进入生产计划 + 无工单 + 无执行 + 未发货：允许保存。
- [x] 已进入生产计划：拒绝保存且订单不变。
- [x] 已生成生产工单：拒绝保存且订单不变。
- [x] 已开始或完成生产执行：拒绝保存且订单不变。
- [x] 已发货：拒绝保存且订单不变。
- [x] 页面打开后另一端推进上述任一状态，再保存：版本或状态冲突，不产生部分写入。
- 证据：应用层状态矩阵、PostgreSQL editability/revision、生产批次锁序、发货 revision 与 HTTP 409 测试通过；development 容器已运行且接口鉴权正常，具体订单状态由 Van 按下方清单验收。

## 权限、操作日志与单据版本

- [x] 详情的可编辑提示与保存接口使用同一订单范围；越权和缺写权限请求不调用保存。
- [x] 合法编辑的操作日志包含员工、订单和业务 before/after 摘要；目录、权限和状态校验失败不写成功编辑日志。
- [x] 编辑成功后详情与列表读取新订单内容；旧销售单/发货单及合并单据版本不再标记为当前。
- [x] 再次导出从保存后的订单读取；发货 Excel 以订单 revision 锁后复核、唯一文件原子落盘，失败不会覆盖或删除历史文件。
- 证据：权限/API、事务审计、单据失效、发货 revision/唯一文件/错误清理自动化通过；发布过程未写订单，实际操作日志和重导出文件由 Van 编辑测试订单后验收。

## 部署与冒烟

- 功能分支：`codex/mini-order-catalog-edit-20260802`。
- 功能提交：`3a8e2459410517acda16f5078b92e78cf4c51319`。
- `origin/develop` 集成提交：`0274ee9edd2a2b3831881d20cfb6bf2fe11f26c3`。
- development 预检/部署：`./deploy_orderapp.sh development` 通过；`RELEASE_INFO` 提交与环境一致；`erp_orderapp` 为 `running`、restart count `0`、image `sha256:fc67236f407328c99d08b50e320637352bb330b45766beb9a14d7ecf25a55f0e`；登录 HTTP 200、未登录 mini order-form HTTP 401，近 10 分钟错误关键词扫描无结果。
- 回滚证据：previous source `/opt/stacks/erp/orderapp.backup.deploy-20260803000425-0274ee9edd2a`；rollback image `kferp-orderapp-rollback:development-20260803000425-0274ee9edd2a`。
- 开发小程序固定包：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`；`RELEASE_INFO` 为 development、API base `https://dev.qacoohee.com/app`、提交 `0274ee9e`；`PAGE_FILE_MANIFEST` 52 项复验通过；上一版保留于 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev.backup-20260803001008-0274ee9edd2a`。
- production：未部署。`erp_prod_orderapp` 仍为 2026-08-01 启动的原实例、restart count `0`、image `sha256:33ef3df6a633dccbbfe4dd96a5d2e5a2cd17d47ef270e020c8adced212de7cc6`；生产应用、数据库和正式小程序固定包未切换。

## Van 验收清单

- [ ] 开发版小程序新建订单选择测试客户后，搜索不到未进入当前默认发布的“红岩”，可售商品/规格与 ERP 同客户录单一致。
- [ ] 打开尚未进入生产和发货的本人订单，详情顶部可进入编辑；客户只读，商品、规格、数量、成交价、运费、优惠、收件和备注均可保存并正确回显。
- [ ] 打开已进入生产计划/工单/执行或已发货订单，不能编辑；页面打开后由另一端推进状态再保存，也会明确提示冲突且订单不变。
- [ ] 编辑前的旧单据不再标为当前，重新导出的销售单/发货单内容与编辑后订单一致。
- [ ] 管理员和普通销售的订单范围正确，操作日志能查到合法编辑记录。
