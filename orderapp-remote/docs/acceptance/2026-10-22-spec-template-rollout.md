# PR-715 规格模板下发与报价、BOM 版本解耦

状态：实现、验证与开发部署已完成；业务验收待 Van。功能分支 `codex/spec-template-rollout-20261022`，目标 `develop` 和开发环境。

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

只部署开发环境。未向正式 BOM 下发、未重新发布正式价格表、未改写历史订单/生产计划/工单。初晓/曜石正式数据业务验收由 Van 在明确正式发布后进行。部署与运行证据见下方开发发布回执。

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
