# 2026-05-31 BOM 分组维护与特殊属性迁入 BOM 版本

## 范围
- PR-389-BOM-GROUP-SPECIAL-ATTRS
- BOM 分组补齐维护入口；特殊属性从 SKU/商品配置模板迁入生产 BOM 版本。

## 代码证据
- 后端扩展 BOM 分组 API：`GET /api/production-bom-groups?include_inactive=1`、`POST /api/production-bom-groups`、`PUT /api/production-bom-groups/:id`、`POST /api/production-bom-groups/:id/disable`。
- `production_bom_versions` 增加 `special_attrs_schema_json`、`special_attrs_json`，草稿版本可保存特殊属性，已发布版本只读。
- 旧 SKU 特殊属性回填到 BOM 版本；同一 BOM 版本属性冲突时自动复制 BOM/版本并重新绑定商品。
- 成本核算和生产工单优先读取绑定 BOM 版本特殊属性，旧 SKU 字段仅作 fallback。
- Vue/Vite：BOM 页面新增“管理分组”和“BOM版本与特殊属性”；商品管理和商品配置模板不再渲染特殊属性编辑入口。

## 测试证据
- RED：新增 API/schema/前端源断言后，Go 测试因缺少分组更新、停用、版本特殊属性字段和迁移标记失败；前端 `node --test` 因缺少“管理分组”和 SKU 特殊属性下线断言失败。
- GREEN：补齐后端 schema、service、repository、API、迁移回填、成本/生产读取逻辑和 Vue 页面后，目标 Go 和前端测试通过。
- 按 Van 当前要求，本轮不做浏览器/人工验收。

## 手册证据
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
