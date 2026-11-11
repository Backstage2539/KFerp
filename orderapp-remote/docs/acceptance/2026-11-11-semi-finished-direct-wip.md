# PR-760 半成品选择与当前页转入 WIP

状态：技术验证中；Van 产品验收待完成。

## 范围与验证

- 生产计划的半成品选择直接确认，内部仍检查库存、计划版本及工位变化；产能页移除逐任务报工类型核对。
- 确认使用不转仓；产能页可将所选批次的非 WIP 部分直接调入 WIP。WIP 现货按同批次、货主及可用量核对，旧草稿未固定批次不能猜选。
- 隔离 PostgreSQL 和真实 HTTP handler 覆盖部分/全部已在 WIP、外部转仓后重新选择、匹配待确认单复用、冲突单拒绝、幂等重试、跨仓失败回滚，以及提交工单后同批次占用。两张同规格订单保留各自订单映射，共用同批 45.4 kg，转仓后工单共一条 45.4 kg 预留；锁后并发外部入 WIP 不重复转仓。
- 定向验证：`ORDERAPP_TEST_DATABASE_URL=... go test ./internal/interfaces/http/production -run 'TestDirect|TestSelectedBatch' -count=1`；`scripts/verify_kferp.sh frontend-tests` 1922 项通过；Vue 构建通过。桌面和 390×844 窄屏真实 Chrome 使用模拟接口验证了确认与转仓状态，未使用真实工序数据。
- 前端组件验证覆盖确认与转仓、输入保留、重复点击及在产供应计算；手册 `docs/OP_MANUAL_PRODUCTION.md` 和 `docs/OP_MANUAL_OPERATION_MODES.md`。
- 服务器发布验证只读；不代用户操作现有生产计划或调拨。

## 交付

- develop、main、开发/正式部署、运行版本、页面检查和回滚证据待补充。
