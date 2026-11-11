export function shippingReturnUploadMessage(data) {
  if (data.rematched) {
    return `已重新匹配；待选择 ${Number(data.pending || 0)} 条；补录寄件重量 ${Number(data.weight_backfilled || 0)} 条；重量冲突或异常跳过 ${Number(data.weight_skipped || 0)} 条。本次未执行发货。`
  }
  return `已回填 ${Number(data.updated || 0)} 条；待选择 ${Number(data.pending || 0)} 条；跳过 ${Number(data.skipped || 0)} 条`
}
