# PR-693 可复用规格换算组与临时换规格下单

2026-10-16：实现、自动化验证、开发分支集成、开发部署和小程序开发包交付均完成。Van 业务验收、真机验收、生产和微信上传／审核／发布不属于已完成项。

## 需求与实现

- 功能分支 `codex/spec-conversion-orders-20261015`，从当时最新 `develop@25b377af` 创建独立工作树；集成前合入已完成开发部署的样品功能 `origin/develop@b2f7d6fe`，保留两边订单编辑行为。初始编号与并行任务冲突后改为 PR-693，DEV-771～774。
- 独立 `spec_conversion_groups` 保存人工关系和修订，复用单位字典。关系是具体 BOM 规格 ID，或规格模板 ID＋模板内规格键＋库存单位；不按名称、克数、磅常量推断。管理接口检查当前有效规格关联、冲突和单位。
- 来源采用已发布价格记录的精确规格／版本／档位键；来源原 BOM 版本可为历史版本。目标仅来自同商品当前默认已发布 BOM。按有理数计算并四舍五入到分，再乘件数；不自动换档，不写价格表。
- ERP 商品管理维护；ERP 录单抽屉可快捷维护；员工小程序选用已配关系。两端只读报价，隔离迟到报价及配置加载响应，改变客户／商品／价格来源清理选择。
- 保存订单复用后台报价服务。历史冻结只由服务器原订单行读取，不能信任客户端快照。只改数量不重算；明确重新计价、改变规格／来源、复制则执行现行校验。已验证的历史行就地更新数量，保持原 BOM 版本身份，避免归档版本的插入校验阻断正常历史编辑。
- 客户接口和应用层拒绝 `converted_tier`／换算参数，配置和订单写入保留操作日志。目标规格占用原订单 BOM 字段，来源规格仅在价格快照中。

## RED／GREEN 证据

- 领域测试最初 `undefined Convert/Group`；实现明确比例与身份匹配后通过。覆盖 48 × 0.5 / 0.454 = 52.86，48 × 0.25 / 0.454 = 26.43，方向、半分边界、非重量关系和非法比例。
- 仓储与接口测试先因缺少换算报价函数／参数映射编译失败；共用服务及请求映射完成后通过。
- Vue／小程序辅助逻辑测试先因缺少模块失败；目标规格与来源参数分离、编辑冻结和数量不换档完成后通过。小程序 RED 日志归档为 `kferp-conversion-mini-red.log`。
- 真实数据库测试暴露了乘法浮点尾差，以及归档目标 BOM 的历史行删除重建被触发器拒绝；金额改为有理数分位乘法，原历史行采用受校验的原地更新，GREEN。
- 合并最新开发分支后，旧测试把新建表单当作编辑表单，错误期望保留旧行 ID；补充明确 edit_id，同时验证复制／新建清空旧行 ID。完整检查通过。

## 验证覆盖

| 要求 | 证据与结果 |
|---|---|
| 规格身份、配置错误、来源单位 | `domain/specconversion/conversion_test.go`、`postgres/sales/spec_conversion_test.go`；匹配模板身份／具体 ID，零负缺失、同名未关联、重复／冲突、单位不明均拒绝 |
| ERP 报价与保存 | `interfaces/http/sales/spec_conversion_postgres_test.go` 隔离 PostgreSQL＋真实 HTTP／仓储；500g 单价 52.86，小计 105.72；250g 26.43；普通 454g ×48 与两个换算规格混合总额 2409.72 |
| 权限与伪造 | 跨商品档位、跨客户价格表、未关联目标、无效档位／数量拒绝；提交 0.01 不影响后台成交价；客户拒绝换算；客户端历史快照不能改变冻结价格 |
| 历史／重新计价／复制 | 配置改变后数量 3 仍 52.86，总额 158.58；主动重算与复制按新比例 63.44；配置停用、价格表下线、目标版本归档后原行数量 4 仍 63.44，总额 253.76；新副本失效拒绝 |
| 修订与来源版本 | 旧修订返回 409；来源原 BOM 版本归档但发布价仍有效可换算，目标需使用当前版本 |
| 价格表与履约 | 保存前后表数量及来源单价 48 不变；订单行持有目标 9001／9003；销售单预览与库存批次预览返回 500g／目标 BOM 身份；生产需求单测证明来源 454g 快照不覆盖目标，500g／250g 分组独立 |
| 员工小程序合同 | `mini_spec_conversion_test.go`：员工可预览，客户与缺权限员工 403；创建传递目标与来源独立身份、忽略伪造单价；编辑可信历史来源、重新计价拒绝失效来源 |
| 前端行为 | Vue `spec-conversion.test.js`、订单 helper；miniapp `specConversion.test.ts`；数量保留选档、没有当前目录时历史仍能编辑、复制重算、恢复普通规格范围、迟到响应隔离 |
| 其他订单回归 | 合并后的 `TestSamplesPostgres*` 四组数据库测试通过，样品零价／收费编辑继续有效；普通订单不借用样品或冻结例外 |
| 本地布局 | 真实 Vue 组件、独立示例数据，375px 抽屉核对 500g／250g 切换、公式、金额和应用；管理页人工关联与单位字段可用。没有操作共享环境业务数据 |

