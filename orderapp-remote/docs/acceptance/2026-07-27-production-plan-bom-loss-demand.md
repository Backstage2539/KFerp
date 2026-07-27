# PR-557 Production Plan BOM Loss Demand

## 业务口径

- 订单冻结规格换算是成品需求唯一来源；如目达摩14件454g得到6356g。
- 生产计划只读取解析到的当前有效已发布 BOM。具体SKU无BOM时继承订单冻结父商品BOM，草稿版本不参与排产。
- BOM原料损耗只应用一次：`6356 ÷ (1 - 18%) = 7751.2195g`，系统按整克四舍五入冻结7751g。
- 设备标准批量只用于拆分产能，不改变理论物料需求。

## RED

- PostgreSQL API预览场景中，旧逻辑把6356g按旧80%产出率放大并按设备整批得到8000g。
- 创建草稿场景中，旧逻辑冻结7945g；带18%物料损耗的快照在汇总时再次放大到9453g。
- 前端源码测试发现当前计划预览仍包含`计划投料(g)`表头和`row.input_g`单元格。

## GREEN

- `TestProducePlanSummaryAPIUsesInheritedPublishedBomLossOnceWithoutMachineRounding`：预览冻结父BOM V004、18%损耗和7751g生豆需求。
- `TestProductionPlanAPIAppliesInheritedPublishedBomLossOnce`：新草稿计划投入7751g、成品需求6356g，物料快照和汇总不重复损耗。
- `TestProductionPlanAPIKeepsSameSKUWithDifferentFrozenParentsIsolated`：同一 SKU/规格的两个冻结父商品分别使用10%和20%损耗及各自物料，摘要、配方和库存需求不串用。
- `TestAggregateProductionPlanMaterialSummaryDoesNotApplyBomLossTwice`：冻结标记阻止计划详情/WIP需求二次放大。
- `TestPlannedInputGramsFromMaterialLossUsesBomLossOnce`：固定18%损耗换算和无损耗口径。
- `node --test src/lib/produce-plan.test.js`：当前预览删除计划投料列，12%/18%/20%损耗摘要格式保留。

## 开发数据只读核对

- BOM-000644当前V003为published，BOM损耗0；V004仍为draft，BOM及物料行损耗实际为18%。
- PP-0000000077冻结V003，planned_output_g=6356、planned_g=7945；本需求不自动修改或撤销该草稿。
- 发布V004后，由用户撤销旧草稿并重新生成，才能得到基于V004的7751g新草稿。

## 数据影响

- 不新增数据库字段，不改写历史计划、工单、BOM、库存或生产日志。
- 新草稿仅在冻结组件JSON中增加`input_includes_material_loss`兼容标记。
- production不部署。

## 验证结果

- production domain/application/infrastructure/HTTP 全包与支持合同通过。
- 当前计划前端定向测试46/46通过，Vue/Vite生产构建通过。
- 全部前端测试为810/817；7项失败与干净`origin/develop`基线完全一致，均位于既有workspace/customer上下文测试，不涉及本需求文件。
- `git diff --check`、冲突标记检查和独立二次代码审查通过。
- 全仓Go测试仍受既有非production临时schema合同失败影响；本需求涉及的production包均独立全绿。

## 集成

- 功能提交：`bd08360b`。
- 最新`origin/develop`基线：`b7388726`；无冲突合并提交：`dbed1beb`。
- 合并态重新通过`verify_kferp.sh changed`、production domain/application/infrastructure/support Go全包、HTTP production包及前端定向46/46。
- 已推送到`origin/develop`；本轮不部署development或production。
