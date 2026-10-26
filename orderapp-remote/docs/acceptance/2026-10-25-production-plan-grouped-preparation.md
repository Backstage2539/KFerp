# PR-732 生产计划按规格合组并就近展示备料

## 状态

实现与本地验证、development 部署完成；Van 业务验收待办。最新 `origin/develop` `d9b6db27f5ae64fe14f0173490533ebd92b1cf32` 已合入功能分支，并在 `fd9c0abfd55578988efb81a4394677fee483d0a7` 合入 `develop`。Van 已明确授权当前 develop（包含 PR-731 尚待隔离数据/浏览器验收的变更）部署到 development。

生产计划 PP-0000000017 只作为用户提供的展示参考。页面验证使用与示例数量一致的隔离/本地演示数据，不保存、刷新供应、提交或操作任何生产工单。

## 实现

- 详情改为阶段 → 配置兼容的同规格生产组 → 本组备料摘要 → 可展开的原订单任务。
- 仅按输出类型与对象身份、冻结规格/单位换算、BOM/工艺/生产配置快照和目标仓合组；不使用客户名称或商品名称作为唯一键。
- 通过 `component_sources.production_plan_item_id` 将后端备料结果关联到原任务。组内汇总需求、WIP、待领、上游供给和真实缺口；克/千克按后端克数合计，计件单位独立合计。来源仓、货主、批次和原任务分配仍保留在各自任务明细中。
- 单任务组直接显示现有操作；多任务组折叠订单任务及其修改、领料和重排操作。核对项展开并滚动至所属任务/来源。
- 旧版来源控件仍位于原任务；无法关联来源及无任务归属的旧版总表明确显示“未关联备料”。无新 API 字段、迁移或生产写操作。

## 验证证据

- TDD：新增任务合组/备料汇总测试时，未实现的分组函数触发 RED；实现后目标用例通过。增加克数为 0、件数为正的来源用例，避免包装袋件数丢失。
- 前端：集成最新 develop 后重跑 `./scripts/verify_kferp.sh frontend`，Vue shell 全量测试和 Vite production build 通过；构建仅报告现有大 chunk 提示。
- 后端：`./scripts/verify_kferp.sh backend` 通过，Go 全量包测试通过。
- API：在专用临时 PostgreSQL 实例和测试 schema 中运行 `TestProductionPlanAPICreatesMaterialShortageWorkOrderAndCompletesIntoDownstreamWIP`，通过。GET 详情验证生豆来源关联自制熟豆任务、熟豆和包装袋来源关联包装任务；测试创建/完工仅在临时 schema 中发生。
- 页面：Vue SSR 用例覆盖跨订单合组、规格/BOM/工艺/单位换算/目标仓不兼容隔离、需求/WIP/待领/上游/缺口汇总、货主分配保留、共享上游、未关联来源及旧版来源控件。核对项展开行为在本地预览中实际点击验证。
- 布局：本地组件预览检查桌面 1440×1100 和窄屏 390×844；窄屏 `documentElement.scrollWidth` 为 390px，无横向溢出；备料数字使用两列布局，底部操作栏位于内容之后，不遮挡摘要。
- 集成最新 `origin/develop` `d9b6db27f5ae64fe14f0173490533ebd92b1cf32` 后，`./scripts/verify_kferp.sh changed` 通过；同一合并结果上的隔离 PostgreSQL 目标 API 测试通过。

## 未通过的宽范围回归

另运行 `ORDERAPP_TEST_DATABASE_URL=… go test ./internal/interfaces/http/production ./internal/application/production ./internal/infrastructure/postgres/production -count=1`。目标 API 测试通过；完整 Go 包回归未全绿，其他既有用例报告：测试 schema 缺 `product_bom_spec_authorities`、父 BOM 来源缺 505g、PR-655 工位人员 fixture 使用旧配置、客户加工预留缺 600g 物料批次。没有为本次展示改动调整这些独立行为。

## Development 部署与运行核验

- 部署命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`；部署完成版本 `fd9c0abfd55578988efb81a4394677fee483d0a7`。
- 回滚锚点：旧源码 `/opt/stacks/erp/orderapp.backup.deploy-20261026020546-fd9c0abfd555`；镜像 `kferp-orderapp-rollback:development-20261026020546-fd9c0abfd555`。
- 容器：`erp_orderapp` 运行中，镜像 ID `sha256:f09725aba267279adc2984998fbd24a81f9f133df5a365942cd7dc7518604baa`；`erp_postgres` healthy，`erp_docconvert` running。
- Smoke：部署脚本外部 `/app/login` 返回 200；`/app/login`、`/app/vue-shell` 返回 200；`/app/` 返回 303 并跳转 `orders`。匿名 requirement API 返回 401，BasicAuth 请求返回 200；PR-732 requirement row、手册 marker 与前端 JS 分组 marker 可读。
- 此次发布包含 PR-731 内容，依据 Van 的明确授权；PR-731 的隔离数据/浏览器验收仍未完成，继续由其流程跟进。development 产品验收待 Van 完成。
- 本次未发布微信小程序，未部署 production，也未写入或刷新生产计划 PP-0000000017。
