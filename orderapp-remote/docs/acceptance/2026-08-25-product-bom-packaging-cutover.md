# PR-607 商品包装 BOM 建立与生产发布验收记录

## 范围

- 固定处理 PR-606 的 31 组半成品，创建 31 个同名商品分装 BOM。
- 每个 BOM 复制发布模板“标准咖啡熟豆规格80g-2.5kg”V001 的 7 个规格，以唯一对应半成品为主料并保留包装物料。
- 26 个发布并设默认，5 个保留“待补半成品配方”草稿。

## RED 证据

- 2026-08-25 定向 Go 测试首次运行失败：缺少 `LoadPR607ProductionManifest`、PR-607 清单类型、迁移命令、PR/DEV 登记与文档合同。
- 重启保护合同首次运行失败：PR-403 legacy 修复尚未排除已存在显式商品产出 BOM 的商品。

## GREEN 证据

- 定向 Go/support GREEN；清单固定 31/26/5/11/3、7 个规格、三项改名、榛巧商品 62 及其迁移前默认 BOM 138/V223、分类映射和显式商品草稿启动保护。
- 2026-08-25 从当前生产只读备份恢复隔离数据库，preview 返回 `ready`、依赖 0；apply 在一个事务创建 31 个商品 BOM、217 个规格变体、26 个发布默认和 5 个草稿。第二次 apply 返回 `already applied`，没有重复建档。
- 同一已应用克隆依次执行 `backfillProductionBomLibrary` 与 `repairLegacyProductionBomBindings`，发布商品默认仍为 26，5 个显式草稿默认仍为 0，未增加 PR-403 legacy BOM。
- 克隆 rollback 恢复商品名称、启用状态和榛巧旧默认，31 个新 BOM 仅失效、不删除；第二次 rollback 返回 `already rolled back`。新增 1 条未完商品生产计划时 preview 整批拒绝；新增 1 条 PR-607 版本生产引用时 rollback 整批拒绝。
- `scripts/verify_kferp.sh all` exit 0：Go 全包 GREEN，Vue 1033/1033，Vite 6594 modules 构建 GREEN（仅既有 chunk-size warning）。
- development 预检及发布 GREEN：`develop@12787fb48f32dbd1ec81561f49a4929bcc0dbe8e`，源码备份 `/opt/stacks/erp/orderapp.backup.deploy-20260825003648-12787fb48f32`，回滚镜像 `kferp-orderapp-rollback:development-20260825003648-12787fb48f32`，应用容器 running、登录页 HTTP 200。
- production 预检 GREEN；PR #41 合入并发布 `main@7c8dec3a05e0124f588d7d69fa763d4dffd68a93`，源码备份 `/opt/stacks/erp-production/orderapp.backup.deploy-20260825005556-7c8dec3a05e0`，回滚镜像 `kferp-orderapp-rollback:production-20260825005556-7c8dec3a05e0`。应用容器 restart 0、PostgreSQL healthy、登录页 HTTP 200、受保护 API 未认证 401。首次构建曾在提升源码/容器切换前遇到基础镜像 TLS 握手超时；现有生产保持 HTTP 200，原提交重试后全门禁通过并成功发布。

## 生产数据

- 验证克隆来源备份：`/opt/stacks/erp-production/backups/pr607-verification-20260825a.dump`，2,282,737 bytes，SHA-256 `db8e8886a9613cbddb8c98a35a3dedc2d70d2a24b2604f110d4e5b9f875b0fa2`；该文件用于测试，不替代正式 apply 前的新备份。
- 正式生产 preview 两次返回 `ready`：31 商品、31 BOM、217 规格、26 发布、5 草稿、11 重新启用、3 改名、未完生产依赖 0；第二次 preview 在备份后、apply 前重新取得数据库锁确认清单未漂移。
- 正式 apply 前 custom-format 备份：`/opt/stacks/erp-production/backups/pr607-pre-cutover-20260825010308.dump`，2,284,242 bytes，1,963 restore-list items，SHA-256 `e33efcac11ca7daf99ffb66e405375afe9ad39c660c9411673c4201bcfcbcacc`。已实际恢复到临时数据库，成功读取 201 个原有生产 BOM 后删除临时库；备份文件保留。
- 单事务 apply 创建 BOM 881–911、版本 293–323。最终只读验收为 31 个启用商品 BOM、217 个规格、434 个组件、31 个唯一半成品主料、31 个商品/BOM 同名、26 个发布默认和 5 个零默认草稿；原 31 个商品产出 BOM 均保持失效。分类分布为单品 22、拼配 9。
- 7 种规格每种均为 31 条：18g `0.021kg + MAT-000101`、36g `0.039kg + MAT-000101`、80g `0.083kg + MAT-000102`、100g `0.116kg + MAT-000102`、227g `0.230kg + MAT-000103`、454g `0.454kg + MAT-000100`、2.5kg `2.500kg + MAT-000105`；半成品单位均为 kg，包材用量均为 1pic。
- 操作日志核心动作：建商品 BOM 31、发布版本 26、设商品默认 26、分类绑定 31、重新启用商品 11、商品改名 3、整批 apply 1。修正后的启用商品为 `云上莓梦`、`曜石2.0`、`白巧坚果`；5 个草稿为 `墨昙、果皮茶、晨曦·娜依、巴布亚新几内亚、萨琪姆水洗`。
- apply 后重复执行返回 `already applied; no changes written`。生产应用重启后登录页恢复 HTTP 200，preview 仍为 `applied`，26 个商品绑定/产出默认/生产配置均保留，5 个草稿仍为零默认；重启后的再次 apply 仍无写入，apply 审计保持 1。

## 人工验收边界

- 自动化和生产数据验收不替代 Van 的页面业务验收；`REV-607-PRODUCT-BOM-PACKAGING` 保持 todo。
