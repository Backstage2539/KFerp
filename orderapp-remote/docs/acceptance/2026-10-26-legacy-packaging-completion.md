# PR-730 旧包装工单合组完成兼容验收

日期：2026-10-26
分支：`codex/legacy-packaging-zero-input`
基线：`origin/develop@a31eab044643924069fb74862acf4f7c861bcdcf`

## 问题复现（RED）

在隔离 PostgreSQL 数据库 `codex_pr734_roast_test` 中按旧数据形状建立包装工单：工单冻结袋数明确，唯一包装工序卡的 `planned_input_qty=0`，且 BOM/物料快照完整。基线实现的完成预览错误返回 `enabled=false`、`legacy_execution=false`、`blocked_code=legacy_execution_unresolved`、`blocked_reason=旧工单暂不可自动完成，请核对冻结 BOM 与工艺路线`。对应测试：`TestLegacyPackagingCompletionUsesFrozenBagQuantityWithoutInputQuantity`。

合组提交另暴露了旧路线快照执行配置 `{}` 被按“配置有变化”拒绝；旧实现返回“工艺路线配置已变化，请刷新核对”，导致已通过预览的来源卡仍不能提交。

## 修复后验证（GREEN）

- 单卡旧包装卡从工单冻结数量读取本次袋数；同一工单出现多张未执行卡时仍明确阻断，不猜测分配。
- 空执行快照只作为旧数据占位处理。首次完成会把计算出的执行配置写入工艺快照并添加审计记录；有效配置、未知字段和损坏 JSON 不会被当作空值覆盖。
- 三张隔离旧工单按 10、8、2 袋组成 20 袋包装组。API 预览返回 3 张来源任务和 2 种冻结物料；合组提交逐工单完成，并保留各来源工单自己的耗料与成品入库单据。
- 库存单据核对得到熟豆耗用 4,540g、包装袋 20 个、成品入库 20 袋；完成流水 3 条，合组操作审计 1 条。用相同请求标识重试返回原结果，未增加库存单据或完成流水。

通过命令：

```sh
ORDERAPP_TEST_DATABASE_URL='postgresql:///codex_pr734_roast_test?host=/tmp' go test ./internal/interfaces/http/production -run '^TestLegacyPackagingCompletion' -count=1
go test ./internal/infrastructure/postgres/production -run 'TestIsEmptyLegacyExecutionSnapshot' -count=1
```

结果：两个命令均通过。隔离 API 测试的 GREEN 输出为 `ok orderapp/internal/interfaces/http/production 2.909s`；快照单测输出为 `ok orderapp/internal/infrastructure/postgres/production 0.406s`。测试只写入隔离测试数据库，不修改开发或生产业务工单。

`./scripts/verify_kferp.sh backend` 全量 Go 测试通过；`./scripts/verify_kferp.sh changed` 与 `git diff --check` 通过。改动仅涉及 Go 后端和现有生产操作手册，没有前端源文件改动。

## 尚待完成

- [ ] 合入最新 develop 并部署开发环境后，使用独立测试工单在浏览器点击“完成并入库”，核对库存、成本、单据与日志。
- [ ] 浏览器验收前不进入生产发布；生产真实工单只读，现场人员处理真实完成数量。

手册：`docs/OP_MANUAL_PRODUCTION.md` 中“包装台同规格任务合组”。
