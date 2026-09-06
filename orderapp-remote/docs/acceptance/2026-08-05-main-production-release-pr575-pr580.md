# 2026-08-05 develop 合入 main 与 production 发布证据

## 发布结果

- 从最新 `origin/main@47e864930749aa643812a0bf0be0964011c9fce7` 创建隔离 release 分支，合并 `origin/develop@918e9f6eba315d9cd6cadad2f091cb108c6e17bd`，人工保留双方 `ACTIVE_REQUIREMENTS.md` 历史证据。
- release merge `a06aa95ebe38d7b91806cd234032c0cc3bb62a7e` 已通过普通非强制 push 合入 `main`，并从干净 `main` 克隆部署到 production。
- 本次发布覆盖 PR-575、PR-577、PR-578、PR-579、PR-580 及当时 develop 已完成的其他提交；不声明待办中的 Van 手工业务验收已经完成。

## 自动化门禁

- 本地变更验证与完整 Go 测试通过。
- production 无切换预检通过：Vue 876/876、小程序 157/157、类型检查、production mp-weixin 13 页面构建、完整 Go 测试和隔离 Docker 构建。
- 正式部署再次完成同一组远端门禁；只重建并切换 `erp_prod_orderapp`，PostgreSQL 与 Caddy 保持运行。

## 数据库与回滚

- 发布前数据库备份：`/opt/stacks/erp-production/backups/pre-deploy-20260805232757-a06aa95ebe38.dump`，SHA-256 `5cc292b6cb4e9226e69bf91d9e6d552de7c5bb128c1763314acf84ef4902696b`。
- 备份通过 `pg_restore --list`，并在无外部端口、临时存储的独立 PostgreSQL 容器完整恢复，确认 207 张表可读取。
- 上一版源码：`/opt/stacks/erp-production/orderapp.backup.deploy-20260805233037-a06aa95ebe38`。
- 回滚镜像：`kferp-orderapp-rollback:production-20260805233037-a06aa95ebe38`。

## 上线只读检查

- `erp_prod_orderapp` 运行且重启次数 0，`erp_prod_postgres` healthy，文档转换和 Caddy 容器运行。
- 外部登录 HTTP 200；共享地址解析接口使用纯测试收件信息返回 200 并正确拆分联系人、电话和地址；近期开机日志没有 panic、fatal、SQLSTATE 等关键错误，Caddy 5xx 为 0。
- 生产源码存在共享收货信息解析路由、无 BOM 精确诊断和空发布 BOM 阻断；旧匿名 `/api/auth/password/set` 路由未注册。
- 独立只读事务确认无组件 published BOM 版本仍为 34 个，其中有商品绑定的空版本仍为 27 个，与部署前基线完全一致。
- 上线过程没有创建、发布或绑定 BOM，没有重算价格表，也没有创建或修改真实客户、外部账号或订单。

## production 小程序包

- 固定目录：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin`。
- 上一包备份：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin.backup-20260805233618-a06aa95ebe38`。
- ZIP：`/Users/yiiiple-work/KFerp-miniapp-mp-weixin-production-a06aa95e.zip`，大小 133331 bytes，SHA-256 `48f74667fe370b5d14e74d2f48e6f88f64a59032ec3c568f38ad39992aac96ae`。
- `RELEASE_INFO` 为 commit `a06aa95ebe38d7b91806cd234032c0cc3bb62a7e`、environment `production`、API base `https://erp.qacoohee.com/app`；52 文件页面清单与 ZIP 完整性通过，包内没有 development API。
- 微信开发者工具上传、提交审核和正式发布未执行；需关闭旧项目并重新导入固定目录后单独完成。

## 待 Van 验收

- 生豆无 BOM、空 published BOM 与正式发布后的正数试算。
- 生豆商品作为生产 BOM 产出商品的搜索与保存。
- 员工管理员分享图片入口开关及客户微信收到后的实际表现。
- 员工小程序粘贴收货信息、目标客户外部密码及客户小程序登录。
