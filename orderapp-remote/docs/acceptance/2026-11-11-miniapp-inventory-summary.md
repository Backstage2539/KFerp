# 客户代发库存摘要精简

- 客户小程序和管理员只读预览在代发页每种规格仅显示现货可用、待入库已预订；隐藏待入库可安排及累计购买／入库，不改变下单和补购计算。
- RED：原页面显示多余数量，4 条新增／调整断言失败，另补缺货时第三项显示的 RED；见 `/tmp/kferp-inventory-summary-red.log`、`/tmp/kferp-inventory-shortage-red.log`。
- GREEN：正常客户与只读预览、零值／非零、规格单位及补购计算含缺货时摘要仍只有两项且商品行保留补购提示，定向 36 项通过；小程序全量 527 项、类型检查及 support Go 测试通过。
- 生产预检通过：服务器 Vue 1917 项、小程序 527 项、Go 全量与隔离镜像构建。
- 已合并 develop `1810efe62961cd1261e837dc1eab5effdc4a5270` 与 main；正式部署 `ac608e456555e417932d737c9f0be30b10ded77d`，应用运行 0 重启、数据库 healthy、外网登录 200；新版小程序包 108 文件，RELEASE_INFO 为生产地址与上述版本。
- 数据库备份 `/opt/stacks/erp-production/backups/pre-inventory-summary-20261111180522.dump`；源码回滚 `/opt/stacks/erp-production/orderapp.backup.deploy-20261111180555-ac608e456555`。
- DevTools 重开并编译生产包，喜沁客户代发页显示曜石 11 袋现货、红岩 9 袋现货，各待入库已预订 0 袋；摘要没有累计、待入库可安排或补购数字。未提交真实订单。
- 按本次指定只部署生产，开发服务未部署本次变更。Van 验收、微信上传／审核／发布待完成。
