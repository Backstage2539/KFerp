# PR-727 默认规格仅用于初始选择和标识

## 实现口径

- 默认规格只用于首次选品和当前默认标签；默认切换及同一有效规格 BOM 换版不再产生 `default_changed` / `default_bom_changed` 异常。
- 按商品与规格身份保留选择、同步当前已发布 BOM，恢复旧草稿和连续刷新会清除过时异常。保存草稿和发布均处理全部命名表，包括尚未打开的表。
- 固定价、手工价和客户沿用报价保留。沿用报价按稳定商品/规格匹配，当前执行身份与原报价成本来源分别保留；不改历史发布版或订单。
- 后端只校验实际所选规格，不再依赖历史默认规格存在或归属；当前规格身份、有效性、父商品、价格行一致性及完整性校验继续生效。无接口结构或数据库迁移。

## TDD 与验证

- RED：前端4项失败，覆盖默认切换、BOM身份更新、刷新残留异常及沿用报价丢失；HTTP4个子用例因历史默认规格为空/不存在而返回400；应用层新增回归同样先失败。
- 补充 RED：未打开的命名表保存仍提交 BOM 423 而非424。复用逐表恢复流程后，保存和发布均通过，活动表及各表报价保持。
- GREEN：全量 Vue 1,578项通过；`scripts/verify_kferp.sh backend` 全包通过；成本应用层和HTTP完整包通过；Vue生产构建、`git diff --check`、changed检查通过。
- HTTP 使用实际 handler + costing service + 隔离内存仓库：保存/发布均保留82元，允许历史默认规格缺失；拒绝跨商品规格、过期提交身份、缺价格及行身份不一致，不写入仓库。
- DOM测试挂载实际规格区模板与真实选择函数：227g显示默认但未勾选，454g原选择保留；可继续多选；实际规格移除后仍显示处理提示。
- 本地浏览器使用真实规格区模板、样式和选择函数及模拟数据验证视觉，未连接业务API。截图及日志保存在工作区旁 `../evidence/default-spec/`；不将局部预览等同于整页真实业务发布验收。

## 手册与交付

- 手册：`docs/OP_MANUAL_COSTING.md` 的刷新流程和规格选择说明，现有 Vue `costingManual` 入口读取同一源文件。
- PR-727；DEV-866（选择/报价）、DEV-867（校验）、DEV-868（交付）。PR保留 review，业务验收由Van完成。
- 目标环境：development。生产环境及真实价格表发布不在本次范围。

## 开发环境交付凭据

- 功能分支：`codex/default-spec-initial-only`；已推送功能提交 `face99a8eee2bbc86497129ec31975e33ef6f075`。
- GitHub PR：[#291](https://github.com/Backstage2539/KFerp/pull/291)，合并到 `develop`；应用部署提交 `e7ae6c61b4d43be327016d219d12d3521542931d`。
- 从独立干净副本 `/private/tmp/kferp-pr727-dev-release` 执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`。服务器 Vue 1,578项、小程序432项、类型检查、构建、Go全包及镜像内Go检查通过；部署退出码0。
- 源码回滚点：`/opt/stacks/erp/orderapp.backup.deploy-20261024114208-e7ae6c61b4d4`。
- 镜像回滚点：`kferp-orderapp-rollback:development-20261024114208-e7ae6c61b4d4`。
- `erp_orderapp` running / restarts0；`erp_postgres` healthy。登录页200，匿名 `/app/`303且受保护需求API401；运行容器认证后的成本页、需求API和源手册均200，手册含更新说明。
- `CostingView-Dpw18Pii.js` 的服务器构建、运行容器及公网响应内容一致，SHA256 `1c56d4ed21ef1cfd7fc5790b3a6fbed2ba410d4f7200ec700b92b6bd41f53578`；已无默认变更提醒文案，保留默认规格标签。
- 开发环境PR-727保持 `review`，DEV-866/867/868均为 `done`；DEV-868交付状态变更写入 `audit_logs` 与 `operation_logs`。最终回执只更新文档及需求种子状态，不改业务实现，运行应用仍为上述部署提交。
- 日志：工作区旁 `../evidence/default-spec/development-deploy.log`、`development-smoke.log`、`delivery-status.log`，以及RED/GREEN日志和隔离预览截图。
- 未执行真实价格表保存/发布、生产部署或微信上传/发布；Van业务验收待确认。原工作区已有改动保留。
