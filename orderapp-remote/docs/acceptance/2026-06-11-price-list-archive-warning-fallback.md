# PR-470-PRICE-LIST-ARCHIVE-WARNING-FALLBACK

## Scope
- 商品价格表商品行 `未设置计价方式` 警示按价格表最终解析结果过滤；价格表默认计价模式托底有效时不误报。
- 已发布价格表版本支持多选归档；归档列表支持移出归档。

## RED Evidence
- Frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` failed because `visibleItemWarnings` and publication archive UI were missing.
- API/service: `go test ./internal/interfaces/http/costing -run TestBeanListPublicationArchiveAPI -count=1` and `go test ./internal/application/costing -run TestArchiveBeanListPublicationsValidatesIDsAndOwner -count=1` failed because archive commands and service methods were missing.
- Repository/support: archive repository functions and PR-470 markers were missing.

## Acceptance Checklist
- 商品行显示 `继承分类`，但价格表默认计价模式选择有效价格计算模板时，不显示 `未设置计价方式`。
- 已发布价格表列表展示 `归档选中` 和 `归档列表`，可多选非当前发布版本归档。
- 归档版本从默认列表移出，在归档列表中可点击 `移出归档` 恢复。
- 后端归档/移出归档写 `bean_list_publications.status` 并通过操作日志记录状态变化。

## GREEN Evidence
- Frontend: `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/product-settings.test.js src/lib/product-price-list-types.test.js src/lib/product-price-list-selection.test.js src/lib/bean-list-pdf.test.js` passed 198/198.
- Backend/unit/API: `go test ./...` in `orderapp-remote` passed.
- Build/check: `npm run build` passed with existing Vite chunk-size warning; `scripts/verify_kferp.sh changed` and `git diff --check` passed.
- Follow-up RED browser: deployed `c48536f4` allowed archiving `V3.0.7 #51`; row moved out of default list and success message appeared, but `归档列表` stayed `(0)` because archive refresh did not update the current visible product-type cache key. The test row was restored through the unarchive API before the fix.
- Follow-up GREEN local: `node --test src/lib/costing-bean-list-version-ui.test.js` passed 28/28; full local checks passed again with `go test ./...`, frontend 198/198, `npm run build`, `scripts/verify_kferp.sh changed`, and `git diff --check`.
- Follow-up 2 RED browser: deployed `1bd2ef90` showed `归档列表 (1)` correctly after archive, but clicking `移出归档` showed success while the default list did not immediately restore `V3.0.7 #51`; API confirmed the backend status was already restored to `published`.
- Follow-up 2 GREEN local: `node --test src/lib/costing-bean-list-version-ui.test.js` passed 28/28 after cache status sync; targeted frontend with product/work-order tests passed 160/160; support/API contracts passed; `npm run build`, `scripts/verify_kferp.sh changed`, and `git diff --check` passed.
- Deployment/browser acceptance: pending redeploy after follow-up 2 fix.
