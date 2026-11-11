const legacyChoices: [string, string][] = [['created','下单成功'],['paid','付款成功'],['production_started','开始生产'],['production_finished','生产完成'],['shipped','发货进度'],['cancelled','订单取消']]
export const defaultOrderNotificationEvents = legacyChoices.map(([key]) => key)
export const orderNotificationChoices: [string, string][] = [...legacyChoices, ['order_accepted','订单接单'], ['shipment_created','订单发货'], ['receipt_received','货款到账'], ['bill_issued','对账单生成']]
