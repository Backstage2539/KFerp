import {describe,it,expect,vi,beforeEach} from 'vitest'
vi.mock('./client',()=>({miniRequest:vi.fn().mockResolvedValue({})}))
import {miniRequest} from './client'
import {fetchSamples,submitSample,confirmSample,fetchStaffNotifications,readStaffNotification} from './samples'
describe('sample API scopes',()=>{
 beforeEach(()=>vi.clearAllMocks())
 it('separates customer and employee endpoints',async()=>{await fetchSamples('visitor',false,{q:'云南 & 豆',page:2});expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/sample-requests?page_size=20&q=%E4%BA%91%E5%8D%97%20%26%20%E8%B1%86&page=2',{token:'visitor'});await fetchSamples('staff',true);expect(vi.mocked(miniRequest).mock.calls[1][0]).toContain('/employee/sample-requests')})
 it('retains idempotency key and zero prices without overwriting registration',async()=>{const data={request_id:'stable-request',entry_key:'entry',publication_id:3,product_keys:['3:0:1'],receiver:{name:'客户',phone:'13800000001',address:'上海'},note:''};await submitSample('t',data);expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/sample-requests',{token:'t',method:'POST',data});const confirm={revision:2,customer_id:1,receiver:data.receiver,items:[{product_id:1,bom_spec_id:3,qty:1,unit_price:0}],shipping_amount:0,note:''};await confirmSample('t',9,confirm);expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/employee/sample-requests/9/confirm',{token:'t',method:'POST',data:confirm})})
 it('uses shared notifications and explicit read operation',async()=>{await fetchStaffNotifications('t',true);expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/employee/notifications?status=unread',{token:'t'});await readStaffNotification('t',7);expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/employee/notifications/7/read',{token:'t',method:'POST'})})
})
