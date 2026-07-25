# PR-552 库存作业与 Stock Entry 收敛验收记录

## 范围

- 库存写入统一到 Stock Entry 草稿、提交、取消生命周期。
- 生产工单通过统一库存单据执行领料、补料、退料、消耗和完工。
- 库存作业只保留库存单据和盘点调整；历史记录只读兼容。
- 本次仅部署 development；production 和历史差异修复不在范围内。

## RED

- 在统一模型尚未实现时执行 `go test ./internal/application/stock -run TestCreateStockDocumentNormalizesLegacyReturnPurpose -count=1`，编译失败并报告 `undefined: StockDocumentCommand`、`undefined: StockDocumentDetail` 等，证明新生命周期合同尚不存在。
- 真实 PostgreSQL 首轮闭环测试进一步复现生产退料和消耗的 `conn busy`：FIFO 结果集尚未关闭时，同一事务连接又查询工单可退批次数量。实现先缓冲并关闭 FIFO 游标，再计算批次上限后，该错误消失。

## GREEN

- `go test ./... -count=1`：完整 Go 测试通过。
- 真实 development PostgreSQL 临时 schema：
  `go test ./internal/infrastructure/postgres/stock -run TestUnifiedStockDocument -count=1 -v`，6/6 通过，覆盖草稿不改库存、提交幂等、取消、FIFO、冻结批次、跨工单退料、可退限制、完工、旧写接口转发及平行表零新增。
- `go test ./internal/interfaces/http/production -run 'TestUnifiedStockDocumentHTTPLifecycle|TestWorkOrderStockDocumentPreview' -count=1 -v`：HTTP 生命周期和工单预填通过。
- `go test ./internal/interfaces/http/support -run TestDev552StockEntryConvergence -count=1 -v`：需求、页面、菜单、操作日志和手册合同通过。
- 定向前端 65/65 通过；Vue/Vite production build 通过（395 modules）。完整前端回归为 802/809，7 条失败均为当前 `origin/develop` 已存在的 customer workspace 静态合同（本需求未修改对应 App、客户门户、BOM 或试算作用域实现）。
- `scripts/verify_kferp.sh changed` 和 `git diff --check` 通过。

## 数据差异报告

- 部署前在 development 执行 `scripts/report_stock_entry_differences.sql`，仅运行 `SELECT`：
  - 物料主档余额与批次剩余量：8 条待核对差异。
  - 批次剩余量与仓位合计：7 条待核对差异。
  - 最新库存流水 after 值与当前仓位余额：37 条待核对差异。
  - 历史平行单据：原料入库 11 条、物料转仓 20 条、成品转仓 0 条。
- 报告明细只保存在服务器临时文件 `/tmp/pr552-stock-difference-predeploy.txt`，未写入仓库、未修改库存，也未自动补单。迁移前因没有统一生命周期列，统一 SE 缺流水检查按设计跳过；部署后重新执行。

## 浏览器与 API 验收

待部署后补充只读 API 与浏览器页面冒烟。真实库存写入闭环已在自动清理的临时 schema 完成，不在 development 业务 schema 自造库存或工单。

## 部署

待补充 development 提交、备份、容器、数据库和页面冒烟；production 保持不变。
