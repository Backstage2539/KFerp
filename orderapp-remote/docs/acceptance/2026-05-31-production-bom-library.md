# 2026-05-31 生产 BOM 配方库收敛

## 范围
- PR-388-PRODUCTION-BOM-LIBRARY
- 把商品内嵌 BOM 来源心智收敛为独立生产 BOM 配方库、BOM 分组、BOM 版本和商品档案默认 BOM 版本绑定。

## 代码证据
- 后端新增生产 BOM 库服务、API、表结构和旧数据回填：`internal/application/bom`、`internal/interfaces/http/bom`、`internal/infrastructure/postgres/bom`。
- 商品管理 API 输出生产 BOM 绑定和非最新版提示字段：`internal/application/catalog`、`internal/infrastructure/postgres/catalog_queries.go`。
- 成本核算、生产计划、物料需求和工单优先读取商品档案绑定的生产 BOM 版本。
- Vue/Vite 商品管理和 BOM 页面改为“生产 BOM（配方库）”“BOM分组”“当前引用 Vxxx，最新 Vyyy”口径。

## 测试证据
- RED：新增生产 BOM API、schema 回填和前端文案测试后，Go 编译因缺少生产 BOM 类型/API 失败，前端 `node --test` 因缺少 `productionBomLabel` 失败。
- GREEN：补齐生产 BOM 表、API、回填、绑定、前端 helper 和页面接线后，目标 Go 和前端测试通过。
- 按 Van 当前要求，本轮不执行浏览器/人工验收。

## 手册证据
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OPERATION_MANUALS.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
