# PR-550 销售规格保存与销售单规格、数量、备注修复

日期：2026-07-24
环境：功能分支验证完成；development 待部署
production：不部署，不改业务数据，不重建历史销售单

## 用户问题

1. `销售规格明细` 新增规格保存时报 `conn busy`。
2. 新价格表件数订单的销售单把规格 `1Kg` 显示成 `1Kg/1Kg`，把数量 `30` 显示成 `301Kg`。
3. 订单没有订单级备注，商品行备注却又在结算区重复显示；商品行备注末尾的 `袋` 被截断，且不能安全换行。

## 根因

- 销售规格模板保存同步派生 SKU 时，在同一 pgx 事务连接的查询结果集仍打开期间发起下一次查询或更新，触发连接忙。
- 订单和价格表快照已经保存正确的规格和件数，销售单渲染器仍无条件追加销售单位。
- 结算区把商品行备注再次汇总为 `订单明细备注`。
- `gofpdf.SplitText` 在中英文混合文本恰好命中列宽边界时可能吞掉最后一个中文字符，且旧算法没有扣除单元格左右边距。

## RED

- `TestSyncDerivedSKUsForTemplateBuffersParentIDsBeforeNestedQueries`、`TestSyncDerivedSKUsForParentBuffersChildrenBeforeStatusUpdates`：证明父商品/子 SKU 查询结果集未关闭时进入嵌套读写。
- `TestSalesOrderItemCellsKeepPublishedSalesSpecCountSeparate`：修复前得到 `1Kg/1Kg`、`301Kg`。
- `TestSalesOrderWrapCellTextPreservesEveryRune`：修复前 `2.5Kg袋装，共12袋` 丢失最后一个 `袋`。
- 结算行测试证明商品行备注被重复生成 `订单明细备注`。

## GREEN

- 同步代码先把父商品 ID 和子 SKU 状态读入内存、关闭并检查结果集，再进入后续查询/更新。
- 销售单快照冻结 `quantity_basis`；`sales_spec_count` 新快照显示纯规格和纯件数，缺少标记的历史快照继续旧兼容。
- 商品备注仅留在商品行；结算区保留真实快递费备注和销售单备注。
- 使用按 rune 安全切分、扣除单元格内边距并尊重显式换行的统一包装函数。
- 一次性 PostgreSQL 16 临时集群内，`TestSaveProductUnitTemplateSyncsDerivedSKUsWithoutBusyConnection`、仓储快照测试和 HTTP API 快照测试 3/3 通过；测试只创建并自动删除随机 schema，不连接业务库。
- 领域、PDF、catalog、sales、销售单 API 和支持合同定向 Go 测试全部通过。
- `scripts/verify_kferp.sh backend` 完整后端通过，`scripts/verify_kferp.sh changed` 与 `git diff --check` 通过。
- 录单/销售单定向前端测试 132/132 通过；Vue/Vite production build 通过（402 modules）。
- 全量前端测试为 802/809，7 条失败均在本分支未改动的 customer workspace 静态合同；本分支相对 `origin/develop` 的 `frontend-vue-shell` 差异为空，不把这组既有失败混入本修复。
- 独立代码审查确认两层 pgx 结果集、历史兼容、PDF/PNG 换行和备注语义无 P0-P3 问题。

## 视觉证据

- 合成预览 PDF：`/private/tmp/kferp-pr550-artifacts/pr550-sales-order-spec-preview.pdf`
- 合成预览 PNG：`/private/tmp/kferp-pr550-artifacts/pr550-sales-order-spec-preview.png`
- Poppler 渲染页：`/private/tmp/kferp-pr550-artifacts/pr550-sales-order-spec-preview-rendered.png`
- 已人工检查：规格为 `1Kg`、数量为 `30`；`2.5Kg袋装，共12袋` 完整换行；没有重复订单明细备注，也没有遮挡、越界或丢字。

## 兼容和数据边界

- 历史订单快照、历史销售单 PDF/PNG 和公开文件不迁移、不重写。
- 修复只影响新预览和修复后新生成的销售单版本。
- 本次不保存截图中的预览订单，不创建、修改或重新发布价格表，不操作 production。

## 部署与冒烟

- 功能分支、`develop` 合并提交、development 备份：待记录。
- development API、浏览器、容器健康和日志：待记录。
