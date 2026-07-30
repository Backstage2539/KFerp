# PR-560 生产领料紧凑明细与库存诊断验收

## 验收范围

- 工单绑定的生产领料、补料、退料和消耗使用紧凑、对齐的多物料行。
- 原料仓库存不足和质检冻结返回中文逐物料诊断。
- 生产领料预览、已有草稿和提交统一当前剩余 WIP 缺口口径。
- 指定开发环境工单仅做只读核对，不保存或提交库存单据，不开始生产。

## RED 证据

- 前端紧凑布局合同在实现前 3 项失败：缺少工单物料紧凑行、统一列头和桌面/窄屏布局。
- 支持合同在实现前缺少 PR-560 需求、手册、验收和代码标记。
- 后端原料仓库存不足与 WIP 超领回归由定向 PostgreSQL 测试记录。

## GREEN 证据

- 应用层：`TestWorkOrderStockDocumentPreviewRestoresExistingDraft` 验证8000g旧草稿按当前7751g缺口收敛且只改变预览；`TestWorkOrderStockDocumentPreviewRemovesDraftItemsWithoutCurrentWIPShortage` 验证缺口归零行从本次预览移除，原草稿不被静默修改；Kg/lb非整数换算直接保留权威`ShortageG`，不会因浮点往返多1g。
- API：`TestWorkOrderStockDocumentPreviewAPIRefreshesStaleDraftAgainstCurrentWIPShortage` 验证HTTP响应返回7751g、`remaining_qty=7751`及草稿变化提示。
- 临时 PostgreSQL 16：原料仓多物料FIFO不足、质检冻结、冻结快照/历史reservation超领和跨工单退料定向测试通过。多物料用例先处理库存充足行，再在第二行触发中文不足，事务回滚后两项原料仓/WIP余额均保持原值。少量冻结库存不足以覆盖需求时仍判定真实总量不足并附冻结数量；退料/消耗的冻结库存按当前工单实际领入量封顶。
- 并发时序：先保存4000g旧草稿，另一单据先领2000g后，旧草稿提交按事务内最新3000g缺口拒绝；更新为3000g后才可提交。
- 前端：`stock-entry-compact-production-items.test.js`与`production-execution-hub.test.js`共10项通过；Vue/Vite生产构建通过。
- 完整校验：`scripts/verify_kferp.sh backend`全绿；完整前端823项中816项通过，7项均为当前`origin/develop`已有的客户工作区/视图上下文陈旧断言，PR-560新增及相邻生产执行测试全部通过。
- 临时 PostgreSQL 全量stock包另有2项与本改动无关的既有失败：期初批次回填改变旧测试的批次数假设、旧仓库清单测试schema缺少`products.customer_id`。两项均在干净`origin/develop@8df3bada`原样复现；本需求涉及的4组事务测试全部通过。
- 集成与部署：功能分支提交`3487726a`经独立复核后合并为`0f8d3645`，已推送`origin/develop`并部署development。部署期间Vue、miniapp和Docker内完整Go测试通过，容器重建成功；部署前备份为`/opt/stacks/erp/orderapp.backup.deploy-20260728165344`。
- 部署后只读冒烟：开发环境Vue shell、PR/DEV API和指定工单领料预览均正常。工单40返回三项当前缺口1974g/1974g/3948g；工单39返回7751g，并提示旧草稿从8000g按当前缺口调整为7751g。未保存、提交或取消库存单据。
- 浏览器边界：公网development在受控浏览器中被`ERR_CERT_AUTHORITY_INVALID`阻断；未绕过证书。通过只读SSH隧道访问同一已部署容器可到达“系统登录”页且控制台无错误，但未借用业务账号绕过登录。因此桌面10条同屏的源码合同、定向测试和构建已通过，最终登录态视觉验收仍由Van手工确认。

## 真实工单只读诊断

- `WO-PP-0000000083-0000000051`（内部 id 40）：当前三项 WIP 缺口和草稿 `SE-0000000004` 均为哥伦比亚1974g、耶加雪菲G2 1974g、黄波旁水洗3948g。原料仓合格 FIFO 可用依次为0g、120000000g、1500000g，无冻结/拒收批次，因此库存不足判定合理，失败物料是哥伦比亚。库存中的“哥伦比亚EP”是另一个 material id，不能按相似名称替换工单冻结快照中的 BOM 物料。
- `WO-PP-0000000080-0000000050`（内部 id 39）：冻结需求和当前剩余 WIP 缺口为7751g，无 WIP、reservation 或已提交领料；旧草稿 `SE-0000000005` 保存8000g，超过当前上限249g。现有草稿读取未合并当前 `required_qty / remaining_qty`，导致前端没有 `max` 上限，错误被拖到提交阶段；该物料当前原料仓合格可用同样为0g。
- 相关物料在用户报告前后的库存流水无变化，以上结论不是事后库存变动造成。若业务需要改用另一个物料，应正式修正 BOM 并按工单生命周期重建；本修复不得模糊匹配替换冻结物料。
- 诊断过程不得创建、保存、提交或取消真实库存单据，不得开始生产。

## 交付边界

- 不新增数据库字段，不重写历史工单、库存批次、库存流水或已提交 Stock Entry。
- 多物料提交继续原子过账；任一物料失败时不产生部分库存、流水、工单统计或操作日志。
- 只部署 development，production 不部署。
- development部署完成后未执行任何真实库存写入；PR保持review，等待登录态页面人工验收。
