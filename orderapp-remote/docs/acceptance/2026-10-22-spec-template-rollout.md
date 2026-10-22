# PR-715 规格模板下发与报价、BOM 版本解耦

状态：实现、验证、开发及正式部署已完成；业务验收待 Van。开发功能分支 `codex/spec-template-rollout-20261022` 已交付；Van 后续明确授权合并 main 并部署生产，生产集成分支 `codex/spec-template-rollout-production-20261022`。

## 变更

- 两处订单目录过滤改为稳定规格与库存单位匹配；发布价档标识、嵌套 effective_sales_spec 原样保留，独立携带当前执行 bom_variant_id。
- 模板版本 GET rollout-preview / POST rollout：预期当前版本、最新模板检查、现有图/商品/BOM/模板锁；逐项事务复用发布校验、库存引用约束、成本与绑定更新。新 audit action rollout_spec_template 记录批次和实际差异。
- 三方合并按模板规格键和字段比较；组件作为整体且展开实际主体。保留本地规格与无冲突局部修改；草稿、来源缺失和冲突阻塞。
- Vue 下发清单、默认选择、差异与阻塞原因、结果与失败重试；BOM 来源和更新提示。网页及员工小程序在过期 BOM 保存失败时刷新规格/运费后要求核对。

## 复现与验证

RED：稳定规格69发布价来自旧执行版本1167，当前1206时两处目录过滤返回空；三方合并未应用重量差异；新增 API 返回404；Vue 未选中可下发行。

GREEN：

- Go 应用、仓储和 HTTP 回归；完整后端测试。
- 本机 PostgreSQL 的独立临时 schema（不读取/写入正式业务）：同规格物流重量更新、并行重复下发只升一版、多个旧模板来源、部分成功、草稿/局部冲突/过期 BOM/过期模板阻塞、发布失败完整回滚、本地名称和自建规格保留、新规格加入、删除规格/删除本地修改冲突/库存阻止删除、改单位生成新身份、审计记录。
- 真实订单 HTTP：旧发布价的自动档与手选档使用当前 BOM，保留原有效规格快照，新订单保存260g物流重量；客户/规格/表选择越权阻断；历史重量快照已有回归保持通过。
- Vue 1506 项测试（合并最新 develop 订单付款修复后）及生产构建通过；下发组件实际 setup/API 的默认勾选、取消个别项、部分失败重试回归。
- 员工小程序录单36项测试和类型检查通过，包含更新执行版本时保留数量、原价档及客户身份。

完整复跑命令：`scripts/verify_kferp.sh backend`、`scripts/verify_kferp.sh frontend`；数据库定向套件 `TestSpecTemplateRollout*`、`TestOrderAPICutoverProductUsesCanonicalPublishedPriceAcrossBOMVersionsWithoutLegacyMapping`、`TestNamedPriceTableOrderAPIUsesSelectedSnapshotAndBlocksBypass`、`TestOrderAPIQuantityEditPreservesFrozenWeightAndHistoryUnknown`，通过 `ORDERAPP_TEST_DATABASE_URL` 指向隔离测试数据库。日志在任务工作区 outputs/pr715。

## 交付界限

前期已部署开发环境，后续正式发布由 Van 明确授权。未向正式 BOM 下发、未重新发布正式价格表、未改写历史订单/生产计划/工单。初晓/曜石正式数据业务验收由 Van 在明确正式发布后进行。部署与运行证据见下方开发发布回执。

界面检查：本机隔离合成数据渲染真实 SpecTemplateRollout 组件，确认默认勾选两项、冲突/已更新项禁选、展开重量差异和取消单项后计数正确；截图 outputs/pr715/rollout-preview.png。未连接业务接口。员工小程序开发包构建通过；微信上传/发布不在本轮范围。

## 开发发布回执

