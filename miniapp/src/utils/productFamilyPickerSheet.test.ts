import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

function source(path: string): string {
  return readFileSync(resolve(path), 'utf8')
}

describe('shared product-family picker sheet', () => {
  it('owns the employee-order search, categories and one-row-per-family list', () => {
    const picker = source('src/components/ProductFamilyPickerSheet.vue')

    expect(picker).toContain('filterEmployeeOrderProductFamilies')
    expect(picker).toContain('employeeOrderProductCategories')
    expect(picker).toContain('商品 / 别名 / 拼音 / 编码 / 规格')
    expect(picker).toContain('v-for="family in filteredFamilies"')
    expect(picker).toContain("select: [family: EmployeeOrderProductFamily]")
    expect(picker).toContain('z-index:1100')
  })

  it('is reused by employee order entry and direct shipment without changing the production selector', () => {
    const employeeOrder = source('src/pages/employee-order-entry/employee-order-entry.vue')
    const directShip = source('src/components/CustomerDirectShipPanel.vue')
    const processing = source('src/components/CustomerProcessingPanel.vue')

    expect(employeeOrder).toContain('ProductFamilyPickerSheet')
    expect(directShip).toContain('ProductFamilyPickerSheet')
    expect(employeeOrder).not.toContain('customer-facing-names')
    expect(directShip).toContain('customer-facing-names')
    expect(directShip).not.toContain('CustomerProductSelector')
    expect(processing).toContain('CustomerProductSelector')
  })

  it('keeps product, spec and quantity controls visible for every direct-shipment row', () => {
    const directShip = source('src/components/CustomerDirectShipPanel.vue')

    expect(directShip).toContain('v-for="(line, index) in lines"')
    expect(directShip).toContain('<text class="line-label">商品</text>')
    expect(directShip).toContain('<text class="line-label">规格</text>')
    expect(directShip).toContain('<text class="line-label">数量</text>')
    expect(directShip).toContain('@tap="openProductSelector(line.key)"')
    expect(directShip).toContain('@change="chooseSpec(line, $event)"')
    expect(directShip).toContain('lines.value = [createDirectShipDraftLine()]')
    expect(directShip).toContain('buildDirectShipDraftItems(lines.value)')
    expect(directShip).toContain('directShipDraftValidation(lines.value)')
    expect(directShip).toContain('let previewVersion = 0')
    expect(directShip).toContain('if (version !== previewVersion) return')
    expect(directShip).toContain('const command = payload()')
    expect(directShip).toContain('createDirectShipRequest(props.token, command)')
  })
})
