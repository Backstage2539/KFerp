package orderresolution

// Deployment capabilities are separate from administrator policy. A policy
// cannot expose a disposition until its execution and reconciliation exist.
func unavailableCapabilities() map[string]string {
	return map[string]string{
		"production:stop":                "在制余料与半成品处置尚未开放，请由生产负责人核对",
		"production:reassign":            "指定需求转供尚未开放，请先核对目标需求",
		"production:scrap":               "请从经批准的库存报损流程处理，订单页面暂不执行报损",
		"customer_stock:physical_return": "已发出的客户货权实物，请进入对应代发订单办理原批次退货；本单只办理留厂货权退回",
		"stock:allocate":                 "库存重新分配尚未开放，请在库存业务核对目标需求",
		"stock:scrap":                    "请从经批准的库存报损流程处理，订单页面暂不执行报损",
	}
}
