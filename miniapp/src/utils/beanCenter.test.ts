import { describe, it, expect } from 'vitest'
import { beanCards, customerMainTabs } from './beanCenter'
import { safeLoginReturn } from './loginReturn'
describe('bean centre visitor flow',()=>{
 it('filters directory without changing its order',()=>{
 const cards=[{key:'b',name:'挂耳',version:'v2'},{key:'a',name:'咖啡豆',version:'v1'}]
 expect(beanCards(cards,' 咖啡 ')).toEqual([cards[1]])
 expect(beanCards(cards,'').map(c=>c.key)).toEqual(['b','a'])
 })
 it('visitors only have home beans and mine; customers retain business navigation',()=>{
 expect(customerMainTabs(false).map(t=>t.key)).toEqual(['home','beans','mine'])
 expect(customerMainTabs(true).map(t=>t.key)).toEqual(['home','beans','orders','billing','mine'])
 expect(customerMainTabs(true).find(t=>t.key==='orders')?.url).toContain('source=official')
 })
 it('allows catalogue return and rejects injected destinations',()=>{
 expect(safeLoginReturn('/pages/bean-list-center/bean-list-center')).toBe('/pages/bean-list-center/bean-list-center')
 expect(safeLoginReturn('/pages/bean-list-center/bean-list-center?url=https://evil.example')).toBe('')
 })
})
