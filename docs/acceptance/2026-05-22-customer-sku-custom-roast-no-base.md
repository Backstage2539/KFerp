# 2026-05-22 客户专属 SKU 定制烘焙度无基础产品

## 需求

- SKU设置创建客户专属 SKU 时，定制烘焙度不再选择基础产品。
- 定制烘焙度不复制基础产品 BOM 或价格梯度。
- 创建表单删除“定制拼配 BOM”定制类型选项。

## 验收证据

- 前端单元测试：`node --test src/lib/product-settings.test.js` 覆盖 `custom_roast` payload 清空 `base_product_id` 和复制开关。
- API 测试：`go test ./internal/interfaces/http/catalog -run TestProductSettingsAPICreatesCustomerCustomRoastWithoutBaseProduct -count=1` 覆盖 `base_product_id=0` 创建定制烘焙度 SKU。
- 仓储回归测试：`go test ./internal/infrastructure/postgres/catalog -run TestCreateCustomProductAllowsCustomRoastWithoutBaseProduct -count=1` 覆盖后端仓储层不再对 `custom_roast` 强制加载基础产品。
- 支持模块测试：`go test ./internal/interfaces/http/support -run TestDev313CustomerSkuCustomRoastNoBaseProduct -count=1` 覆盖 Vue 创建表单、需求和手册接线。
