import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const pageSource = readFileSync(resolve('src/pages/employee-order-entry/employee-order-entry.vue'), 'utf8')
const customerEditorSource = readFileSync(resolve('src/components/EmployeeCustomerEditor.vue'), 'utf8')
const customerPageSource = readFileSync(resolve('src/pages/employee-customers/employee-customers.vue'), 'utf8')

describe('employee mini order entry page contract', () => {
  it('renders explicit product, spec, quantity and unit-price labels', () => {
    expect(pageSource).toContain('<text class="label">商品</text>')
    expect(pageSource).toContain('<text class="label">规格</text>')
    expect(pageSource).toContain('<text class="label">数量（{{ displayedSalesUnit(item) }}）</text>')
    expect(pageSource).toContain('<text class="label">销售单价（元/{{ displayedSalesUnit(item) }}）*</text>')
  })

  it('keeps spec weight derived from the selected spec instead of exposing an editable field', () => {
    expect(pageSource).toContain(':disabled="!familyForItem(item)"')
    expect(pageSource).toContain('employeeOrderItemFromSpec(target, family, spec)')
    expect(pageSource).not.toContain('v-model="item.spec_g"')
  })

  it('initializes the date before loading and exposes searchable customer and product layers', () => {
    expect(pageSource).toContain('order_date: shanghaiToday()')
    expect(pageSource).toContain('搜索客户名称 / 拼音 / 首字母')
    expect(pageSource).toContain('商品 / 别名 / 拼音 / 编码 / 规格')
    expect(pageSource).toContain('if (!form.value.customer_id)')
  })

  it('overwrites the shipping snapshot and provides retry and re-login actions', () => {
    expect(pageSource).toContain('applyCustomerShipping(customer)')
    expect(pageSource).toContain('@tap="loadForm">重试</button>')
    expect(pageSource).toContain('@tap="goToLogin">重新登录</button>')
  })

  it('edits multiple independent item rows and submits every complete item', () => {
    expect(pageSource).toContain('v-for="(item, index) in form.items"')
    expect(pageSource).toContain('新增商品')
    expect(pageSource).toContain('删除本行')
    expect(pageSource).toContain('buildEmployeeOrderItemsPayload(form.value.items)')
    expect(pageSource).toContain('商品估算合计')
    expect(pageSource).toContain('employeeOrderItemsTotal(form.value.items)')
  })

  it('offers customer maintenance and protects manually changed shipping snapshots', () => {
    expect(pageSource).toContain('EmployeeCustomerEditor')
    expect(pageSource).toContain('selectedCustomer?.can_maintain')
    expect(pageSource).toContain('新增客户')
    expect(pageSource).toContain('是否同步客户最新收货资料')
    expect(pageSource).toContain('const targetCustomerID = Number(selectedCustomer.value?.id || 0)')
    expect(pageSource).toContain('Number(selectedCustomer.value?.id || 0) !== targetCustomerID')
    expect(pageSource).toContain('客户已停用，请重新选择启用客户')
  })

  it('drops stale customer detail responses and prevents closing during save', () => {
    expect(customerEditorSource).toContain('customerLoadSequence')
    expect(customerEditorSource).toContain('sequence !== customerLoadSequence')
    expect(customerEditorSource).toContain('Number(props.customerId || 0) !== targetCustomerID')
    expect(customerEditorSource).toContain('if (saving.value) return')
    expect(customerEditorSource).toContain('responsible_employee_id: Number(customer?.responsible_employee_id || 0)')
    expect(customerEditorSource).toContain("selectedEmployeeName() || '请选择负责人'")
    expect(customerEditorSource).toContain('() => [props.visible, props.customerId, props.token] as const')
    expect(customerEditorSource).not.toContain('() => [props.visible, props.customerId, props.context] as const')
  })

  it('searches the full customer set on the server and exposes pagination', () => {
    expect(customerPageSource).toContain("q: query.value")
    expect(customerPageSource).toContain("page: nextPage")
    expect(customerPageSource).toContain('context?.has_next')
    expect(customerPageSource).toContain('加载更多客户')
    expect(customerPageSource).toContain("session.permissions.includes('customers.read')")
    expect(customerPageSource).toContain("session.permissions.includes('customers.write')")
    expect(customerPageSource).toContain(':disabled="!context || loading"')
  })

  it('loads, saves and clears the server order draft', () => {
    expect(pageSource).toContain('fetchEmployeeOrderDraft')
    expect(pageSource).toContain('saveEmployeeOrderDraft')
    expect(pageSource).toContain('deleteEmployeeOrderDraft')
    expect(pageSource).toContain('保存草稿')
    expect(pageSource).toContain('清除草稿')
    expect(pageSource).toContain('clearingDraft')
    expect(pageSource).toContain('preserveUnavailable: true')
    expect(pageSource).toContain('销售单价必须大于 0')
    expect(pageSource).toContain('item?.qty == null ? 1 : Number(item.qty)')
    expect(pageSource).toContain('preserveEmployeeOrderDraftItemsForMissingCustomer(form.value.items)')
    const loadDraftSource = pageSource.slice(
      pageSource.indexOf('async function loadDraft()'),
      pageSource.indexOf('function applyDefaultOptions()'),
    )
    expect(loadDraftSource).toContain('await fetchEmployeeOrderDraft(session.token)')
    expect(loadDraftSource).not.toContain('catch')
  })
})
