# 生产计划投料、真实出品率与生产日志设计

## 背景

当前 `生产计划/开始生产` 与 `生产中` 流程存在三个缺口：

1. `生产中` 默认入库只按计划成品克数拆分件数与散装余量，没有明确记录投料数。
2. BOM 维护了出品率，但运行流程没有把“计划投料数 -> 实际产出 -> 真实出品率”串起来。
3. 系统只有操作日志和物料消耗日志，没有独立的生产日志页面用于追溯单次生产结果。

## 目标

为生产流程建立完整闭环：

- 在 `生产计划/开始生产` 页面录入并保存本次生产的投料数(g)。
- `生产中` 页面仍由操作人录入实际成品件数和散装余料(g)。
- 系统根据实际产出自动计算真实出品率。
- 每次完成生产时写入独立生产日志，可在 `生产流程` 菜单下查询。

## 范围

本次只覆盖当前 ERP 的单批次生产流程，不重做整套生产批次模型。

包含：

- 生产计划页面投料数录入与默认值计算
- 运行项持久化计划投料数与 BOM 出品率快照
- 完成生产时真实出品率计算
- 独立生产日志表、页面、查询
- 5 张需求管理表新增对应条目

不包含：

- 多次分段完工/拆批完工
- 手工修改真实出品率
- 返工、报废、损耗单独流程
- 历史批次回填

## 已确认业务口径

1. 投料数在 `生产计划/开始生产` 页面编辑，不在 `生产中` 页面编辑。
2. 投料数默认值按 `ceil(计划缺口成品克数 / BOM出品率)` 计算。
3. `生产中` 页面由操作人填写：
   - 完成件数
   - 散装余料(g)
4. 完成生产时自动计算：
   - 实际产出总克数 = `完成件数 * 规格g + 散装余料g`
   - 真实出品率 = `实际产出总克数 / 投料数`
5. 生产日志入口放在 `生产流程` 菜单下。

## 方案选择

采用“计划数据与结果日志分层”的中间方案：

- `produce_running_items` 继续承载运行中的生产任务。
- 在运行项上新增本次计划投料、BOM 出品率快照等字段，保证执行期数据稳定。
- 新增 `production_logs` 作为完成生产后的事实表，记录最终产出、真实出品率、库存增减和操作者。

不选择整体重做 `produce_batches`，避免把需求扩展成大规模重构。

## 数据设计

### 1. 运行项扩展

扩展 `produce_running_items`：

- `input_g BIGINT NOT NULL DEFAULT 0`
- `bom_yield_rate NUMERIC(10,4) NOT NULL DEFAULT 0.8000`
- `planned_units BIGINT NOT NULL DEFAULT 0`
- `planned_loose_g BIGINT NOT NULL DEFAULT 0`

含义：

- `input_g`：本次计划投料数，来自生产计划页，可手动编辑后入库。
- `bom_yield_rate`：开始生产时锁定的 BOM 出品率快照，避免 BOM 后续改动影响运行中的任务。
- `planned_units` / `planned_loose_g`：基于计划成品克数拆出的默认入库值，用于页面展示和后续比对。

### 2. 新增生产日志表

新增 `production_logs`：

- `id`
- `running_item_id`
- `batch_id`
- `product_id`
- `product_name`
- `spec_g`
- `order_nos`
- `planned_need_g`
- `input_g`
- `bom_yield_rate`
- `finished_units`
- `finished_loose_g`
- `finished_total_g`
- `actual_yield_rate`
- `started_by`
- `started_at`
- `finished_by`
- `finished_at`
- `inventory_units_before`
- `inventory_loose_g_before`
- `inventory_units_after`
- `inventory_loose_g_after`
- `material_summary JSONB`
- `created_at`

说明：

- `material_summary` 存每次生产实际扣减的物料摘要，避免日志页再去拼装多表。
- 日志一旦生成，不可重复写；`running_item_id` 需要唯一约束。

## 页面与交互

### 1. 生产计划/开始生产

当前页面每个选中商品行增加一列 `投料数(g)`：

- 默认值：`ceil(gap_g / bom_yield_rate)`
- 若商品 BOM 未配置或出品率非法，默认按 `0.8`
- 允许人工直接修改
- 点击开始生产时一并提交到后端

后端行为：

- 为每个运行项保存 `input_g`
- 同时保存 `bom_yield_rate`、`planned_units`、`planned_loose_g`

