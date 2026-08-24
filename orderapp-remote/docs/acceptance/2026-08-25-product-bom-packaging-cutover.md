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
- 待补：development / production 预检、发布与健康检查结果。

## 生产数据

- 验证克隆来源备份：`/opt/stacks/erp-production/backups/pr607-verification-20260825a.dump`，2,282,737 bytes，SHA-256 `db8e8886a9613cbddb8c98a35a3dedc2d70d2a24b2604f110d4e5b9f875b0fa2`；该文件用于测试，不替代正式 apply 前的新备份。
- 待补：正式生产 preview 报告、完整备份路径/大小/恢复清单/校验和。
- 待补：单事务 apply、服务重启后 31/217/26/5、默认绑定、`can_manufacture`、审计和重复 apply 幂等结果。

## 人工验收边界

- 自动化和生产数据验收不替代 Van 的页面业务验收；`REV-607-PRODUCT-BOM-PACKAGING` 保持 todo。
