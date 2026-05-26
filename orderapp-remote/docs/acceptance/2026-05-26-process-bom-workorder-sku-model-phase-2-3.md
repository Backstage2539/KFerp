# 通用制造模型二、三期验收记录

日期：2026-05-26

## 范围
- 二期：工艺模板、工艺路线、工单冻结工艺快照。
- 三期：行业字段模板、工序参数 JSON、工序卡实际损耗记录。

## 验收点
- 商品与配方菜单新增“工艺模板”和“行业字段模板”，均为 Vue/Vite 页面。
- 工艺模板可绑定 SKU、BOM 版本、行业字段模板，维护多道工序，并支持保存草稿、发布、停用。
- 行业字段模板可配置咖啡、服装、鲜果等字段，不把烘焙度、布料损耗或去皮损耗写死到主表。
- 开始生产时，系统读取已发布工艺模板，冻结 `process_snapshot_json` 到工单，并按工艺路线生成工序卡。
- 工序卡可记录计划投入、实际投入、实际产出、实际损耗、损耗率和异常原因，保存后写 `job_card` 操作日志。
- BOM 页面展示关联工艺模板，生产工单展示冻结工艺和工序执行汇总。

## 证据
- 后端：`internal/application/manufacturing`、`internal/infrastructure/postgres/manufacturing`、`internal/interfaces/http/manufacturing`。
- 前端：`ProcessTemplatesView.vue`、`IndustryFieldTemplatesView.vue`、`BomView.vue`、`WorkOrdersView.vue`、`JobCardsView.vue`。
- 手册：`OP_MANUAL_PRODUCTION.md`、`OP_MANUAL_INVENTORY_MATERIALS.md`，并同步到 `orderapp-remote/docs/`。
- 单元/API：`cd orderapp-remote && go test ./... -count=1`。
- 前端构建：`cd orderapp-remote/frontend-vue-shell && npm run build`。
- 前端单测：`cd orderapp-remote/frontend-vue-shell && node --test src/lib/product-settings.test.js`。
- App schema：本地干净库启动时修正 `materials` 在 `bom` 前初始化，避免 BOM 回填物料成本时找不到 `materials` 表。
- 浏览器验收：
  - 登录本地 Vue/Vite 工作台：`http://127.0.0.1:18087/vue-shell`。
  - “行业字段模板”页面保存 `通用加工字段模板-浏览器验收`，字段 `expected_process_loss`，类型 `比例`。
  - “工艺模板”页面保存并发布 `通用制造工艺-浏览器验收`，绑定 `UI通用制造测试SKU2300`、`V001` BOM 版本和行业字段模板。
  - “BOM配方维护”页面选中 SKU 后显示 `关联工艺 1 个模板` 和 `通用制造工艺-浏览器验收 · 已发布`。
  - “工序卡”页面录入实际投入 `10000`、实际产出 `8200`，保存后自动得到实际损耗 `1800`、损耗率 `18.00%`。
  - “生产工单”页面显示冻结工艺快照 `通用制造工艺-浏览器验收`，工序执行汇总显示 `通用加工 running，损耗 1800g / 18.00%`。
  - “操作日志”页面可查到 `industry_field_template 新增`、`process_template 新增/发布`、`job_card update_metrics actual_loss 1800.0000`。
