# PR-719 生产计划产能拆分优化

2026-10-23。分支 `codex/production-capacity-batches-20261023`，基线 `470018f9`。原 ccd2 工作区已有改动保持不变。PR 状态 review，Van 业务验收待完成。

## 交付范围

核对面板折叠；可点击工位及完整编辑抽屉；正整数批次和只读产量；承担所有批次；自动拆分整批余量；旧尾批保存兼容。按任务 ID 和冻结工序序号隔离，重复工序 ID 不串行。名称/费率刷新保留旧尾批，批量变化保留批次数重算。服务端读取当前产能，拒绝无效批次，旧数量请求仍兼容。没有新增数据库字段。

权限验证补齐生产计划接口在统一权限映射中的遗漏：读取归 production.read，预览与保存归 production.run；工位/产能编辑仍为 bom.write。已有角色权限未扩张。业务写入沿用操作日志。

## RED → GREEN

- 前端初始 RED：缺少批次驱动、承担全部、整批余量和旧尾批兼容函数。
- 服务层 RED：按批次请求被旧 planned_qty 必填挡住，整张草稿未拒绝无效行。
- PostgreSQL/API RED：count-only 预览返回 400，工位失去适用工序后仍可保存。
- 运行时 RED：缺少可折叠核对和共享工位编辑行为。
- 联调补充 RED：旧草稿携带旧产能时，编辑应使用当前产能；保存重载后缺 operation_id 的行应从冻结工序序号恢复校验身份。
- 权限 RED：缺生产权限的员工可进入计划接口；补齐映射后相关接口 403/200 边界通过。
- GREEN：1,529 项前端测试、全量 Go 包测试、Vue 生产构建。构建仅有既有 chunk-size 提醒。
- 隔离 PostgreSQL：TestCapacityBatchAPIReadWriteCompatibility、两个旧拆分 API 测试、整张草稿原子保存及版本冲突测试通过。覆盖预览、两种保存入口、重载、旧数量、无效批次、服务端最新产能、停用/工序变更阻断、操作日志和已提交快照保护。
- 计算覆盖 kg/g/件、计时/计件成本、浮点整批边界、整除及余量、同任务工序隔离。

## 验证范围与既有问题

业务写入只使用本机临时 PostgreSQL schema 和本地内存 UI fixture，没有写生产计划 11。全量 Go 默认不配置集成数据库；本次接口的真实数据库验证单独运行。

扩展数据库回归遇到两个旧 fixture 问题，在原始 470018f9 基线同样复现：归档 BOM 测试缺 product_bom_spec_authorities 表、父商品冻结测试缺来源库存。另一个提交/取消并发测试偶发库存不足，基线连续三次通过；该测试不走拆分接口，保留失败记录，不计入本次 GREEN。

## 浏览器验证（隔离 fixture，实际 Vue 组件）

- 宽屏默认展开；805px/390px 默认收起，可切换，收起仍保留超排勾选。
- 103.797kg / 20kg = 6 批、120kg、超排 16.203kg；单行取代原同任务两行，150 分钟、255 元。
- 旧 3.797kg 尾批打开保留；1.5 批显示错误且保存禁用。
- 抽屉含基础资料、人员、工序、成本和产能档。保存 12kg 产能时保留 6 批并变为 72kg，原未保存拆分仍在。关闭已保存表单正常返回。
- 工位和产能表单分别保存，保留另一个表单未保存输入，由运行时测试验证。
- 改回 20kg 后保留 9 批变为 180kg，承担所有批次重算 6 批、120kg。
- 确认安排返回详情且仍为草稿；刷新重开数量、批次一致，无工序身份误报。
- IAB 对原生未保存确认框的自动控制受限，后续交互在独立 Chrome 验证会话完成。

截图均为隔离数据：

![整批安排](assets/pr719/batches.png)
![窄屏折叠](assets/pr719/narrow.png)
![工位抽屉](assets/pr719/drawer.png)
![手机布局](assets/pr719/mobile.png)

## 可重复验证

```sh
bash scripts/verify_kferp.sh changed
bash scripts/verify_kferp.sh frontend
(cd orderapp-remote && go test ./...)
(cd orderapp-remote && ORDERAPP_TEST_DATABASE_URL='postgresql:///postgres?host=/tmp' go test ./internal/interfaces/http/production -run '^Test(CapacityBatchAPIReadWriteCompatibility|ProductionPlanOperationSplitAPIReadsAndSavesDraftCapacitySplits|ProductionPlanOperationSplitPreviewAPIReturnsDemandGapWithoutSaving|ProductionPlanDraftAPISavesTheWorkspaceAtomically|ProductionPlanDraftAPIReportsVersionConflict)$' -count=1)
```

## 发布

待记录整合版本、部署完成、运行/静态资源检查和回滚依据。生产计划 11 只读核对，不做真实保存或提交验收。Vue 手册页直接加载 OP_MANUAL_PRODUCTION.md 单一来源，工作区内说明同步更新。
