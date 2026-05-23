# PR-349-ORDER-ENTRY-BEANLIST-SUMMARY-WARNING

## 范围
- 录单/编辑订单商品明细标题区把熟豆、生豆、挂耳豆单分别按行展示，长版本号和发布时间不能被压成一列或省略到看不清。
- 切换到历史豆单版本后，已有商品行要先同步当前选择的豆单发布 ID，再重新匹配商品梯度、单价和行级版本号。
- 行级豆单发布不是当前客户、当前豆单类型最新发布时，右侧“豆单版本”标红并显示感叹号；鼠标悬停或手机点击提示“非新版本豆单”。

## 验收证据
- 单元测试：`node --test src/lib/order-entry.test.js` 覆盖 `latestBeanListVersionOption` 按发布时间/版本号识别最新豆单，覆盖旧版 publication 触发 stale warning。
- 前端静态测试：`OrderEntryView shows selected bean lists as readable rows and refreshes row versions from selection` 覆盖豆单摘要按行渲染、移除单行省略、切换版本前先同步行级 publication 再取价。
- 支持模块测试：`go test ./internal/interfaces/http/support -run TestDev349 -count=1` 覆盖需求种子、手册、验收文档和 Vue 接线。
- 手册：`OP_MANUAL_ORDER_SALES.md` 记录商品明细标题区按行展示豆单、切换历史版本后行级价格/版本同步，以及非最新版本红色感叹号提示。

## 验收点
- 商品明细标题区能同时看到熟豆豆单、生豆豆单、挂耳豆单三行完整版本信息。
- 在“选择豆单”抽屉里把某一类型改为旧版本后，已有该类型商品行的梯度、单价、小计和豆单版本都来自旧版本发布快照。
- 旧版本不是该类型最新发布时，右侧豆单版本显示红色感叹号；点击或悬停能看到“非新版本豆单”。
