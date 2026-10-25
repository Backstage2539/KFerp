# 旧完工工单直接入库验收

## 范围

工单 `WO-PP-0000000114-0000000116` 的两道工序均已完成、累计完工量为 5.6 kg，只有成品入库待处理。生产环境只读，不在真实工单上执行过账。

## 问题与修复

旧工序完成记录没有完整关联到新的工序耗料事件。工单列表的直接入库请求误走旧单核对逻辑，在入库时按冻结配方再次扣料，因此用 3 kg 对比 WIP 可用 2.82 kg 后报缺 0.18 kg。成品入库不应重新执行耗料；当前路径按已记录完工累计量扣除已有入库后，只补入成品，并保留已有成本与产出记录。旧耗料流水缺失时返回核对提示，不重复扣料，也不伪造库存扣减。

## RED / GREEN

- RED：`TestRecordedOutputReceiptDoesNotReconsumeMaterialsWhenWIPIsShort` 在修复前以 WIP 22.52 kg、旧完工记录和 22.7 kg 冻结需求提交，API 返回库存不足 0.18 kg。
- GREEN：同一接口测试在修复后通过；5.6 kg 已记录产出成功入库，WIP 批次位置和预留消耗不变，未生成原料耗料单，只生成一张成品入库单，并返回历史用料核对提醒。
- GREEN：旧单人工核对的原接口测试仍通过，保持其既有补账行为。
- GREEN：发布同款 `env -u ORDERAPP_TEST_DATABASE_URL -u DATABASE_URL go test -p 1 ./...` 全 Go 测试通过；数据库 API 回归用例另以隔离本地 PostgreSQL 运行。
- GREEN：工单列表运行时测试 9/9 通过；全 Vue 测试 1,614/1,614 通过；生产 Vue 构建通过。构建仍提示已有的大体积 PDF 预览 chunk 警告。
- 边界：带本地数据库运行整个生产接口包时，除本次相关用例外还出现多项失败，包括 BOM 规格表缺失的测试夹具，以及计划/排班/普通工序完成测试中的旧供给与状态预期；单独运行旧单核对与直接入库用例均通过。数据库全包失败列表与本次修改路径无关；开发部署前端到端验收仍待完成。

## 验收边界

## 开发环境交付证据

- 代码已合入 `develop`：`ca71a75b797ee097c62944b9324690444e595715`；`origin/develop` 指向同一提交。
- 部署命令：`KFERP_SKIP_MINIAPP_EXPORT=1 ./deploy_orderapp.sh development`，脚本预检和部署成功。
- 回滚锚点：源目录 `/opt/stacks/erp/orderapp.backup.deploy-20261026011914-ca71a75b797e`；镜像 `kferp-orderapp-rollback:development-20261026011914-ca71a75b797e`。
- 运行状态：`erp_orderapp` running、`erp_postgres` healthy；根盘可用约 18 GB，未清理缓存。
- HTTP：`/app/login` 200、`/app/` 303、`/app/vue-shell?view=productionFlow` 200。部署静态资源为 `ProductionFlowView-7GvqZEy3.js`，包含直接入库的新帮助说明；生产操作手册已同步到开发服务器。
- 浏览器只读查看开发环境工单列表，确认目标工单展示“确认数量并入库”及更新后的帮助文字。没有点击写入按钮：此工单不是隔离测试数据，避免改动实际库存。API 隔离数据库测试已验证 5.6 kg 只生成成品入库、没有再次扣原料。

生产工单没有被写入，也没有部署生产。独立浏览器隔离数据的按钮点击验收仍待完成；生产发布继续以该验收为门槛。真实补料、数据修复及业务验收留给现场人员与 Van。
