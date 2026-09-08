# PR-637 EFS 客户报价继承回归

## 复现与原因

- 开发运行代码 60e0c43a；客户 302（EFS咖啡），引用 280，源商品 1063（初晓-商品）。引用及规格读取正常。
- `/api/costing/bean-list?customer_id=302`、客户/公共供货表 publications 接口均 200；客户尚无已发布表。
- 公共最新表 122 / V3.0.22 / 2026-09-07 17:42:03 包含 227g（BOM spec 3、variant 483）的四档价格 28、33、26、24。页面选择同一规格后仍显示 0，真实接口数据重放可匹配全部四档。
- 多张命名表初始化会改变单张草稿 key。报价加载把此 key 用作请求范围，结果在初始化过程中被丢弃；切换命名表也未重新填充。现在报价请求以客户/商品类型为范围，草稿仍按命名表独立保存，并在命名表切换时补齐报价。
- 用户最初选中 454g（spec 7、variant 484）；此规格没有在最新公共表 122 报价，旧表 110 / V3.0.21 报价不能自动套用。保留待报价并明确指出要补齐公共最新版本或填写客户价格。

## 测试

- `customer-price-source-loading.test.js` 执行实际页面加载与 Vue watcher：初始化期间响应、切换同选品命名表均 RED，错误为实际 `[0]`、期望 `[28,33,26,24]`；跨客户迟到响应隔离原已通过。
- 修复后同一组 GREEN，另覆盖客户既有报价保留、草稿持久化、不同客户隔离、缺规格不取旧公共版本。
- RED `/tmp/efs-price-source-red.log`；GREEN `/tmp/efs-price-source-green.log`，完整 Vue 1108/1108 和 Vite 构建 `/tmp/efs-price-frontend.log`；统一 changed 检查通过。部署额外执行全量 Go、小程序 238 项测试、类型检查及构建。
- 手册：`orderapp-remote/docs/OP_MANUAL_COSTING.md`，现有 Vue 成本核价手册入口。未改变商品、BOM、库存、历史报价或录单规则。

## 发布与复验

- 功能分支 `codex/efs-customer-price-inheritance-20260908`，推送代码 `d3e1ef7f`，合入 `origin/develop@277c56d18aa68a035d9f37dc1cddbabd4fcd4704`。干净 release 工作区运行 `./deploy_orderapp.sh development`，退出 0；日志 `/tmp/efs-price-deploy.log`。
- 回滚源码 `/opt/stacks/erp/orderapp.backup.deploy-20260908110430-277c56d18aa6`，回滚镜像 `kferp-orderapp-rollback:development-20260908110430-277c56d18aa6`。
- 应用 running / 重启 0，数据库 healthy，登录页 200，未认证价格接口 401，认证报价 API 均 200；CostingView、测试和手册服务端 SHA256 与本次代码一致。
- 页面选择 EFS / 初晓-商品 / 227g 后，平铺行与预览为 2–13 件 33 元、14–23 件 28 元、24–47 件 26 元、48 件起 24 元。刷新后仍显示四档；保留 `after-227g.png` 与 `after-refresh-dom.txt`。
- 页面恢复原来的 454g 选品；仍显示缺最新规格报价的新提示，未套用 227g 或旧版报价。客户引用 280 有效；EFS 已发布价格表仍为空；本次未向任何发布/报价写入接口发送请求。测试只改变并恢复本机选品草稿。
- 开发小程序包 56 个清单文件核验并同步完成，未上传微信；生产运行代码仍 `ce58d4ec90df6fe2ea079f650362ccdacf7046ec`，未部署生产。
- 本次只修复报价加载和提示，不改 BOM、规格身份、库存或历史价格。未针对本次 Web 修复执行小程序真机验收。

证据目录：`/Users/yiiiple-work/.codex/worktrees/ccd2/KFerp/outputs/efs-price-inheritance-20260908/`。最终验收文档补交不改变运行代码 `277c56d1`。
