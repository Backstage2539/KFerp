# PR-393 商品分类模板与配置抽屉改造验收记录

## 范围
- 商品档案只引用分类模板，分类归属改由“配置分类”抽屉维护。
- 客户商品名单个维护和批量添加商品档案支持分类模板；批量不选时复制/复用来源商品档案分类模板到客户侧。
- 行业字段值只能来自行业字段模板定义，商品档案配置不再临时新增或删除字段定义。
- 商品档案配置进入生产 BOM 后，可通过“返回商品档案配置”恢复当前商品配置抽屉。

## RED 证据
- `node --test src/lib/product-settings.test.js src/lib/bom.test.js` 初次失败：旧实现缺少分类模板抽屉、客户商品名批量分类模板、BOM 返回入口等 PR-393 断言。
- `go test ./internal/interfaces/http/support -run TestDev393 -count=1` 初次失败：PR-393 需求种子、schema/API/Vue 标记缺失。

## GREEN 证据
- Frontend: `node --test src/lib/product-settings.test.js src/lib/bom.test.js`
- Backend/API: `go test ./internal/application/catalog ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1`
- Build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing Vite chunk-size warning.
- Changed verifier: `scripts/verify_kferp.sh changed` exited 0.

## 手册
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md`
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`

## 说明
- 本轮按 Van 当前要求不做浏览器/人工验收，只保留代码、单测、API 测试、构建和 changed verifier 证据。
- 不做破坏性迁移，不回改历史订单、价格表、BOM、工单或旧分类字段。
