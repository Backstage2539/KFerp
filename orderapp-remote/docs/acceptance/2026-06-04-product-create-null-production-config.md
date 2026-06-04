# PR-413 新建商品档案空生产配置兼容

## 范围
- 修复新建商品档案成功后自动打开“商品档案配置”抽屉时，商品尚无生产配置记录导致 `Cannot read properties of null (reading 'expected_loss_rate')` 的前端报错。
- 空生产配置行时，抽屉以商品档案行为默认值：预期损耗率为 0，生产 BOM、BOM 版本、工艺路线和行业字段模板为空。

## RED
- `node --test src/lib/product-settings.test.js`：新增空生产配置场景测试后失败，原因是 `buildProductProductionConfigForm` 尚未导出，页面内表单构造逻辑未兼容 `null`。

## GREEN
- `node --test src/lib/product-settings.test.js`：109/109 通过。

## 验收点
- 创建商品档案接口成功后，前端仍会定位重载后的新商品并打开商品档案配置抽屉。
- 新商品没有 `product_production_configs` 记录时，不再读取空对象的 `expected_loss_rate`。
- 本轮不改变商品档案创建接口、历史商品配置、价格表、BOM、工单或操作手册流程。

## 证据
- 目标前端测试：`node --test src/lib/product-settings.test.js`，109/109 通过。
- support 需求种子测试：`go test ./internal/interfaces/http/support -run 'TestDev413|TestDev408' -count=1` 通过。
- Vue build：`npm run build` 通过，保留既有 chunk-size warning。
- Changed verifier：`scripts/verify_kferp.sh changed` 退出 0。
- development stack 部署 smoke：`./deploy_orderapp.sh development` 已部署 `origin/develop=6a0acc4979f787f6b95de5524fac69a7ae0b7165`；容器运行；`GET /app/` 返回 303 到 `/app/orders`；`GET /app/vue-shell/` 返回 200；认证需求 API 暴露 `PR-413-PRODUCT-CREATE-NULL-PRODUCTION-CONFIG`；远端源码包含 `buildProductProductionConfigForm`。
