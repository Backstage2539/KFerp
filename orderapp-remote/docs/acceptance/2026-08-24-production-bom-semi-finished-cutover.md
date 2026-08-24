# PR-606 生产 BOM 半成品化与编辑缺陷修复验收记录

## 范围

- 修复 BOM 抽屉保存/发布反馈、失败保护、统一草稿标准化及损耗实时重算。
- 为已发布 BOM 提供保留历史的替代草稿流程。
- 用锁定清单把生产“烘焙豆-半成品”31 个商品产出 BOM 切换为半成品物料产出 BOM。

## RED 证据

- application BOM 定向测试首先因缺少替代草稿命令、接口和已发布产出身份错误而编译失败。
- frontend 定向测试首先确认 API 错误码未透传、损耗批量重算函数不存在，BOM 抽屉内也没有常驻反馈与替代草稿入口。
- 这些失败均发生在实现前，用于锁定本次回归边界。

## GREEN 证据

- application/API/frontend 定向测试已通过：草稿标准化、19.5% 损耗重算、409 稳定错误码、替代草稿事务和源历史不变均有自动化覆盖。
- PR-606 清单测试已通过：31/30/26/5 数量、初晓配方、曜石2.0 V006、榛巧拼配磅转 kg 及 5 个待补草稿均被锁定。
- 第一次生产克隆库应用准确整批回滚并发现已停用组件：曜石2.0/曼特宁引用物料 42，森林瑰夏日晒引用物料 67。清单随后锁定 42→7、67→27 的启用同业务物料映射，不恢复失效档案且不绕过发布校验。
- 合入最新 `origin/develop` 并改用未冲突的 PR-606 编号后，`scripts/verify_kferp.sh all` 再次通过：Go 全包 GREEN；Vue 1033/1033；Vite 6594 modules 构建 GREEN（仅既有 chunk size warning）。
- 正式 PR-606 清单的生产只读预演返回 `state=ready`、`dependency_count=0`。生产完整归档（2,254,444 bytes / 1,955 archive items）恢复到独立临时数据库后，应用得到 31 个失效源 BOM、31 个有效 kg/kg 自制半成品物料、31 个替代 V001（26 published / 5 draft）、26 个物料默认 BOM；5 个待补名称准确，初晓为 `0.195 + 56:50/85:15/10:20/47:15`，榛巧为 `249.122356g/249.122356g/749.571691g`，白巧坚果比例与来源版本一致，三个旧组件引用全部映射为 7/27 且无 42/67。
- 同一克隆库第二次应用返回 `already applied; no changes written` 且 apply 审计仍为 1 条；补偿回滚后 31 个源 BOM 重新启用、31 个替代 BOM 失效。重新恢复的克隆库加入 1 条未完生产引用后，应用返回阻断且替代 BOM 数为 0。
- 临时数据库已删除。生产服务器曾因根盘满使 PostgreSQL 在自恢复期间短暂不可用，已删除本次 0 字节失败备份并只清理可重建构建缓存及 7 天前未使用镜像；数据库、当前运行镜像、最新回滚镜像和业务备份保留，正式切换前根盘可用空间 9.4GB。
- PR-606 已作为隔离生产发布合入 `main@a6776cd1dd268ab3ce7e2b1dad2cd5df61dba8dd`，明确未携带仅限 development 的 PR-605。development 与 production 发布门禁均通过；生产源码备份为 `/opt/stacks/erp-production/orderapp.backup.deploy-20260824142714-a6776cd1dd26`，回滚镜像为 `kferp-orderapp-rollback:production-20260824142714-a6776cd1dd26`，公网登录 HTTP 200、未认证受保护接口 HTTP 401。
- 正式生产第二次锁定预览仍为 `state=ready`、31/30/1/31/26/5 且依赖为 0。随后创建并验证 `/opt/stacks/erp-production/backups/pr606-pre-cutover-20260824143425.dump`（2,256,127 bytes / 1,955 archive items / SHA-256 `81c881dee5bef781fa484962ad8748f5a58eae41eaf3848eb42913dad5d27642`），再以单事务应用。
- 生产应用后验收：31 个目标物料、31 个替代 BOM、26 个 published/default/`can_manufacture=true`、5 个空配方 draft，31 个源 BOM 全部 inactive，旧商品默认/产出/生产配置绑定均为 0；初晓为 `0.195 + 50→62.11/15→18.63/20→24.84/15→18.63`，曜石2.0 来源为 published V006，榛巧与白巧坚果均符合锁定值，42/67 引用为 0。
- 相同生产迁移第二次运行返回 `already applied; no changes written`；最终预览为 `state=applied`。操作日志包含 30 个物料创建、1 个初晓物料更新、31 个替代草稿创建、26 个发布、26 个默认 BOM 绑定、31 个源 BOM 失效、31 个商品制造绑定清理及唯一 1 条整批 apply 审计。
- 验收记录版本重启后，两条旧兼容路径曾把 24 个仍有旧配方的商品重新绑定到已失效源 BOM。真实 PostgreSQL 回归先复现 `intentionally-inactive` 商品错误得到 1 条绑定，system library backfill 的源码合同也先红；修复后 PR-403 repair 与 system backfill 均只选择 active 目标 BOM，测试转绿，故意失效的切换源不会在后续启动时复绑。
- 两道保护已部署到 `production main@c2fb95cd1393b5f661f2e5a1d72e3b0ec4aa488f` 和 `development develop@663c563088d23fc9f0c45afd477fed1f60b4a5f5`。生产随后以带锁事务精确删除 24 条 `system-backfill` 兼容绑定，数量不符会整笔回滚，并写入唯一 `repair_restart_reintroduced_bindings` 审计；再次重启后 31/26/5、31 个有效物料、31 个失效源 BOM保持不变，旧商品 binding/output-binding/config 三类当前关联均为 0，登录 HTTP 200、未认证受保护接口 HTTP 401。

## 生产数据门禁

1. 代码先经过全量验证并依次部署 development、production。
2. 生产代码健康后重新执行只读预览；来源成员、版本、配方指纹或未完生产引用漂移时整批中止。
3. 创建完整 PostgreSQL 备份并验证归档可读取后，才允许以单事务应用锁定清单。
4. 应用后核对 31 个物料/BOM、26 个发布默认、5 个待补草稿、旧绑定解除、初晓与特殊换算以及全部操作日志。
5. 回滚只做补偿状态和绑定恢复，不删除新旧 BOM、版本、快照、物料或审计记录。

## 当前结论

- 自动化开发、正式生产克隆验证与 live production 验收：已完成。
- development / production 代码发布：已完成。
- 生产数据：单事务切换完成并通过幂等、配方、绑定、制造状态和审计核对；启动兼容回归已修复、审计清理并经再次重启验证；可恢复备份已保留。
- Van 页面与业务验收：待执行。
