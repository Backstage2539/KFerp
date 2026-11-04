# PR-710 客户代发配置与虚拟仓库管理

- 状态：开发与生产环境交付完成，自动化、本地界面和两环境只读运行核对通过；Van 业务验收待确认。
- 开发分支：`codex/fulfillment-virtual-warehouses-20261021`，基线 `5b838315`。
- DEV-819：代发场景运费、仓库、服务费与审计整体保存，兼容旧客户端。
- DEV-820：虚拟子仓归类、分页、增改删、客户隔离及业务引用保护。
- DEV-821：开启后显示场景配置、仓库右抽屉、普通发货政策搬到业务物流设置。
- DEV-822：回归、手册、审核与开发环境交付。

## 验证边界

使用本机隔离 PostgreSQL 数据库与测试独立 schema；不修改真实客户配置、余额或订单。虚拟父级只展示分类，子仓沿用现有库存账。开发部署完成与 Van 业务验收分别记录。

## RED / GREEN 证据

- 原子保存：应用/HTTP 测试先失败，再验证嵌套政策、无 sale 依赖、政策或仓库错误整体回滚、审计故障整体回滚、关闭保留与旧格式兼容；单独修改 sale 不改变代发版本。证据 `/tmp/kferp-pr710-atomic-{unit-red,api-red,sale-red,target-green,final}.log`。
- 客户隔离：实际 PostgreSQL 先复现未绑定虚拟仓被当公共代发/加工来源，再覆盖材料、半成品、预览及提交锁定修复。证据 `/tmp/kferp-pr710-atomic-{scope-red,processing-scope-red,processing-scope-green}.log`。
- 虚拟仓：新路由先返回404；数据库测试覆盖历史仓归类及余额不变、CRUD/分页、版本冲突、审计回滚、删除引用保护、已完成业务可停用、历史不可改绑、引用写入共享锁及旧货主写入拒绝。证据 `/tmp/kferp-pr710-warehouse-{api-red,pg-red,green,final}.log`。
- Go 常规全包通过：`env -u ORDERAPP_TEST_DATABASE_URL -u DATABASE_URL go test ./...`，证据 `/tmp/kferp-pr710-backend-full-final.log`。启用数据库的相关回归单独执行，使用本机隔离测试库；两次完整空库启动与重复建表通过，证据 `/tmp/kferp-pr710-schema-final.log`。
- Vue 全量1472项通过、构建通过，证据 `/tmp/kferp-pr710-vue-final.log`、`/tmp/kferp-pr710-build-final.log`；行为测试 RED 证据 `/tmp/kferp-pr710-ui-{red,runtime-red,candidates-red}.log`。
- 本地浏览器模拟数据验证通过：关闭场景隐藏字段、开启显示同区域配置、包邮/零服务费、统一保存、新仓自动选中、费用草稿保留、未开门户普通客户政策维护。桌面和390px iframe视口的右抽屉已截图检查。截图保存在 `/Users/yiiiple-work/Documents/Codex/2026-10-21/fulfillment-virtual-warehouses/`，不代表真实客户业务验收。
- 独立代码复审通过：按真实 schema 核对引用列及完成状态，修复工单 released、订单状态ID、已撤回计划、已完成加工和货主仅变更更新的边界；无剩余阻塞项。

### 原有数据库夹具限制

尝试完整数据库包时，原有夹具存在缺少 `product_bom_spec_authorities`、物流重量、`work_orders.target_warehouse` 等错误，部分旧成品转仓夹具缺BOM权威配置。已从干净基线独立源码复现，未修改无关业务以绕过失败。相关日志为 `/tmp/kferp-pr710-atomic-{baseline,processing-baseline}.log` 与 `/tmp/kferp-pr710-warehouse-baseline.log`；本次相关数据库、接口和迁移回归均单独通过。

## 操作手册

- `docs/OP_MANUAL_CUSTOMER_STOCK_DROPSHIP.md`：场景统一保存和仓库抽屉。
- `docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`：履约入口与流程。
- `docs/OP_MANUAL_FREIGHT.md`：普通发货与代发政策入口。
- `docs/OP_MANUAL_STOCK.md`：虚拟分类、客户绑定及删除保护。

上述 Markdown 由原有 Vue/Vite 手册入口展示，不增加根目录手册副本。

## 开发交付

