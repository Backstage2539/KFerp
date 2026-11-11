# 工位任务展开逐批报工验收记录（PR-762）

## 开发验证

- 先补充默认产出、批量接口及幂等冲突失败测试，再完成实现。
- 冻结计划默认产出覆盖 BOM 20% 损耗、1.25 kg 投入对应 1 kg 产出、单位换算、少安排及尾批取整守恒；冻结来源不一致时不推测。
- 隔离 PostgreSQL 的批次 API 验证四批、单批后批量完成、重复请求、版本和权限校验、零工时、失败整笔回滚，以及已投入 59.962 kg 不重复扣料。
- `go test ./...` 通过；Vue 前端全量 1942 项测试通过，Vite 构建通过。
- 本地模拟接口下的 Chrome 桌面 1400×1000 与窄屏 390×844：四批并列展开，三个计划默认值显示，收起后输入保留，完成全部提交各批当前值，无横向溢出。
- 操作手册：`OP_MANUAL_PRODUCTION.md`、`OP_MANUAL_OPERATION_MODES.md`。

## 发布和用户验收

- 正式部署版本：`cfa8174a060304c4677deebb9cc2aea919741c7a`，来自已推送的 main；`KFERP_ERP_ONLY=1 ./deploy_orderapp.sh production` 成功，服务器端 1942 项前端测试、Vite 构建、主机和镜像内 Go 全量检查均通过。
- 清理前服务器约 1.8 GB 可用；按用户授权清理 36 个旧回滚镜像标签、41 个旧发布源码备份及不用的构建缓存。保留运行容器使用的镜像、生产/开发各最近 3 组回滚镜像和源码副本、数据库及数据库备份。清理清单：`/opt/stacks/erp-production/backups/pr762-space-cleanup-manifest.json`；部署后约 37 GB 可用。
- 只读检查：应用 running、重启 0，PostgreSQL healthy；登录页 200、匿名批次接口 401、管理员 GET 返回新计划产出/工时字段。工序卡 100 的前三批实际投入/产出仍各 15 kg，第四批仍待报工，未执行任何业务报工。
- 页面资源 `WorkstationView-kAgF5T5B.js`，服务器源码、容器和公网 SHA-256 一致：`c27a453fb2e018e338be2d074b10759e4d4cc940e4680034fe22fb9dcb526c84`。
- 现有小程序包完整目录哈希前后一致：`5ae9ea1f239b4396438ea3d860b7922daf7e164fec78e8a2ce81250b3f425955`，未重新构建、导出、上传或发布。
- 数据库备份：`/opt/stacks/erp-production/backups/pre-pr762-20261111221622.dump`，已验证可读取；SHA-256 `07e1bde9a1a3e7413f33e1ef30902b26cc81962adf2bc5a18f425c1d14275041`。
- 本次回滚源码：`/opt/stacks/erp-production/orderapp.backup.deploy-20261111221705-cfa8174a0603`；回滚镜像：`kferp-orderapp-rollback:production-20261111221705-cfa8174a0603`。
- 用户产品验收待完成。部署后的说明文档提交不改变上述已核验的应用版本。