### 2. 生产中

当前页面新增只读列：

- 计划投料数(g)
- BOM 出品率
- 预计成品

录入仍只保留：

- 完成件数
- 散装余料(g)

提交完成生产后：

- 增加成品库存
- 扣减物料库存
- 计算真实出品率
- 写 `production_logs`
- 运行项状态改为 `done`
- 相关订单在全部运行项完成后置为 `生产完成`

### 3. 生产日志

新增菜单：`生产流程 -> 生产日志`

日志页默认展示最近记录，支持筛选：

- 日期范围
- 产品
- 批次号
- 操作人

表格至少展示：

- 完成时间
- 批次
- 产品
- 规格(g)
- 订单号
- 计划成品(g)
- 投料数(g)
- BOM 出品率
- 完成件数
- 散装余料(g)
- 实际产出(g)
- 真实出品率
- 完成人

详情或展开区展示：

- 成品库存前后变化
- 物料扣减摘要
- 开始时间 / 开始人

## 核心计算规则

### 1. 计划投料默认值

`default_input_g = ceil(gap_g / normalized_bom_yield_rate)`

其中：

- `normalized_bom_yield_rate` 在 `(0,1]` 外时取 `0.8`

### 2. 默认预计入库

仍按计划成品克数 `gap_g` 拆分：

- `planned_units = gap_g / spec_g`
- `planned_loose_g = gap_g % spec_g`

### 3. 实际产出与真实出品率

- `finished_total_g = finished_units * spec_g + finished_loose_g`
- `actual_yield_rate = finished_total_g / input_g`

约束：

- `input_g > 0`
- `finished_units >= 0`
- `finished_loose_g >= 0`
- `actual_yield_rate` 允许大于 `1`，不在服务端强行截断；页面照实显示，方便识别录入错误或工艺异常

## 后端改动点

- `production_flow.go`
  - 开始生产时接收每行 `input_g`
  - 保存运行项扩展字段
  - 完成生产时计算真实出品率并写生产日志
- `unprod_summary_page.go` / 对应模板
  - 展示与提交投料数
- `materials.go` / schema 初始化
  - 新增 `production_logs`
  - 扩展 `produce_running_items`
- 新增 `production_logs_page.go`
  - 列表与筛选查询
- `material_consumption.go`
  - 提供本次扣料摘要，复用到 `production_logs.material_summary`

## 前端改动点

- 当前 `生产计划`、`生产中` 是模板页，先在模板页补齐字段
- 同步在 `frontend-vue-shell` 菜单中增加 `生产日志` 入口
- 不在本次把两页整体迁移成 SPA，只做当前需求所需迁移增量

## 日志与审计关系

- `操作日志` 继续记录“谁执行了完成生产”这类动作轨迹
- `生产日志` 记录业务事实，用于追溯与统计
- `物料消耗日志` 继续记录逐个物料库存扣减明细
- 三者并存，各自职责不同，不互相替代

## 测试与验收

### 单元测试

- 计划投料默认值计算
- 真实出品率计算
- 运行项保存投料与 BOM 快照
- 生产日志写入的字段完整性
- 模板包含新增字段和入口

### API / 流程测试

- 开始生产后运行项能查到 `input_g`
- 完成生产后：
  - 成品库存变化正确
  - 物料库存扣减正确
  - `production_logs` 有记录
  - `actual_yield_rate` 正确
- 生产日志页返回 200 且包含关键字段

### 需求管理表

新增并维护：

- PR：生产计划投料、真实出品率、生产日志
- DEV：数据结构、计划页、生产中、日志页、菜单入口
- UT：计算与落库测试
- API：开始生产、完成生产、日志查询
- REV：按本设计验收

## 风险与控制

1. **BOM 后改导致口径漂移**
   - 通过运行项保存 `bom_yield_rate` 快照解决

2. **重复完成生产导致重复记账**
   - `running_item_id` 在 `production_logs` 上做唯一约束
   - 完成生产事务中锁定运行项状态

3. **前端模板和 Vue 壳菜单不同步**
   - 同次改动同时更新模板入口和 `frontend-vue-shell`

4. **历史运行项缺少新字段**
   - schema 迁移给默认值
   - 仅对新开始生产的数据使用完整能力

## 本次不做的延伸项

- 生产日志导出
- 物料损耗单独原因分类
- 真实出品率图表报表
- 单批次多次完工记录