- 实现提交 `342758cd664e29b6669136e1259668ad242b25fc` 已推送功能分支；在独立干净 checkout 合并当前 origin/develop，合并树与已验证功能树一致。
- 开发运行版本 `2059307c38968c88bc1da8936d42a3a979543d63`。命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`。部署日志 `/tmp/kferp-pr710-deploy.log`。
- 服务器闸门：Vue1472项、小程序393项、类型检查、开发小程序构建和24页产物检查、宿主及镜像内Go全包测试通过。小程序产物仅在服务器校验，未下载或上传微信平台。
- 回滚锚点：源备份 `/opt/stacks/erp/orderapp.backup.deploy-20261021224657-2059307c3896`；镜像 `kferp-orderapp-rollback:development-20261021224657-2059307c3896`。
- 开发应用运行、PostgreSQL健康；外部登录页HTTP200；客户门户与业务设置页面、3份手册HTTP200；匿名新仓库API和履约设置API均401。
- 产品进度API包含 PR-710 review，DEV-819～822 done。编译后静态资源包含虚拟仓库抽屉，服务器产物 RELEASE_INFO 对应本次开发提交。
- 部署前后仓库20条、成品库存34条的完整校验值一致；客户仓归类14条，已绑定客户但未归类为0条，跨模块引用保护30个。仅归类、无余额或历史单据修改。
- 正式应用镜像及启动时间与部署前相同。未执行真实客户配置保存、订单、库存或账单业务验收。
- 构建期间仅清理可再生成的npm/Go/临时依赖与Docker构建缓存，保留数据库、源备份和带标签的回滚镜像。
- 完整日志及本地模拟数据截图归档：`/Users/yiiiple-work/Documents/Codex/2026-10-21/fulfillment-virtual-warehouses/`，只读运行证据为 `validation/kferp-pr710-runtime-smoke.log`。
- 最终回执仅更新进度与验收文档，后续文档合并不改变上述已部署运行版本。

## 生产交付授权与合并检查

- Van 于2026-10-21明确要求合并main并部署生产。使用独立干净main checkout，合入已验证的develop `7277f43e`；合并无冲突。
- 独立复审确认：PR710实现与develop一致，生产独有BOM物料产出回填保护及既有验收记录保留，无无关代码丢失。前端及小程序源代码与开发已验证版本一致。
- 合并后Go常规全包、本次原子保存/虚拟仓/加工来源隔离/完整建表重复初始化与生产BOM回填数据库回归通过。证据：`/tmp/kferp-pr710-production-{backend,database}.log`。
- 生产数据库发布前备份：`/opt/stacks/erp-production/backups/pr710-before-production-20261021230607.sql.gz`，gzip校验通过，权限600。
- 生产部署命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh production`。运行提交 `867fe68c6266d56a668848281960fd4379571222`，合并与推送完成后从干净main部署；全部6阶段成功，外部登录页HTTP200。
- 服务器发布闸门：Vue1472项、Vite构建、小程序393项、类型检查、生产构建及24页面产物检查、宿主Go全包与镜像内Go全包均通过。服务器小程序 RELEASE_INFO 精确匹配本次main提交、production及正式API域名；未下载小程序包或上传/发布微信版本。
- 生产应用镜像 `sha256:8522f5a3a2a772b25c19a5041c78060cdd22f2fb3dce0459e7a78ae74b8743e5`，启动时间 `2026-10-21T15:19:36.889540679Z`，运行且重启次数0；生产PostgreSQL健康。
- 生产源回滚锚点 `/opt/stacks/erp-production/orderapp.backup.deploy-20261021230954-867fe68c6266`；镜像 `kferp-orderapp-rollback:production-20261021230954-867fe68c6266`，指向原运行镜像 `sha256:654b6367cd2ec6d210f4ec10d019499319e89126d0b677f0dfe1dfdc309224ec`。源码、镜像和上述数据库备份均已核对存在且数据库压缩文件校验通过。
- 生产只读冒烟通过：客户门户设置、业务设置与3份手册HTTP200；两项匿名新接口401；已认证虚拟仓库分页/修订版本及客户履约设置读取通过；PR-710为review，DEV-819～822均done。编译后静态资源包含仓库抽屉。
- 生产发布前后仓库10条绑定校验值 `8abbb7706a32877e92fa23ca6c9ced95`、成品库存3条完整行校验值 `d0ade821bbcf4e16f86ab59f0d6e4a18` 一致。1个历史客户仓归入虚拟分类，无遗漏；30个跨模块引用保护触发器已安装。
- 开发应用镜像 `sha256:3ca7db88b26b4753ef780c389033017872d9af00ea0cb86f630f0cff1e99f465` 与启动时间 `2026-10-21T14:56:44.684207283Z` 均与生产发布前一致。
- 首次手册冒烟使用生产.env中的ORDERAPP_PASS返回401；改用运行容器实际加载的APP_USER/APP_PASS后全量检查通过，未输出或修改凭据。验证仅GET/SELECT，不执行真实客户配置、订单或库存业务写入。
- 构建后在部署锁保护下清理可再生成的Docker构建缓存，最终系统盘可用1.8GB；数据库、源码备份和带标签回滚镜像保留。生产日志与脚本归档到上述交付目录的validation，文件前缀为 `kferp-pr710-production-`。
- 本次最终回执只更新进度和验收文档，不重新部署；生产实际运行版本仍为 `867fe68c`。用户业务验收保持待确认。
