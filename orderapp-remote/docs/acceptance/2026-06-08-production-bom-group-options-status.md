# PR-448 生产 BOM 目标分组与商品档案 BOM 状态

## Scope

- 生产 BOM 批量移动时，目标分组应能选择分组管理中已经维护的业务分组项，不能只剩“未分组”。
- 商品档案配置抽屉的“被哪些 BOM 使用”需要展示 `BOM状态`：`默认状态`、`启用状态`、`失效状态`。

## RED

- `node --test src/lib/product-settings.test.js`：生产 BOM 目标分组选项对已有非 BOM 用途分组返回空；商品档案抽屉缺少 `BOM状态` 和状态 helper。
- `go test ./internal/interfaces/http/bom -run TestProductionBomProductUsageAPIReturnsOutputAndComponentBoms -count=1`：`ProductionBomUsedByBom` 缺少 `BomStatus` / `IsDefault`。
- `go test ./internal/infrastructure/postgres/catalog -run TestBusinessGroupAssignmentsSupportStringObjectRefsAndAudit -count=1`：assignment 保存路径缺少自动补分组用途逻辑。

## GREEN

- `node --test src/lib/product-settings.test.js`：124/124 passed。
- `go test ./internal/interfaces/http/bom -run TestProductionBomProductUsageAPIReturnsOutputAndComponentBoms -count=1`：passed。
- `go test ./internal/infrastructure/postgres/catalog -run TestBusinessGroupAssignmentsSupportStringObjectRefsAndAudit -count=1`：passed。
- `node --test src/lib/bom.test.js src/lib/product-settings.test.js`：137/137 passed。
- `go test ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/interfaces/http/support -count=1`：passed。
- `npm run build` in `orderapp-remote/frontend-vue-shell`：passed，保留既有 Vite chunk-size warning。
- `go test ./...` in `orderapp-remote`：passed。
- `scripts/verify_kferp.sh changed`：passed。
- `git diff --check`：passed。

## Manual

- 更新 `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`：生产 BOM 目标分组可选择已有业务分组，保存时自动补 `production_bom` 用途并写日志；商品档案 BOM 使用区说明 BOM 状态。
- 更新 `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`：生产 BOM 分组来自通用分组管理，商品档案只读展示 BOM 状态。

## Pending

- Van product acceptance。

## Deployment

- Feature branch pushed and merged to `develop`。
- `origin/develop=49ca2224c7ae4d166e3573a050f1455c7578a1cf` deployed to development.
- Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260608112250`。
- Deploy script evidence: Vue shell build passed, miniapp typecheck/build and `build:mp-weixin` passed, Docker build ran container-internal `go test ./...` and restarted `erp_orderapp`。

## Browser Acceptance

- 生产 BOM：目标分组选项包含 `商品分组 / 商品-咖啡熟豆`、`商品分组 / BOM-咖啡熟豆`、`商品分组 / BOM1`，不再只有 `未分组`。Screenshot: `/tmp/pr448-bom-target-group.png`。
- 商品档案：打开 `SKU-000539` 商品档案配置抽屉，`被哪些 BOM 使用` 区展示 `BOM状态：默认状态`。Screenshot: `/tmp/pr448-product-bom-status.png`。
- Browser console errors: 0。
