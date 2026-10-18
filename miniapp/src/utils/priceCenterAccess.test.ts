import {describe,it,expect} from 'vitest'
import {canUseEmployeePriceCenter,canOpenPageEntry} from './priceCenterAccess'
const warehouse={accountType:'employee',roles:['warehouse']}
describe('price center employee access scope',()=>{
 it.each(['article','function'])('preserves existing public %s access for warehouse staff',kind=>{expect(canOpenPageEntry(warehouse,{document:{kind},sample_enabled:false})).toBe(true)})
 it('restricts the official price center without broadening other permissions',()=>{expect(canOpenPageEntry(warehouse,{document:{kind:'price'},sample_enabled:true})).toBe(false);expect(canOpenPageEntry({...warehouse,roles:['sales']},{document:{kind:'price'},sample_enabled:true})).toBe(true);expect(canOpenPageEntry({...warehouse,roles:['admin']},{document:{kind:'price',bean_center:true}})).toBe(true);expect(canOpenPageEntry(warehouse,{document:{kind:'price'},sample_enabled:false})).toBe(true)})
 it('requires an employee identity for the employee shortcut',()=>{expect(canUseEmployeePriceCenter({accountType:'customer',roles:['admin']})).toBe(false)})
})
