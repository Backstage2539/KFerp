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
- 首次交付目标为 development；后续按 Van 追加授权发布生产环境，凭据见下文。真实价格表发布仍由业务人员执行。

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
- 开发交付时未执行真实价格表保存/发布、生产部署或微信上传/发布；Van业务验收待确认。原工作区已有改动保留。


## 生产发布范围（2026-10-24 追加授权）

- Van 后续明确要求合并到 main 并部署生产环境。生产集成分支 `codex/default-spec-production` 从 `origin/main@20d886f2` 创建，只合入 PR-727 功能及交付记录，保留 main 上生产专属修复和其他需求的验收状态。
- 本次生产技术验证使用只读页面、GET/API、手册及静态资源身份；不保存、重新计算或发布真实价格表，不修改历史订单。
- 生产发布已完成，技术凭据如下；业务验收仍由 Van 完成。

## 生产交付凭据

- 集成分支：`codex/default-spec-production`；已推送候选 `d63222bd22855ebaf0111eaed965a8bc81144ba4`。GitHub [PR #293](https://github.com/Backstage2539/KFerp/pull/293) 合并到 `main`，部署提交 `612dffd0894fb39c2bc2d305d71ad13269cb9769`。集成差异仅含 PR-727 的22个文件；main已有生产修复和其他需求验收状态保留。
- 干净发布副本 `/private/tmp/kferp-pr727-prod-release` 与 `origin/main` 完全一致，执行 `KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production`，退出码0。候选全量Go、Vue1578、生产构建和changed检查通过；服务器Vue1578、小程序432、类型/正式环境构建、全量Go及镜像内Go再次通过。
- 部署前在发布锁内清理可再生Go/npm/未使用Docker构建缓存，可用空间从3.3GB提高到4.1GB；数据库、源码备份和回滚镜像未清理。
- 生产源码回滚点：`/opt/stacks/erp-production/orderapp.backup.deploy-20261024124341-612dffd0894f`。
- 生产回滚镜像：`kferp-orderapp-rollback:production-20261024124341-612dffd0894f`，镜像ID `sha256:f01a1c99d231cf73bc6de3f6efb6f4f68ed4ec9c57faafa3ad9423c22e438041`；已确认源码目录与镜像实际存在。
- 新应用镜像：`sha256:c38aded0bafc7368fceebd554f6a780ea0e8fbb64cadce0c18c235c8211953df`。`erp_prod_orderapp` running/restarts0，生产数据库healthy，累计重启次数仍为发布前的13；开发应用仍运行原镜像 `a280c9e826d7`。
- 公网登录页200；匿名 `/app/`及受保护需求API401；从运行容器读取认证配置后，根入口303，成本页面、成本设置、价格表发布列表摘要、手册及需求API均200。PR-727保持review，DEV-866/867/868均done。
- 公网、容器和服务器构建的 `CostingView-Dpw18Pii.js` 内容一致，SHA256 `1c56d4ed21ef1cfd7fc5790b3a6fbed2ba410d4f7200ec700b92b6bd41f53578`；无旧默认变更提醒文案，保留默认规格标签。
- 发布前后只读快照一致：179条价格表发布记录、108条订单、190条订单明细，以及两张成本试算表的记录数和完整行内容哈希均未变化。未执行真实价格表保存、改价、发布或订单写入。
- 服务器内小程序产物 `RELEASE_INFO` 为production/上述提交/正式API地址，24个声明页面与96个清单文件均完整。本次未导出到本机固定目录，未执行微信上传、审核或发布。
- 证据保存在工作区旁 `../evidence/default-spec/production-backend.log`、`production-frontend.log`、`production-deploy.log`、`production-smoke.log`、`production-before.json`、`production-after.json`和`production-snapshot-comparison.txt`。后续回执提交只更新文档，运行应用仍为 `612dffd0`。
