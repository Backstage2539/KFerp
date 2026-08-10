# PR-593 BOM 损耗与包材固定单位

## 业务合同

- 开启 BOM 版本级原料损耗后，页面分为“损耗原料”和“非损耗物料（含包材）”。
- 损耗原料使用 `比例 %`，版本损耗只写入这些物料行。
- 包材、固定数量物料和商品组件使用个、件、袋、盒等全局单位固定用量，行损耗为 0；两类组件可在同一 BOM 共存。
- 仍使用既有 `consume_unit / qty_per_unit / ratio_pct / material_loss_rate` 字段，不迁移或回算历史 BOM、工单、库存流水和价格快照。

## 自动验证

- RED：应用层拒绝 `个`，生产需求把 `个` 降级为比例，Vue 损耗开启后只显示比例单位，PostgreSQL 仓储重复拒绝非比例行。
- GREEN：BOM application、draft API、PostgreSQL BOM 仓储合同、生产固定包材需求、Vue BOM 定向测试及 PR-593 support 合同通过。

## 人工验收

- [ ] Van 在 development 打开带原料损耗的 BOM，确认两个编辑区域同时可用。
- [ ] 在损耗原料区保存比例原料，在非损耗区保存包材固定用量，刷新后单位、数量和分区保持正确。
