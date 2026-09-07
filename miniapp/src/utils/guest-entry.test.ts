import {describe,it,expect} from 'vitest'
import {readFileSync} from 'node:fs'
import {packagingEstimate,guestServiceGuides} from './guest-entry'
import {prepaymentPresets,prepaymentByRate} from './prepayment'
describe('guest browsing before login and deposit presets',()=>{
 it('offers useful public service content and packaging calculations',()=>{
 expect(guestServiceGuides.length).toBeGreaterThanOrEqual(3)
 expect(packagingEstimate(10,227)).toEqual({packs:44,remainderGrams:12})
 expect(packagingEstimate(0,0)).toEqual({packs:0,remainderGrams:0})
 })
 it('calculates 30/50/70 percent of receivable total and rounds to cents',()=>{
 expect(prepaymentPresets).toEqual([30,50,70]);expect(prepaymentByRate(616,30)).toBe(184.8);expect(prepaymentByRate(99.99,50)).toBe(50)
 })
 it('cold launch renders guest services without login or private API request',()=>{
 const index=readFileSync(new URL('../pages/index/index.vue',import.meta.url),'utf8')
 expect(index).toContain('GuestHome')
 expect(index).not.toContain("url: '/pages/login/login'")
 expect(index).not.toContain('getPhoneNumber')
 expect(index).not.toContain('fetchMe')
 })
 it('clears the selected deposit rate after an order is saved',()=>{
 const view=readFileSync(new URL('../pages/employee-order-entry/employee-order-entry.vue',import.meta.url),'utf8')
 expect(view.slice(view.indexOf('function resetAfterSubmit()'),view.indexOf('function returnToOrderDetail()'))).toContain('prepaymentRate.value = 0')
 })
 it('login page provides a way back to public browsing',()=>{
 const login=readFileSync(new URL('../pages/login/login.vue',import.meta.url),'utf8');expect(login).toContain('先浏览服务');expect(login).toContain('/pages/index/index')
 })
})

it('miniapp preset uses the final receivable total including freight',()=>{
 const source=readFileSync(new URL('../pages/employee-order-entry/employee-order-entry.vue',import.meta.url),'utf8')
 const body=source.match(/function applyPrepaymentPreset\(rate: number\) \{([\s\S]*?)\n\}/)![1]
 const form={value:{prepayment_amount:0}}
 const total={value:100}
 const apply=new Function('rate','prepaymentRate','form','prepaymentByRate','orderGrandTotal','prepaymentGoodsAmount','selectPrepaymentStatus',body)
 apply(50,{value:0},form,prepaymentByRate,total,{value:80},()=>{})
 expect(form.value.prepayment_amount).toBe(50)
 total.value=120
 apply(50,{value:50},form,prepaymentByRate,total,{value:80},()=>{})
 expect(form.value.prepayment_amount).toBe(60)
 expect(source).toMatch(/watch\(orderGrandTotal,/)
 expect(source.indexOf('watch(orderGrandTotal,')).toBeGreaterThan(source.indexOf('const orderGrandTotal = computed'))
 expect(source).not.toContain('不含运费')
})
