# PR-634 商品与分类复制到客户验收记录

- 分支：`codex/customer-catalog-copy-20260907`，基于 `develop 5496efc4`。
- DEV-634-CATALOG / DEV-634-PRICING / DEV-634-DELIVERY；产品状态 review，等待 VA 验收。
- 范围：development。生产部署、微信上传/审核不在本次发布范围。

## 自动验证

- RED：目录仓储/API/前端测试分别记录新契约缺失；报价草稿刷新测试记录丢失继承报价的失败。原始日志 `outputs/pr634-customer-catalog/pr634-*-red.log`。
- GREEN：PostgreSQL 16 独立测试 schema 实测全量（只启用公共）、所选/单个、完整祖先路径、未分类、排除空分类、重复/并发幂等、停用恢复、跨客户改名、整批失败无日志、旧引用预览/补齐/幂等。
- 回滚脚本实测恢复原分类链接/排序；迁移后分类被改名则整批拒绝回滚。`scripts/sql/pr634_customer_catalog_rollback.sql` 根据迁移操作日志执行，日志保存迁移前后引用及新建节点。
- API 测试：复制请求校验、客户目录读取、内部岗位权限、禁止客户账号维护/越权读取、仅管理员迁移。
- Vue 测试：三入口统一复制接口、客户目录投影、别名不改主档、精确商品/规格报价匹配、保留客户表与草稿、缺报价、刷新保留调整。完整 Go/Vue 测试及 Vite 构建通过；小程序 235 项、类型检查和 development 构建通过。
- 网页/小程序录单继续使用同一默认价格表筛选服务；既有按类型客户表优先、公共回退、规格快照、历史价格、预付款与订单复制回归由完整测试覆盖。

## 开发环境验证

待本分支合入及开发部署后，补入独立验收客户标识、页面截图、接口与数据库数量核对、迁移操作日志和发布结果。

## 数据与回滚

复制事务只写 `product_customer_references`、`customer_product_catalog_nodes` 和 `audit_logs`。商品、BOM、规格、组件与库存保留共用身份；运行迁移前后独立核对主档数量与库存指纹。
迁移接口 `POST /api/product-settings/customer-catalog/migration` 默认 `apply=false` 预览，显式 `apply=true` 执行。回滚时传入数据库 schema 与迁移日志 ID，发现分类/引用后续变化立即中止，禁止覆盖后续业务操作。

## 手册

`OP_MANUAL_INVENTORY_MATERIALS.md`、`OP_MANUAL_COSTING.md`、`OP_MANUAL_ORDER_SALES.md`、`OP_MANUAL_MINIAPP_EMPLOYEE_ERP.md`；通过现有 Vue 手册入口提供。

## 首轮开发环境实测（2026-09-07）

- development `18b71539`，旧引用 15 条：预览 15、执行 15、复查 0；迁移日志 8484。迁移前后商品/BOM/规格/库存指纹一致。
- 独立客户 A=300、B=301；页面搜索“初晓”后全量复制到 A，实际创建 86 个有效公共商品引用，日志 8485；页面复选商品 1063/594 到 B，单个商品 1084 到 B。
- A 商品 1063 改为“PR634客户初晓”，分类“意式咖啡”改为“PR634客户意式”；目录与价格表选品一致。
- 发现并修复：成功消息未渲染、价格表筛选只取旧履约范围、平铺报价行仍展示主档名、客户未分类商品缺少选品入口。追加 RED/GREEN 回归并重新部署后复测。