## 检查结果

- `scripts/verify_kferp.sh all`：全部 Go 包、Vue 1365 项测试及 Vite 构建通过。
- `ORDERAPP_TEST_DATABASE_URL='postgres:///postgres?host=/tmp' go test ./internal/interfaces/http/sales -run TestConversion -count=1`：真实 PostgreSQL 换算 HTTP／保存／历史链路通过。
- 同环境 `go test ./internal/appmain -run '^TestSamplesPostgres' -count=1 -timeout=3m`：四组已有样品事务回归通过。
- `npm test`（miniapp）：53 个文件、326 项通过；`npm run typecheck`、`npm run build:mp-weixin:development` 通过。
- `git diff --check`：通过。集成前自审包括服务端权限、历史行可信来源、修订锁、目标身份、来源单位、合并重复 DTO 字段和异步失效处理。

可选的全库 PostgreSQL 套件未宣称全绿：早期广跑有 84 个失败，已在原始干净基线 `25b377af` 复现完全相同的失败集合，主要是旧测试缺少订单字段／旧 SQL 列数／文档版本预期。本需求新数据库测试和相关样品回归均独立通过。普通 `go test ./...` 通过时，不提供数据库连接的旧集成测试按既有规则跳过。

## 手册、证据与交付范围

- 独立手册 `docs/OP_MANUAL_SPEC_CONVERSION.md`；同步订单销售、员工小程序手册、操作手册索引及 Vue `specConversionManual` 帮助入口。
- PR-693 保持 review；DEV-771～774 记录实现与技术验证，不代替 Van 产品验收。
- 交付目录：`/Users/yiiiple-work/KFerp-spec-conversion-delivery-20261016/`，包含日志、组件截图和 `KFerp-miniapp-mp-weixin-dev` 开发包。
- 没有创建实际业务换算组、修改正式价格表或执行真实客户录单验收。生产部署及微信上传／审核／发布未执行。

### 首次部署门禁修正

首次服务器构建在切换服务前，需求记录回归测试发现批量更新进度时误改了其他 CONVERSION 名称的旧需求证据。已按完整需求 code 精确恢复其他记录，只保留 PR-693／DEV-771～774 的进度；重新执行全量检查后继续部署。首次尝试没有替换开发应用或修改业务数据。

## 开发交付回执

- 功能提交 `34a8c876` 经 PR #202 合入 develop；修正提交 `f7bdd239` 经 PR #203 合入。实际运行提交：`a7a0f910f25a087eab2f8306d151a15a5225198f`。
- 部署命令：`KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/Users/yiiiple-work/KFerp-spec-conversion-delivery-20261016/KFerp-miniapp-mp-weixin-dev ./deploy_orderapp.sh development`，在独立、干净的 develop 集成目录执行。
- 源码备份：`/opt/stacks/erp/orderapp.backup.deploy-20261016122357-a7a0f910f25a`；回滚镜像：`kferp-orderapp-rollback:development-20261016122357-a7a0f910f25a`。
- 第二次尝试因镜像仓库 502 在应用切换前停止；本机从 Docker 官方源获取同版本 linux/amd64 基础镜像，传入服务器并验证缓存构建后，原发布脚本完整重跑成功。未绕过测试或发布门禁。镜像为 golang:1.22-alpine（平台摘要 `sha256:6d405dfc5fdf3a45df1529cf060b920041f52ce523487e0f36f02765af294a51`）和 alpine:3.20（平台摘要 `sha256:c64c687cbea9300178b30c95835354e34c4e4febc4badfe27102879de0483b5e`）。
- 服务器 Vue、小程序、Go 全包、镜像内 Go 测试和编译均通过。应用 running、0 次重启，PostgreSQL healthy；外部登录页 HTTPS 200。认证商品配置页、换算管理 API、独立手册均 200；未认证配置 API 和员工小程序接口均 401。根路径沿用 303 重定向。
- 配置接口首次读取为 0 个换算组、47 个有效规格、6 个单位。未自动猜测或创建实际业务关系；首次使用在「商品管理 → 规格换算」人工配置，然后从员工录单行进入「临时换规格」。
- 运行需求表：PR-693 review，DEV-771～774 done，Van 产品验收待进行。
- 开发包：`/Users/yiiiple-work/KFerp-spec-conversion-delivery-20261016/KFerp-miniapp-mp-weixin-dev`。RELEASE_INFO 提交与运行版本一致，环境 development、API 为 `https://dev.qacoohee.com/app`；24 个页面、96 个清单文件验证通过，包含换规格组件。
- 交付目录保存 deployment.log、两次未完成尝试日志、base-image-transfer.log、runtime-check.log、自动化测试日志和 quote-250g.png。本地组件截图只作示例，无共享业务数据操作。

本回执为部署后的纯文档补记，不替换上面的实际运行提交。未执行真实客户录单／配置验收、真机验收、生产发布及微信上传／审核／发布。
