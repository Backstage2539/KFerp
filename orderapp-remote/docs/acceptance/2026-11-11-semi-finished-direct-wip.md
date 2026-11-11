# PR-760 半成品选择与当前页转入 WIP

状态：技术验证及正式发布完成；Van 产品验收待完成。

## 范围与验证

- 生产计划的半成品选择直接确认，内部仍检查库存、计划版本及工位变化；产能页移除逐任务报工类型核对。
- 确认使用不转仓；产能页可将所选批次的非 WIP 部分直接调入 WIP。WIP 现货按同批次、货主及可用量核对，旧草稿未固定批次不能猜选。
- 隔离 PostgreSQL 和真实 HTTP handler 覆盖部分/全部已在 WIP、外部转仓后重新选择、匹配待确认单复用、冲突单拒绝、幂等重试、跨仓失败回滚，以及提交工单后同批次占用。两张同规格订单保留各自订单映射，共用同批 45.4 kg，转仓后工单共一条 45.4 kg 预留；锁后并发外部入 WIP 不重复转仓。
- 定向验证：`ORDERAPP_TEST_DATABASE_URL=... go test ./internal/interfaces/http/production -run 'TestDirect|TestSelectedBatch' -count=1`；`scripts/verify_kferp.sh frontend-tests` 1922 项通过；Vue 构建通过。桌面和 390×844 窄屏真实 Chrome 使用模拟接口验证了确认与转仓状态，未使用真实工序数据。
- 前端组件验证覆盖确认与转仓、输入保留、重复点击及在产供应计算；手册 `docs/OP_MANUAL_PRODUCTION.md` 和 `docs/OP_MANUAL_OPERATION_MODES.md`。
- 服务器发布验证只读；不代用户操作现有生产计划或调拨。

## 交付

- develop `470dcddd21a515a8d3b624cfad87904ca0ac0fec`；main 正式运行 `c739b3d82438f18e84ef5e5c8aa2485b0d95d303`。按照本次指定仅部署正式环境。
- 正式发布服务器前端 1922 项、小程序 527 项、类型检查、构建和 Go 全量以及镜像内 Go 全量通过；应用运行且重启 0、数据库健康、登录 200、匿名计划接口 401、管理员计划接口 200。页面 JS 公网与服务器 SHA256 均为 `d8dfacf3a06b76912f191911b364bf56291540502ee87b1df0d51753aab6fb40`。
- 正式计划 26 只读核对仍为总需求 60.112 kg、所选 15 kg、本次生产 45.112 kg；未操作该计划、调拨或提交。
- 数据库备份 `/opt/stacks/erp-production/backups/pre-pr760-20261111191557.dump`，源码回滚 `/opt/stacks/erp-production/orderapp.backup.deploy-20261111191630-c739b3d82438`，回滚镜像 `kferp-orderapp-rollback:production-20261111191630-c739b3d82438`。
- 生产小程序包 `/Users/yiiiple-work/KFerp-miniapp-mp-weixin`，108 文件与版本清单已核对；微信上传、审核及发布未执行。
