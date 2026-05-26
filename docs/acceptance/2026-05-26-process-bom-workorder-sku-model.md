# PR-375-PROCESS-BOM-WORKORDER-SKU-MODEL 验收记录

## 范围
- SKU、BOM、工艺、工序、工单和模板按通用制造模型重新划分边界。
- BOM 用户侧维护预期损耗率，系统换算预期产出率，底层继续兼容 `yield_rate`。
- 工序卡记录实际投入、实际产出、实际损耗、实际损耗率和异常原因。
- 工单展示冻结的 BOM 版本、预期损耗、计划投料、预计产出和工序实际损耗汇总。
- 一期不新增完整工艺模板页面，只完成口径统一、BOM 预期损耗、工序卡实际损耗和工单快照展示。

## 验收点
- BOM API 返回 `expected_loss_rate`、`expected_yield_rate` 和兼容字段 `yield_rate`。
- BOM 保存 `expected_loss_rate` 时换算写入 `yield_rate`，非法损耗率会被拒绝。
- 工序卡 API 返回并可更新计划投入、实际投入、实际产出、实际损耗、实际损耗率、异常原因和扩展指标。
- 工单 API 返回预期损耗、预期产出、BOM 版本和工序实际损耗汇总。
- SKU/BOM/成本/生产页面使用通用制造口径，不再把模型写死为咖啡烘焙。
- 用户触发的 BOM 损耗率保存和工序卡实际损耗更新写入操作日志。

## 自动验证
- `go test ./internal/domain/production ./internal/application/bom ./internal/interfaces/http/bom ./internal/interfaces/http/production ./internal/domain/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
- `node --test src/lib/manufacturing-loss.test.js`
- `npm run build`

## 浏览器验收
- 进入 Vue/Vite 的 SKU 设置与 BOM 配方维护，选择一个 SKU，确认页面显示“预期损耗率”和换算后的“预期产出率”。
- 在 BOM 页面维护预期损耗率，确认保存后详情、版本和列表都按通用口径展示。
- 进入生产工序卡，录入实际投入、实际产出和异常原因，确认页面计算实际损耗和实际损耗率并可保存。
- 进入生产工单，确认能看到冻结的 BOM 预期损耗、计划投料、预计产出和工序损耗汇总。
- 进入操作日志，按 BOM 或工序卡对象查询，确认能看到对应保存/更新记录。

