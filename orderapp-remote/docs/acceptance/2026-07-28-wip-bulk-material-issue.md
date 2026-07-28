# PR-561 工单批量领料与 WIP 建议量验收

## 业务结论

- 工单逐物料 WIP 缺口继续用于执行枢纽阻断、首次领料默认值和页面提示，但不再限制原料仓到 WIP 的实际领料数量。
- 仓管可按 60Kg 等实际标准批量领料。超过工单当前需求的部分只增加 WIP 库存，不增加工单生产消耗；实际消耗继续使用独立生产消耗单据。
- 同一工单同一物料最近一次已提交的领料量作为下一次标准批量默认值；库存单位为 g 时，60Kg 录入为 60000g。
- 提交仍校验工单物料归属、库存单位和真实原料仓合格 FIFO/指定批次实存。真实库存不足时返回中文逐物料明细，多物料整单回滚。
- 生产消耗仍受工单尚未消耗的冻结物料需求约束，不能把提前领入 WIP 的整批库存误记成本工单消耗。

## RED

- `TestWorkOrderStockDocumentPreviewRestoresExistingDraft`：旧实现把 8000g 草稿裁剪为 7751g。
- `TestWorkOrderStockDocumentPreviewPreservesDraftItemsWithoutCurrentWIPShortage`：旧实现把当前缺口为 0 的草稿物料从预览删除。
- `TestWorkOrderStockDocumentPreviewKeepsZeroSuggestionMaterialAvailableForBulkIssue`：旧实现对无草稿且建议量为 0 的工单返回 `no stock document items available`。
- `TestWorkOrderStockDocumentPreviewAPIPreservesBulkDraftAndReportsSuggestion`：HTTP 预览仍返回被裁剪的 7751g。
- `stock-entry-compact-production-items.test.js`：旧页面仍包含基于 `remaining_qty` 的 `max` 和保存时“超过当前剩余 WIP 缺口”硬错误。
- `TestDev561WIPBulkIssueContracts`：实现前缺少 PR-561 需求、验收、手册和支持合同标记。

## GREEN

- 应用与 HTTP 预览定向测试通过：新预览默认建议量，已有 8000g/60Kg 草稿保留原数量，建议量为 0 时仍保留可编辑物料。
- 前端定向测试通过：不再设置建议量最大值，不再因超过建议量阻止保存；页面显示“建议量仅用于默认填充、超出部分保留为 WIP、生产消耗另记”。
- 前端保存忽略未填写的零数量行，全部为 0 时给出中文提示；从库存单据列表重新打开生产草稿时经过工单预览刷新建议量和警告。
- 临时 PostgreSQL 16 定向测试通过：
  - 冻结物料快照工单允许领用数量高于建议缺口。
  - 历史 reservation 工单允许提交高于当前建议量的旧草稿。
  - 三种物料分别按 1974g、1974g、3948g 需求，可在同一张 `SE-*` 中各领入 60000g；生产消耗仍只能按工单剩余冻结需求提交。
  - 超过真实原料仓实存仍返回中文 `原料仓库存不足`，并保持原料仓/WIP 不发生部分变更。
  - 原有多物料 FIFO、质检冻结和整单回滚测试继续通过。
- 完整 Go 测试通过：`GOFLAGS='-p=1' ./scripts/verify_kferp.sh backend`。
- 前端定向测试通过 14/14，Vue/Vite 生产构建通过；全量前端为 815/823，与干净 `origin/develop` 已确认的 8 项既有失败相同，本需求未新增失败。
- 最终只读代码复审无 P0/P1；确认具体草稿 ID、历史消耗、负数量、计数物料 WIP 批次和多物料事务边界。

## Development 交付与只读冒烟

- 功能提交 `33ac6a25a64da67283423fb2411875e1a9969b88` 已通过无冲突集成提交 `3227a0a1b3d745df4c5400b9c5eda533c86f5bb7` 合入并推送 `origin/develop`。
- `./deploy_orderapp.sh development` 成功；部署时 Docker 构建内完整 Go 测试通过，开发环境应用备份为 `/opt/stacks/erp/orderapp.backup.deploy-20260729010445`。
- `erp_orderapp`、`erp_postgres` 和 `erp_docconvert` 正常运行，应用无重启；认证入口返回预期跳转，Vue shell 和 `index-CJz3ddQX.js` 均返回 200。
- 指定工单 `WO-PP-0000000083-0000000051` 只读预览返回 3 行：建议量 1974g、1974g、3948g，已有草稿 60000g、60000g、60000g 均完整保留，并逐行说明超出部分留在 WIP。预览前后工单库存单据列表一致，未保存、未提交、未过账。
- 实时只读诊断确认 BOM 引用的 `哥伦比亚`（物料 ID 14）当前没有原料仓批次；原料仓存在库存的是另一档案 `哥伦比亚EP`（ID 2），系统继续按具体物料身份校验，不会自动替代。
- 公网自动化浏览器访问开发站点被证书链 `ERR_CERT_AUTHORITY_INVALID` 阻断，未绕过证书；因此页面登录态手工验收仍未标记通过。

## 交付边界

- 不新增数据库字段，不改写历史库存单据、工单、批次和库存流水。
- development 部署后只做 API 和页面只读冒烟，不为 `WO-PP-0000000083-0000000051` 自动保存或提交 60Kg 领料。
- production 不部署。
