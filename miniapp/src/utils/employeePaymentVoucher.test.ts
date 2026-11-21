import { afterEach, describe, expect, it, vi } from 'vitest'
import { chooseEmployeePaymentVoucher } from './employeePaymentVoucher'
import { uploadEmployeeOrderPaymentVoucher } from '../api/customerPortal'

vi.mock('../api/customerPortal', () => ({ uploadEmployeeOrderPaymentVoucher: vi.fn() }))
const upload = vi.mocked(uploadEmployeeOrderPaymentVoucher)
afterEach(() => { vi.unstubAllGlobals(); vi.clearAllMocks() })

describe('employee payment voucher photo selection', () => {
  it('opens the photo album and only uploads the selected file', async () => {
    const chooseImage = vi.fn(options => options.success({ tempFilePaths: ['/tmp/receipt.png'] }))
    vi.stubGlobal('uni', { chooseImage })
    upload.mockResolvedValue({ asset: { id: 21, filename: 'receipt.png', kind: 'payment_voucher', content_type: 'image/png', bytes: 128, created_at: '', created_by: 'employee' } })
    await expect(chooseEmployeePaymentVoucher('session', 42)).resolves.toMatchObject({ id: 21 })
    expect(chooseImage).toHaveBeenCalledWith(expect.objectContaining({ sourceType: ['album', 'camera'], count: 1 }))
    expect(upload).toHaveBeenCalledWith('session', 42, '/tmp/receipt.png')
  })

  it('leaves the draft untouched when the employee cancels', async () => {
    vi.stubGlobal('uni', { chooseImage: (options: UniApp.ChooseImageOptions) => options.fail?.({ errMsg: 'chooseImage:fail cancel' }) })
    await expect(chooseEmployeePaymentVoucher('session', 0)).resolves.toBeUndefined()
    expect(upload).not.toHaveBeenCalled()
  })
})
