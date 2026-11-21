import { uploadEmployeeOrderPaymentVoucher, type EmployeeOrderAsset } from '../api/customerPortal'

export function employeePaymentNeedsVoucher(status: string) {
  return /已付款|已收款|已支付/.test(status)
}

export async function chooseEmployeePaymentVoucher(token: string, orderID: number): Promise<EmployeeOrderAsset | undefined> {
  const path = await new Promise<string>((resolve, reject) => {
    uni.chooseImage({
      count: 1,
      sizeType: ['compressed'],
      sourceType: ['album', 'camera'],
      success: result => resolve(result.tempFilePaths[0] || ''),
      fail: error => {
        if (/cancel/i.test(error.errMsg || '')) resolve('')
        else reject(new Error('未能打开照片，请检查微信相册权限后重试'))
      },
    })
  })
  if (!path) return
  const { asset } = await uploadEmployeeOrderPaymentVoucher(token, orderID, path)
  if (!asset?.id) throw new Error('凭证上传未完成，请重试')
  return asset
}
