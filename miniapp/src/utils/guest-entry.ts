export const guestServiceGuides = [
  { id: 'order', title: '豆单与订购', summary: '了解选品、规格和订购流程', steps: ['先确定需要的咖啡品类、包装规格与数量。', '绑定客户账号后，查看对应的已发布价格表与可售规格。', '核对收件信息和运费，提交订单后查看处理进度。'] },
  { id: 'processing', title: '咖啡代加工', summary: '从原料准备到烘焙与包装', steps: ['准备原料、目标产品规格和包材要求。', '与工作人员确认配方、加工数量及交付安排。', '按确认的生产流程完成烘焙、包装和成品入库。'] },
  { id: 'fulfillment', title: '仓储与代发', summary: '了解库存查询和发货服务', steps: ['先由工作人员确认客户仓库及库存归属。', '登录绑定账号后，可查看自己的仓库库存和订单。', '提交发货需求并核对收件信息，发货后查看物流进度。'] },
]
export function packagingEstimate(kilograms: number, gramsPerPack: number) {
  if (!Number.isFinite(kilograms) || !Number.isFinite(gramsPerPack) || kilograms <= 0 || gramsPerPack <= 0) return { packs: 0, remainderGrams: 0 }
  const total = Math.round(kilograms * 1000)
  const packs = Math.floor(total / gramsPerPack)
  return { packs, remainderGrams: Math.round((total - packs * gramsPerPack) * 100) / 100 }
}
