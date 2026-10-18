# PR-694 全部 BOM 规格录单与手动档位模板计价

日期：2026-10-17。产品验收：待 Van。DEV-775/776/777。开发交付，不含生产和微信上传发布。

## 规则与实现

- 商品仍须存在于所选价格表，ERP/员工小程序规格选择读取当前默认已发布且启用 BOM 的全部真实规格。默认规格第一并选中。
- 首次手动选择来源档位；换目标规格沿用来源档位发布时的模板，重新计算目标 BOM 原料、包材、工序成本。数量不换档。来源原规格保留发布价。
- 发布价格表时服务器冻结可执行模板，忽略客户端伪造模板。旧表没有模板快照时明确提示刷新计算发布一次；手工调整价和固定价可用原换算组。报价、保存均不新增或改写价格表。
- bom_tier 与原 converted_tier 共存。保存重新报价、比较报价修订哈希；历史快照仅从数据库读，数量编辑冻结原价；主动重新计价和复制走当前规则。
- 源档位身份、发布版本、模板快照、目标计算明细、两位小数成交价随订单保存；履约和库存引用目标 BOM ID。
- 客户自行下单拒绝两种员工换算指令；客户、商品、档位、规格身份均由后台校验；两端隔离过期异步响应，包含复制订单报价。

## RED

- Vue 新 BOM helper 尚未实现时 node:test 报模块不存在（/tmp/kferp-bom-pricing-vue-red.log）。
- 小程序 BOM helper 缺失时 Vitest 报模块不存在（/tmp/kferp-bom-pricing-mini-red.log）。
- Go 来源解析测试在 bomTierSource 实现前编译失败（undefined: bomTierSource）。
- 追加停用 BOM 场景后 PostgreSQL HTTP 测试失败：inactive BOM exposed as orderable（/tmp/kferp-bom-inactive-red.log）。随后给公共规格查询补充 BOM 启用状态校验。

## GREEN

- `scripts/verify_kferp.sh all`：完整 Go 测试、Vue 1368 tests、Vite build 通过（/tmp/kferp-bom-all.log）。最终 UI 文案和复制响应隔离修改后，相关 src/lib 1339 tests + Vite build 再通过。
- `ORDERAPP_TEST_DATABASE_URL='postgres:///postgres?host=/tmp' go test ./internal/appmain ./internal/interfaces/http/sales -run 'TestBOMOrderPricingPostgresHTTP|TestConversionPostgresHTTPAndFrozenOrder' -count=1` 通过；每例使用独立临时 schema，不写共享开发业务数据。
- PostgreSQL 测试包括：ERP/员工小程序报价一致；100g/500g/250g；不同包材成本；独立模板档位；普通行与两档 BOM 行混合；客户端伪造金额/模板被忽略；非法来源/跨客户/跨商品/无权员工/无效数量；成本变化409；历史编辑冻结；复制刷新；主动重新计价；停用 BOM；库存预览目标身份；价格表内容不变与订单日志。
- 原换算组 PostgreSQL API 全链路通过，包括配置、非法关系、跨客户、冻结订单与库存目标规格。
- 客户接口指令拒绝单测包括 bom_tier 字符串/数组/嵌套行。
- 小程序 `npm test`：54 文件、329 tests；`npm run typecheck`；`npm run build:mp-weixin:development` 均通过。
- 本地 Vue 真实组件 + 模拟 API，桌面 1365px 和窄屏：500g 默认第一且选中；必须手动选档；第二档切到250g，数量100仍为第二档、23.10元/袋、小计2310；原临时换规格入口保留。截图保存于交付目录。此项为本地 UI 验证，不是开发业务验收或真机小程序验收。
- 手册源 OP_MANUAL_BOM_ORDER_PRICING.md、订单/小程序/成本/换算组手册同步；录单内新增帮助入口。

## 交付

功能分支 codex/bom-spec-order-pricing-20261016，基于 origin/develop e5f409b0。集成与部署已完成，见下方回执。
构建的 chunk size/Browserslist 提示为现有非阻断提示。

## 开发交付回执

- GitHub PR #204：https://github.com/Backstage2539/KFerp/pull/204，已合并。
- 功能分支：codex/bom-spec-order-pricing-20261016；已推送实现提交 51a18c1ca40b465a27ab8bbae4b42e80ca4e1420。
- 已部署 develop 提交：3421b1b8c576c7e5c3a773887d70d3b02ce66362。
- 部署命令：`KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/Users/yiiiple-work/KFerp-bom-pricing-delivery-20261017/KFerp-miniapp-mp-weixin-dev ./deploy_orderapp.sh development`，在最新且干净的 develop 检出执行，退出0。
- 服务器再次通过 Vue 1368 tests、miniapp 329 tests/typecheck/build、完整 Go 测试和 Docker 镜像内测试；24个页面、PAGE_FILE_MANIFEST 96文件验证通过。
- 开发环境：https://dev.qacoohee.com/app/ 。应用 running、PostgreSQL healthy；login 200；app 根路由303；已认证需求/新手册200；未登录报价接口401。需求 PR-694 状态 review，等待 Van 业务验收。
- 回滚源码：`/opt/stacks/erp/orderapp.backup.deploy-20261017003932-3421b1b8c576`；回滚镜像：`kferp-orderapp-rollback:development-20261017003932-3421b1b8c576`。
- 开发小程序目录：`/Users/yiiiple-work/KFerp-bom-pricing-delivery-20261017/KFerp-miniapp-mp-weixin-dev`；压缩包：`/Users/yiiiple-work/KFerp-bom-pricing-delivery-20261017/KFerp-miniapp-dev-3421b1b8.zip`。
- RELEASE_INFO 确认 environment=development、api_base=https://dev.qacoohee.com/app、commit=3421b1b8；ZIP完整性通过，不含 project.private.config.json。
- 日志与本地测试截图保存在 `/Users/yiiiple-work/KFerp-bom-pricing-delivery-20261017`。
- 本回执为部署后的文档补录，不改变运行版本。未做生产部署、微信上传/审核/发布、真机或共享开发数据业务验收。
