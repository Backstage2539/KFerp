# PR-401 商品价格表缺少阶梯价模板提示

## 背景
- Van 要求：像 `初晓2.5kg装` 这种没有配置阶梯价模板的商品，报价表要提示没有配置阶梯价模板。
- PR-400 已修复无模板商品不再自动生成默认商业阶梯价，本轮补齐用户可见提示，避免报价卡片静默空白。

## 需求
- 熟豆、速溶等需要商业报价阶梯的商品，如果没有有效阶梯价模板且 `commercial_wholesale_tiers` 为空，返回 warning：
  - `未配置阶梯价模板：商品价格表不会生成商业阶梯价。请在商品档案绑定含阶梯价模板的商品配置模板，或设置客户商品规则。`
- 绑定有效阶梯价模板的商品不显示该提示。
- 生豆直接销售和挂耳商品不因为缺少熟豆阶梯价模板显示该提示。
- Vue 商品价格表继续使用统一 warning chip 展示后端 `warnings`。

## RED Evidence
- `go test ./internal/domain/costing -run 'TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers|TestProductWithGradientTemplateDoesNotWarnMissingGradientTemplate' -count=1`
  - 失败原因：无模板商品 `warnings=[]`。
- `go test ./internal/application/costing -run TestBeanListRequiresExplicitGradientTemplateForCommercialTiers -count=1`
  - 失败原因：商品价格表服务返回的无模板商品没有 warning。

## GREEN Evidence
- `go test ./internal/domain/costing -run 'TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers|TestProductWithGradientTemplateDoesNotWarnMissingGradientTemplate' -count=1`
  - 通过。
- `go test ./internal/application/costing -run TestBeanListRequiresExplicitGradientTemplateForCommercialTiers -count=1`
  - 通过。
- `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
  - 通过。
- `go test ./...`
  - 通过。
- `scripts/verify_kferp.sh changed`
  - 通过：命令退出码 0。
- Development deploy：
  - `./deploy_orderapp.sh development` 已部署 `origin/develop=577e821444d08f4f72878ec9a010d490514be44a`。
  - 备份：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260603201536`。
  - Docker build 内部 `go test ./...` 通过。
  - Smoke：容器运行；未登录 `/app/` 返回 303；BasicAuth `/app/vue-shell` 返回 200；需求 API 暴露 `PR-401-PRICE-LIST-MISSING-GRADIENT-WARNING`。
  - Live API：Karen 商品价格表中 `初晓2.5kg装` 返回 `commercial_wholesale_tiers=0`，并包含 `未配置阶梯价模板` warning。

## 手册
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
