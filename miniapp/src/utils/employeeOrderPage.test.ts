import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const pageSource = readFileSync(resolve('src/pages/employee-order-entry/employee-order-entry.vue'), 'utf8')
const productPickerSource = readFileSync(resolve('src/components/ProductFamilyPickerSheet.vue'), 'utf8')
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
    expect(pageSource).toContain(':disabled="productCatalogLoading || !familyForItem(item)"')
    expect(pageSource).toContain('employeeOrderItemFromSpec(target, family, spec)')
    expect(pageSource).not.toContain('v-model="item.spec_g"')
  })

  it('initializes the date before loading and exposes searchable customer and product layers', () => {
    expect(pageSource).toContain('order_date: shanghaiToday()')
    expect(pageSource).toContain('搜索客户名称 / 拼音 / 首字母')
    expect(pageSource).toContain('ProductFamilyPickerSheet')
    expect(productPickerSource).toContain('商品 / 别名 / 拼音 / 编码 / 规格')
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

  it('invalidates stale customer-editor intents while customer context is loading', () => {
    expect(pageSource).toContain('let customerEditorIntentSequence = 0')
    expect(pageSource).toContain('const intentSequence = ++customerEditorIntentSequence')
    expect(pageSource).toContain('intentSequence !== customerEditorIntentSequence')
    expect(pageSource).toContain('customerContextLoadPromise')
    expect(pageSource).toContain('customerEditorIntentSequence += 1')
  })

  it('keeps the editable order form hidden until both form data and the server draft finish loading', () => {
    expect(pageSource).toContain('<template v-else-if="formData">')
    const guardedFormSource = pageSource.slice(
      pageSource.indexOf('<template v-else-if="formData">'),
      pageSource.indexOf('</template>', pageSource.indexOf('<template v-else-if="formData">')),
    )
    expect(guardedFormSource).toContain('<text class="label">订单日期</text>')
    expect(guardedFormSource).toContain('v-model="form.notes"')
    expect(guardedFormSource).toContain('@tap="submit"')
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

  it('reuses the shared recipient parser in the customer editor without local parsing rules', () => {
    expect(customerEditorSource).toContain('parseEmployeeCustomerRecipient')
    expect(customerEditorSource).toContain('mergeEmployeeCustomerRecipientFields')
    expect(customerEditorSource).toContain('snapshotEmployeeCustomerRecipientFields')
    expect(customerEditorSource).toContain('粘贴收货信息')
    expect(customerEditorSource).toContain('地址解析')
    expect(customerEditorSource).toContain('recipientParsing')
    expect(customerEditorSource).toContain('recipientParseSequence')
    expect(customerEditorSource).toContain('sequence !== recipientParseSequence')
    expect(customerEditorSource).toContain('Number(props.customerId || 0) !== targetCustomerID')
    expect(customerEditorSource).toContain('recipientPaste.value.trim() !== text')
    expect(customerEditorSource).toContain('if (!text || recipientParsing.value || saving.value) return')
    expect(customerEditorSource).toContain(':disabled="recipientParsing || saving || !recipientPaste.trim()"')
    expect(customerEditorSource).toContain('if (recipientParsing.value) return')
    expect(customerEditorSource).toContain(':disabled="saving || recipientParsing"')
    const parserSource = customerEditorSource.slice(
      customerEditorSource.indexOf('async function parseRecipient()'),
      customerEditorSource.indexOf('async function save()'),
    )
    expect(parserSource).toContain('const recipientFieldsAtRequest = snapshotEmployeeCustomerRecipientFields(form)')
    expect(parserSource.indexOf('const recipientFieldsAtRequest = snapshotEmployeeCustomerRecipientFields(form)')).toBeLessThan(
      parserSource.indexOf('await parseEmployeeCustomerRecipient'),
    )
    expect(parserSource.indexOf('sequence !== recipientParseSequence')).toBeLessThan(
      parserSource.indexOf('Object.assign(form, mergeEmployeeCustomerRecipientFields(form, parsed, recipientFieldsAtRequest))'),
    )
    expect(parserSource).not.toContain('form.name =')
    const parserCatchSource = parserSource.slice(
      parserSource.indexOf('} catch (cause) {'),
      parserSource.indexOf('} finally {'),
    )
    expect(parserCatchSource).not.toContain('Object.assign')
    expect(customerEditorSource).not.toMatch(/1\[3-9\].*\\d\{9\}/)
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

  it('reloads the customer-scoped published catalog and disables product selection while it is loading', () => {
    expect(pageSource).toContain('fetchEmployeeOrderForm(session.token, targetCustomerID)')
    expect(pageSource).toContain('productCatalogLoading')
    expect(pageSource).toContain('async function chooseCustomer')
    expect(pageSource).toContain('await loadCustomerProductCatalog')
    expect(pageSource).toContain(':class="{ muted: !form.customer_id || productCatalogLoading || loading || !formData }"')
    expect(pageSource).toContain('if (productCatalogLoading.value')
    expect(pageSource).toContain('form.value.items = [createEmployeeOrderItem()]')
    expect(pageSource).toContain('saving || savingDraft || clearingDraft || loading || productCatalogLoading')
    const restoreDraftSource = pageSource.slice(
      pageSource.indexOf('async function restoreDraft'),
      pageSource.indexOf('async function loadDraft'),
    )
    expect(restoreDraftSource).toContain('await loadCustomerProductCatalog')
    expect(restoreDraftSource).toContain('preserveManualPrice: true')
    expect(restoreDraftSource).not.toContain('preserveUnitPrice: true')
  })

  it('reuses the entry page for pre-production editing without touching the create-order draft', () => {
    expect(pageSource).toContain("editOrderID.value = Number(options?.edit_id || 0)")
    expect(pageSource).toContain("uni.setNavigationBarTitle({ title: '编辑销售订单' })")
    expect(pageSource).toContain('fetchEmployeeOrderDetail(session.token, editOrderID.value)')
    expect(pageSource).toContain('hydrateEmployeeOrderEditItems')
    expect(pageSource).toContain('employeeOrderEditableOrderDiscount')
    expect(pageSource).toContain('updateEmployeeOrder(session.token, editOrderID.value')
    expect(pageSource).toContain('v-if="!isEditMode"')
    expect(pageSource).toContain('{{ isEditMode ? \'保存修改\' : \'提交订单\' }}')
    expect(pageSource).toContain('if (!isEditMode.value) await loadDraft()')
    expect(pageSource).toContain('if (isEditMode.value) return')
    expect(pageSource).toContain('<text class="label">运费（元）</text>')
    expect(pageSource).toContain('<text class="label">优惠（元）</text>')
    expect(pageSource).toContain('orderGrandTotal')
    expect(pageSource).toContain('shipping_amount: Number(form.value.shipping_amount || 0)')
    expect(pageSource).toContain('discount_amount: Number(form.value.discount_amount || 0)')
    expect(pageSource).toContain('detail.order_discount_amount')
    expect(pageSource).toContain('employeeOrderOutsourceTotal(detail)')
    expect(pageSource).toContain('preservedRoundToInt.value = Boolean(detail.round_to_int)')
    expect(pageSource).toContain('preservedOutsourceTotal.value')
    expect(pageSource).toContain('preservedRoundToInt.value')
    expect(pageSource).toContain('原订单代加工费用（保留）')
    expect(pageSource).toContain('原订单应收按原设置向下取整')
    expect(pageSource).toContain("const editRetailOrder = /零售|retail/i.test")
    expect(pageSource).toContain('targetCustomerID, editRetailOrder)')
    expect(pageSource).toContain('@input="quantityChanged(item)"')
    expect(pageSource).toContain('repriceEmployeeOrderItemForQuantity')
    expect(pageSource).toContain('isEmployeeOrderNonNegativeMoney')
    expect(pageSource).toContain('cause instanceof MiniRequestError && cause.statusCode === 409')
    expect(pageSource).toContain("title: '订单已不能编辑'")
    expect(pageSource).toContain('await refreshCurrentProductCatalog()')
    expect(pageSource).toContain('价格目录已更新，请检查商品和价格后重试')
    const catalogInvalidationSource = pageSource.slice(
      pageSource.indexOf('function isPriceCatalogInvalidationError'),
      pageSource.indexOf('async function submit()'),
    )
    expect(catalogInvalidationSource).toContain('价格表已更新')

    const editPayloadSource = pageSource.slice(
      pageSource.indexOf('const payload = {'),
      pageSource.indexOf('if (isEditMode.value)'),
    )
    expect(editPayloadSource).not.toContain('outsource_')
    expect(editPayloadSource).not.toContain('round_to_int')
  })

  it('keeps the server edit revision in edit state and sends it only with PUT updates', () => {
    expect(pageSource).toContain("edit_revision: String(detail.edit_revision || '')")

    const submitSource = pageSource.slice(
      pageSource.indexOf('async function submit()'),
      pageSource.indexOf('onLoad((options)'),
    )
    const basePayloadSource = submitSource.slice(
      submitSource.indexOf('const payload = {'),
      submitSource.indexOf('if (isEditMode.value)'),
    )
    const editSubmitSource = submitSource.slice(
      submitSource.indexOf('if (isEditMode.value)'),
      submitSource.indexOf('const result = await createEmployeeOrder'),
    )
    const createSubmitSource = submitSource.slice(
      submitSource.indexOf('const result = await createEmployeeOrder'),
    )

    expect(basePayloadSource).not.toContain('edit_revision')
    expect(editSubmitSource).toContain("edit_revision: String(form.value.edit_revision || '')")
    expect(createSubmitSource).not.toContain('edit_revision')
  })
})
