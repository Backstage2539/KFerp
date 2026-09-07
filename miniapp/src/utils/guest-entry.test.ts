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
 it('calculates 30/50/70 percent of goods and rounds to cents',()=>{
 expect(prepaymentPresets).toEqual([30,50,70]);expect(prepaymentByRate(616,30)).toBe(184.8);expect(prepaymentByRate(99.99,50)).toBe(50)
 })
 it('cold launch renders guest services without login or private API request',()=>{
 const index=readFileSync(new URL('../pages/index/index.vue',import.meta.url),'utf8')
 expect(index).toContain('GuestHome')
 expect(index).not.toContain("url: '/pages/login/login'")
 expect(index).not.toContain('getPhoneNumber')
 expect(index).not.toContain('fetchMe')
 })
 it('login page provides a way back to public browsing',()=>{
 const login=readFileSync(new URL('../pages/login/login.vue',import.meta.url),'utf8');expect(login).toContain('先浏览服务');expect(login).toContain('/pages/index/index')
 })
})
