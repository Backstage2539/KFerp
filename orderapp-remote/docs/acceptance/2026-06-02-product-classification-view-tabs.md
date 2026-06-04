# PR-394 商品/客户商品分类视图验收证据

## 范围
- 商品档案和客户商品不再直接引用分类模板。
- 分类模板作为页面启用的分类视图：启用一个模板新增一个 Tab。
- 商品档案页和客户商品页在当前模板 Tab 内勾选对象并移动分类。
- 分类模板页只维护模板结构和分类项，不配置客户或对象归类。
- 客户商品层删除生产/BOM 操作入口。
- 生产 BOM 返回商品档案配置改为前端内存态临时返回导航，刷新后消失。

## RED 证据
- Frontend：新增分类视图测试后，`node --test src/lib/product-settings.test.js src/lib/bom.test.js src/lib/view-routing.test.js` 最初失败，缺少 `buildClassificationTemplateUsagePayload`、分类模板 Tab/group helper 和生产 BOM 返回导航参数支持。
- API：新增接口测试后，catalog API 最初失败，批量客户商品仍接受 `classification_template_id`，分类模板保存仍保留 `customer_id`，并且商品档案/客户商品分类模板启用 API 不存在。
- Support markers：现有支持测试仍检查旧“分类模板下拉/配置分类抽屉”模型，最初失败，说明文档和源码标记仍是上一版口径。

## GREEN 证据
- `node --test src/lib/product-settings.test.js src/lib/bom.test.js src/lib/view-routing.test.js`：通过，110/110。
- `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1`：通过。

## 最终验证
- `npm run build` in `orderapp-remote/frontend-vue-shell`：通过；仅保留既有 Vite chunk-size warning。
- `scripts/verify_kferp.sh changed`：通过，退出码 0。
- 合并前同步 `origin/develop` 后复跑关键检查：待合并前补充。

## 手册和需求
- 已更新物料/BOM、成本、生产、客户履约手册。
- 已更新 `REQUIREMENTS.md` 和 `ACCEPTANCE_TESTS.md`。

## 人工验收
- Van 明确要求本轮不做浏览器/人工验收，节省 token；本轮只保留代码、文档、单测、API 测试、构建和脚本验证证据。
