# 包装补料、生产需求显示与工位产能验收

## 范围

- 包装合组缺料时在工位打开现有领料抽屉，选择整组任务并保留预览、库存、货权、幂等及审计约束。
- 待计划需求异常正常换行，删除库存充足提示区，不改变库存覆盖判断或需求筛选。
- 工位产能标准分钟仅接受非负整数，0 和空值兼容，小数不取整、不保存，前后端给出明确提示。

## 验证证据

- RED：自动工序三卡点击补料后抽屉未打开；需求表状态列缺宽且库存充足面板仍展示；新建与编辑产能输入 0.1 时仍发请求，API 返回 `invalid request`。
- GREEN：Vue 全量 1779 项通过；`scripts/verify_kferp.sh backend`（未配置业务数据库）通过；`scripts/verify_kferp.sh frontend-build` 通过。
- 隔离 PostgreSQL 生产 API：`TestPackagingGroupRefillThreeSimplifiedCards44Units` 检查三张简化包装卡合计 44 个包材，领料到 WIP、同请求重复提交不重复领料、无开工和耗料、业务审计与操作日志；批量领料 API 和补料后开工回归通过。
- Chrome 使用真实 Vue 表格和样式的代表性数据进行布局验证：720px 和 1366px 窗口下异常正文宽 240px、两行，`20 pic` 单行，仅表格区域横向滚动；截图在 `/tmp/kferp-demand-layout-20261109/`。
- 工位产能前端与 HTTP 测试覆盖完整用户请求、0、整数、空值、小数、负数、无效类型以及新增和编辑。
- 手册：`OP_MANUAL_PRODUCTION.md`、`OP_MANUAL_STOCK.md`；Vue 手册读取相同 Markdown 来源。

## 交付边界

- 未改真实订单、库存或生产业务数据；未部署。等待用户在目标环境确认实际业务场景。
- 已知历史部分完成且投入量为零的旧包装卡补料规则不在本次修复范围。