- PR [#246](https://github.com/Backstage2539/KFerp/pull/246) 已合并。功能分支推送提交 `1bb5841ca0cb5699df453f5de961c5f3e2f049cb`；开发运行提交 `624341aa53e34b486043409e47f4e131e38691fd`（合并树与验证代码相同）。
- 发布命令：`KFERP_MINIAPP_DEVELOPMENT_EXPORT_DIR=/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev ./deploy_orderapp.sh development`，退出0。服务器 Vue1505/1505、小程序426/426、类型检查、两套构建、宿主Go和镜像内Go检查全部通过。下发组件额外1项运行测试在本机通过，合计Vue1506；交付记录提交将该测试移到部署脚本覆盖的lib目录，便于以后每次发布自动运行。
- 2026-10-22 21:47:59 Asia/Shanghai 开发应用启动，运行镜像 `sha256:69d1f9b3cb444e0d7729d20dcdd5dd71088458090011db0c6ef00bf217bdc306`，重启计数0。开发及正式数据库均healthy。
- 开发生产配置与录单shell HTTP200，根入口303；未认证保护API401。已认证模板预览接口参数校验400（路由已存在）；PR-715及DEV-839—842在需求API可读。产品与交付状态保持review，等待Van业务核对。
- `ProductionSettingsView-oZ9_tW2b.js`、`OrderEntryView-D02KgihE.js` 公开响应、运行容器和服务器构建产物字节一致，并包含下发更新与过期BOM刷新标记。两个相关手册HTTP200且包含PR-715。
- 回滚源码 `/opt/stacks/erp/orderapp.backup.deploy-20261022213755-624341aa53e3`；回滚镜像 `kferp-orderapp-rollback:development-20261022213755-624341aa53e3`。
- 开发小程序产物已导出到 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin-dev`，RELEASE_INFO对应624341aa/development/dev.qacoohee.com，24个页面与96个清单文件校验通过。原包保留为同路径 `.backup-20261022214822-624341aa53e3`。未上传或发布微信小程序。
- 正式应用镜像和启动时间与发布前完全一致。本轮无正式业务写入，未执行开发/正式业务验收。为发布释放1.6GB可重建Go缓存；未清理业务数据或回滚镜像。
- 原始证据：任务工作区 `outputs/pr715/deploy.log`、`runtime-smoke.json`、`predeploy-runtime.txt`、数据库/API/前端测试日志。交付回执和测试路径调整为后续文档/测试提交，不改变已部署运行代码。

## 正式发布准备

- 2026-10-22 Van 明确要求“合并到main，部署到生产环境”。基于最新main `6dea2af0` 单独集成已验证的本次功能、回归和开发交付记录；保留main专用物料回填保护及其回归，不覆盖既有生产交付记录。
- 生产版本重新执行Go、真实隔离PostgreSQL/API、Vue及构建；服务器发布继续运行小程序全套检查及正式包构建。实际发布与回滚证据见正式发布回执。
- 本次授权包含生产服务器和对应小程序产物；不执行真实模板下发、订单保存或价格表发布，不替代Van业务验收。

## 正式发布回执

- PR [#248](https://github.com/Backstage2539/KFerp/pull/248) 合并main；生产集成分支已推送 `da5c53313232a7e6ae313c49ce78f052df5cb9ca`，正式运行及导出包提交 `b2d6e1be65b787679f73409a4d46bfcf0f689707`。功能与已验证开发版本一致，额外保留main既有物料回填保护及生产交付记录。
- 本机重新验证：完整Go、Vue1506/1506与构建、真实隔离PostgreSQL模板下发和订单API，以及生产物料回填回归全部通过。发布命令 `./deploy_orderapp.sh production` 在干净main检出 `/tmp/kferp-pr715-production-20261022` 执行并退出0。
- 服务器发布门禁：Vue1506/1506、小程序426/426、类型检查、正式包24页构建、宿主Go和镜像内Go全部通过。正式入口login外网HTTP200。
- 正式应用于2026-10-22 22:35:22 Asia/Shanghai启动，镜像 `sha256:36dd955998d74fdb1f3ea3d4c517dcfcaa730dc5a1bb7971e22bb2b1972104e4`，重启0；两套数据库healthy，开发应用镜像和启动时间与发布前一致。
- 使用运行容器实际认证配置执行只读检查：根入口303，生产配置与录单shell200，模板预览无效ID校验400，未认证API401，两个手册200且含PR-715，需求API可见PR-715与DEV-839—842。产品及交付状态保持review，等候Van业务核对。旧部署说明中的固定BasicAuth用户名不适用于当前正式实例；没有修改任何认证配置。
- `ProductionSettingsView-oZ9_tW2b.js` 与 `OrderEntryView-D02KgihE.js` 公开响应、容器文件和服务器构建产物完全一致；包含下发更新和过期BOM刷新标记。对应SHA256为 `874815c51dd1c695529f16d4ff678fd41260bd6696507182eb8b44400915f42f`、`9ab11eddd89d3c755b3408db6e2e318ce263dcacd20a67ab472c25e6d37de5e6`。
- 回滚源码 `/opt/stacks/erp-production/orderapp.backup.deploy-20261022222528-b2d6e1be65b7`；回滚镜像 `kferp-orderapp-rollback:production-20261022222528-b2d6e1be65b7` 保留发布前运行镜像。发布前仅清理可重建Go/npm/Docker构建缓存，将可用空间从1.7GB恢复到4.2GB；发布后2.6GB可用，未删除业务数据、源备份或回滚镜像。
- 正式小程序包 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`：RELEASE_INFO为b2d6e1be/production/erp.qacoohee.com，24页、96清单文件校验通过。旧包保存在同路径 `.backup-20261022223540-b2d6e1be65b7`。未上传、送审或发布微信小程序。
- 本轮未执行真实BOM模板下发、订单保存、价格表重新发布或历史快照修改；业务验收仍由Van完成。
- 证据：任务工作区 `outputs/pr715/production-deploy.log`、`production-runtime-smoke.json`、`production-predeploy-runtime.txt`、`production-backend.log`、`production-frontend.log`、`production-isolated-api.log`、`production-backfill-regression.log`。此发布回执属于后续文档提交，不改变正式运行代码。
