# PR-758 物料准备与调拨验收

## 范围和状态

- 独立修复分支：`codex/production-material-preparation-20261110`；初始基于 `origin/develop` 的 `23f15a7f54e4b256bb1cb8b07d39d032da295cf3`，之后合入 `71a798b93bed49f10c2131df50163d8b7dd06e18` 并重新验证。
- 物料准备与产能安排分步，计划关联单项及批量调拨草稿，仓库确认后入 WIP；最终提交复核物料、有效产能和待处理调拨。保留烘焙、色选与包装的现有产能规则。
- 已合入开发与正式分支并发布两个环境。未使用 PP26 真实库存进行写入测试，未修改正式环境业务数据。Van 现场验收待完成。

## 已有验证证据

- 提交 WIP 校验：`TestRequirePlanMaterialsInWIP` 初次编译因函数不存在失败；实现后现货未入 WIP、已入 WIP、计划内上游与混合供应的定向测试通过。
- 待确认调拨阻止提交：`TestPendingPlanTransfersBlockWorkOrders` 初次编译因函数不存在失败；实现后隔离数据库覆盖草稿、已提交、已取消及无单据的定向测试通过。
- 数量和网络重试 RED/GREEN：`1.001 kg` 精确转换为 `1001 g`，小于整克的输入被拒绝；单项调拨创建结果因网络中断未知时，重试保留原输入与请求标识，避免重复创建，修改输入后重新核对并使用新标识。
- 隔离 PostgreSQL 定向验证了计划调拨、精确批次/规格/货主和质量、数量单位、计划变化保护、接口权限、重复请求与最终提交校验。`TestPlanTransferSerializesWithIncomingWIP` 验证调拨过账与其他调拨或采购入库并发时重新核对 WIP，超量单据保持草稿。
- 历史已下发工单补料相关定向回归、五个生产/成品型来源流程通过；隔离夹具先满足或验证新的入 WIP 提交条件，再继续检验历史工单操作，未修改真实库存。
- 前端全量测试 1848 项、Vue 构建、无数据库 Go 全量测试及 `scripts/verify_kferp.sh changed` 通过。桌面及 805×752 窄窗口在使用模拟接口的数据页面检查了物料准备、两种调拨抽屉和横向滚动；这不等同于真实接口的浏览器端到端验收。
- 生产和库存手册已更新；Van 实际操作验收待完成。

## 完整数据库回归限制

- 使用独立 PostgreSQL 运行库存仓储完整测试通过。生产仓储套件的 `TestProductionTraceAnalyticsOmitsEmptyTraceRows` 因旧测试库夹具缺少 `job_cards.scheduling_mode` 失败；在同一隔离环境的原 `origin/develop` 基线上复现。
- 完整生产接口套件在隔离数据库中仍有 21 项失败；同一环境原 develop 基线有 25 项失败，逐项比较无新增失败。生产接口套件与生产仓储套件均未全绿，不将基线对照记作全量通过。

## 开发环境发布与只读检查

- 运行版本：`352b1d700cf9d9cee2ec41dc7ee7b772857c0b40`；服务器端前端 1875 项、构建和无数据库 Go 测试通过，保留既有小程序包。
- 应用运行、重启次数 0，数据库 healthy；登录 HTTP 200，未登录物料库存接口 HTTP 401。`PR-758` 及三项开发记录为待验收。
- 公网、服务器目录和容器内 `ProducePlanView-Op9_xa1v.js` 的 SHA-256 均为 `33403e2e8ec26be520b94bfd969310c9d77865e120ebc6a55f8b96f18c9e9129`，包含生成全部缺料调拨单和下一步产能安排文案。版本号依据发布日志；该目录无可独立核验的 `.release-commit` 文件。
- 回滚源：`/opt/stacks/erp/orderapp.backup.deploy-20261111002519-352b1d700cf9`；回滚镜像：`kferp-orderapp-rollback:development-20261111002519-352b1d700cf9`。未写入真实业务数据。

## 正式环境发布与只读检查

- 正式发布版本：`3fc77b5c7fb944aa3689b9bb5f394cabf8358511`，依据发布日志；ERP-only 保留原正式小程序包 `1613c632b867`。
- 正式应用 running、重启 0；数据库 healthy，历史重启计数 13；正式 Caddy running。登录 HTTP 200，未登录调拨候选接口 HTTP 401。计划调拨关联表 `production_plan_material_transfer_links` 和 `production_plan_material_transfer_requests` 已存在；PR-758 及三项 DEV 为待验收。
- 公网、主机、容器的 `ProducePlanView-Op9_xa1v.js` SHA-256 均为 `33403e2e8ec26be520b94bfd969310c9d77865e120ebc6a55f8b96f18c9e9129`，公网返回 200，包含单据生成与产能安排文案。以上均为只读技术检查，未进行真实业务端到端验收。
- 发布前数据库备份：`/opt/stacks/erp-production/backups/pre-pr758-20261111014813.dump`，权限 600，SHA-256 `2f6c3b944b19261fb580a1a39ab9dec90e94cd1423107e322442e850eb72e41a`；已验证 pg_restore 列表可读，未进行完整恢复演练。
- 发布回滚源：`/opt/stacks/erp-production/orderapp.backup.deploy-20261111014913-3fc77b5c7fb9`；回滚镜像：`kferp-orderapp-rollback:production-20261111014913-3fc77b5c7fb9`。未创建或过账业务单据。
