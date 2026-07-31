import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const pageSource = readFileSync(resolve('src/pages/employee-order-entry/employee-order-entry.vue'), 'utf8')

describe('employee mini order entry page contract', () => {
  it('renders explicit product, spec, quantity and unit-price labels', () => {
    expect(pageSource).toContain('<text class="label">商品</text>')
    expect(pageSource).toContain('<text class="label">规格</text>')
    expect(pageSource).toContain('<text class="label">数量（{{ displayedSalesUnit }}）</text>')
    expect(pageSource).toContain('<text class="label">销售单价（元/{{ displayedSalesUnit }}）</text>')
  })

  it('keeps spec weight derived from the selected spec instead of exposing an editable field', () => {
    expect(pageSource).toContain(':disabled="!selectedFamily"')
    expect(pageSource).toContain('form.value.spec_g = productSpecWeightG(spec)')
    expect(pageSource).not.toContain('v-model="form.spec_g"')
  })

  it('initializes the date before loading and exposes searchable customer and product layers', () => {
    expect(pageSource).toContain('order_date: shanghaiToday()')
    expect(pageSource).toContain('搜索客户名称 / 拼音 / 首字母')
    expect(pageSource).toContain('商品 / 别名 / 拼音 / 编码 / 规格')
    expect(pageSource).toContain('if (!form.value.customer_id)')
  })

  it('overwrites the shipping snapshot and provides retry and re-login actions', () => {
    expect(pageSource).toContain('Object.assign(form.value, customerShippingDefaults(customer))')
    expect(pageSource).toContain('@tap="loadForm">重试</button>')
    expect(pageSource).toContain('@tap="goToLogin">重新登录</button>')
  })
})
