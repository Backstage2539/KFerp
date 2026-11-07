package orderresolution

// Deployment capabilities are separate from administrator policy. A policy
// cannot expose a disposition until its execution and reconciliation exist.
func unavailableCapabilities() map[string]string {
	return map[string]string{
		"action:return":                   "退货流程尚未开放，请先联系仓库核对实物、批次和货权",
		"production:stop":                 "在制余料与半成品处置尚未开放，请由生产负责人核对",
		"production:reassign":             "指定需求转供尚未开放，请先核对目标需求",
		"production:allocate":             "库存重新分配尚未开放，请在库存业务核对目标需求",
		"production:scrap":                "请从经批准的库存报损流程处理，订单页面暂不执行报损",
		"direct_ship:replace_supply":      "更换代发货源尚未开放，请先核对明确的替代货源",
		"customer_stock:ownership_return": "客户货权退回尚未开放，请先联系仓库核对批次和数量",
		"customer_stock:physical_return":  "实物退货尚未开放，请先联系仓库核对退货实物",
		"stock:allocate":                  "库存重新分配尚未开放，请在库存业务核对目标需求",
		"stock:scrap":                     "请从经批准的库存报损流程处理，订单页面暂不执行报损",
	}
}
