import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const listSource = readFileSync(resolve('src/pages/employee-orders/employee-orders.vue'), 'utf8')
const detailSource = readFileSync(resolve('src/pages/employee-order-detail/employee-order-detail.vue'), 'utf8')
const entrySource = readFileSync(resolve('src/pages/employee-order-entry/employee-order-entry.vue'), 'utf8')
const pagesSource = readFileSync(resolve('src/pages.json'), 'utf8')

describe('employee order detail miniapp page contract', () => {
  it('opens valid summary cards and keeps malformed rows visible without an empty navigator url', () => {
    expect(listSource).toMatch(
      /<navigator\b(?=[^>]*v-if="row\.detail_url")(?=[^>]*:url="row\.detail_url")[^>]*>/,
    )
    expect(listSource).toContain('<template v-for="row in rows"')
    expect(listSource).toContain('<view v-else class="card card-disabled">')
    expect(listSource).toContain('订单编号异常，无法查看')
    expect(listSource).toContain('employeeOrderNavigationRows')
    expect(listSource).toContain('rows.value = employeeOrderNavigationRows(response.rows)')
    expect(listSource).not.toContain(':url="employeeOrderDetailPagePath(row.id)"')
    expect(listSource).not.toContain('@tap="openOrderDetail(row)"')
    expect(listSource).toContain('rememberEmployeeOrderListQuery(q.value)')
    expect(listSource).toContain('employeeOrderListQuery()')
  })

  it('registers the detail page and renders the web-order information groups', () => {
    expect(pagesSource).toContain('pages/employee-order-detail/employee-order-detail')
    for (const label of ['订单日期', '单据日期', '收件信息', '物流信息', '订单状态', '费用明细', '商品明细', '报价来源', '生产来源', '订单信息']) {
      expect(detailSource).toContain(label)
    }
    for (const field of ['收款', '发货', '生产', '开票', '寄件人', '负责人', '录入人', '备注', '价格表版本', '小计']) {
      expect(detailSource).toContain(field)
    }
    expect(detailSource).toContain('order.sender_label')
  })

  it('offers real PDF and image sharing for sales orders and delivery notes', () => {
    expect(detailSource).toContain('fetchEmployeeOrderDetail')
    expect(detailSource).toContain('generateEmployeeOrderDocument')
    expect(detailSource).toContain('shareMiniappFileOutput')
    expect(detailSource).toContain('fetchEmployeeShareSettings')
    expect(detailSource).toContain('needShowEntrance: imageNeedShowEntrance')
    expect(detailSource).toContain("format === 'png'")
    expect(detailSource).toContain('await showShareSettingsFallbackNotice()')
    expect(detailSource).toContain('本次将按安全方式继续，图片不会携带小程序入口。')
    expect(detailSource).toContain('asset?.filename')
    expect(detailSource).toContain('asset?.version_no')
    for (const label of ['销售单 PDF', '销售单图片', '发货单 PDF', '发货单图片']) {
      expect(detailSource).toContain(label)
    }
  })

  it('places export and WeChat sharing before the order summary and detail sections', () => {
    const documentSection = detailSource.indexOf('<view class="section document-section">')
    const heroSection = detailSource.indexOf('<view class="hero-card">')
    expect(documentSection).toBeGreaterThan(-1)
    expect(heroSection).toBeGreaterThan(-1)
    expect(documentSection).toBeLessThan(heroSection)
    expect(detailSource.match(/class="section document-section"/g)).toHaveLength(1)
  })

  it('refreshes on show and exposes a valid edit action only when the backend allows it', () => {
    expect(detailSource).toContain("import { onLoad, onShow } from '@dcloudio/uni-app'")
    expect(detailSource).toContain('onShow(() => void loadDetail())')
    expect(detailSource).toContain('response.can_edit ?? response.order.can_edit')
    expect(detailSource).toContain('v-if="canEdit"')
    expect(detailSource).toContain('@tap="openEditor"')
    expect(detailSource).toContain('editBlockReason')
    expect(detailSource).toContain('`/pages/employee-order-entry/employee-order-entry?edit_id=${orderID.value}`')
    expect(detailSource).not.toMatch(/<navigator[^>]*edit_id/)
  })

  it('renders exactly one add-product action after the final item row', () => {
    const itemLoop = entrySource.indexOf('v-for="(item, index) in form.items"')
    const addButton = entrySource.indexOf('@tap="addItem"')
    expect(itemLoop).toBeGreaterThan(-1)
    expect(addButton).toBeGreaterThan(itemLoop)
    expect(entrySource.match(/@tap="addItem"/g)).toHaveLength(1)
    const sectionHead = entrySource.slice(entrySource.indexOf('<view class="section-head">'), itemLoop)
    expect(sectionHead).not.toContain('@tap="addItem"')
  })
})
