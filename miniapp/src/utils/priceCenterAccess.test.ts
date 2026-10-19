import {describe,it,expect} from 'vitest'
import {canUseEmployeePriceCenter,canOpenPageEntry,hasAuthenticatedCustomerSession,pageEntryAccessAction} from './priceCenterAccess'
const warehouse={accountType:'employee',roles:['warehouse']}
describe('price center employee access scope',()=>{
 it('authenticated customers reuse their session but need an approved binding',()=>{
  const customer={token:'session',miniUserID:1,accountType:'customer',bindings:[{status:'approved'}]}
  expect(hasAuthenticatedCustomerSession(customer)).toBe(true)
  expect(hasAuthenticatedCustomerSession({...customer,accountType:''})).toBe(true)
  expect(hasAuthenticatedCustomerSession({...customer,bindings:[{status:'revoked'}]})).toBe(false)
  expect(hasAuthenticatedCustomerSession({...customer,token:''})).toBe(false)
  expect(pageEntryAccessAction({statusCode:403,code:'mini_price_access_denied'})).toBe('denied')
  expect(pageEntryAccessAction({statusCode:403,code:'mini_customer_selection_required'})).toBe('choose_customer')
  expect(pageEntryAccessAction({statusCode:401})).toBe('login')
  expect(pageEntryAccessAction(new Error('network'))).toBe('retry')
 })
 it.each(['article','function'])('preserves existing public %s access for warehouse staff',kind=>{expect(canOpenPageEntry(warehouse,{document:{kind},sample_enabled:false})).toBe(true)})
 it('restricts the official price center without broadening other permissions',()=>{expect(canOpenPageEntry(warehouse,{document:{kind:'price'},sample_enabled:true})).toBe(false);expect(canOpenPageEntry({...warehouse,roles:['sales']},{document:{kind:'price'},sample_enabled:true})).toBe(true);expect(canOpenPageEntry({...warehouse,roles:['admin']},{document:{kind:'price',bean_center:true}})).toBe(true);expect(canOpenPageEntry(warehouse,{document:{kind:'price'},sample_enabled:false})).toBe(true)})
 it('requires an employee identity for the employee shortcut',()=>{expect(canUseEmployeePriceCenter({accountType:'customer',roles:['admin']})).toBe(false)})
})
