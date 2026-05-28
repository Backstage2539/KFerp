# PR-384 SKU商品分类管理归属与删除修复验收

## 范围

- SKU设置 → 商品配置 → 商品分类管理只维护当前归属自己拥有的产品类型/产品子类型。
- 去掉“复制为客户分类”，客户复用公共或其他客户 SKU 与分类结构统一走“SKU复制”。
- 产品类型收起后更明显，产品子类型缩进显示。
- 修复分类删除按钮被拖拽事件抢占导致点击失效的问题。

## 验收步骤

1. 进入 SKU设置，选择“凯哥”等履约客户。
2. 打开“商品配置”页签，再进入“商品分类管理”。
3. 确认分类管理中不显示公共分类引用；如果该客户自己的分类都已失效，列表应为空。
4. 确认页面里没有“复制为客户分类”按钮；需要复制公共或其他客户 SKU 时，回到 SKU 列表右上角使用“SKU复制”。
5. 折叠任意产品类型，确认收起后的产品类型以单独一行边框展示。
6. 展开产品类型，确认产品子类型相对产品类型缩进显示。
7. 点击产品类型或产品子类型的删除模式，再点击红色删除按钮，确认能直接删除，分类下 SKU 回到停车场。

## 预期结果

- 客户分类管理不再把公共分类引用混进可维护列表。
- “复制为客户分类”入口消失，复制分类结构由“SKU复制”统一完成。
- 产品类型折叠态和产品子类型层级更清楚。
- 删除按钮不会触发拖拽，点击即可调用删除逻辑。

## 验证证据

- Unit/UI guard: `node --test src/lib/product-settings.test.js --test-name-pattern 'SKU settings opens SKU creation|SKU category management only edits|SKU category management makes collapsed|SKU category management delete buttons'`
- Full frontend tests: `node --test src/lib/*.test.js src/api/*.test.js`，407 pass
- Frontend build: `npm run build`
- API/support guard: `go test ./internal/interfaces/http/support -run 'TestDev(370|382)|TestProductSettingsVueSupportsCategoryDelete|TestDevOperationManuals' -count=1`
- Browser smoke: mock API 打开 `/vue-shell/?view=productSettings&workspace=customer&customer_id=122`；客户开启公共商品分类时，商品分类管理只显示客户自有 `客户测试类型 / 客户测试子类型`，不显示 `公共产品类型`，不显示“复制为客户分类”；点击删除子类型和产品类型后 mock API 记录 `deleteCalls=[102,101]`，分类管理显示“没有匹配的商品分类”。
- Manual: `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
