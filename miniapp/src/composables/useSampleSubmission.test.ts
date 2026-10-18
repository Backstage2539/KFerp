import {describe,it,expect,vi} from 'vitest'
import {useSampleSubmission} from './useSampleSubmission'
import type {SampleSubmit,SampleRequest} from '../api/samples'
const payload:SampleSubmit={request_id:'stable-id',entry_key:'entry',publication_id:1,product_keys:['1:0:0'],receiver:{name:'客户',phone:'13800000000',address:'云南'},note:''}
const result={id:42} as SampleRequest
function setup(){let stored:any;let resolve!:(r:SampleRequest)=>void;const request=vi.fn(()=>new Promise<SampleRequest>(r=>{resolve=r}));const storage={get:vi.fn(()=>stored),set:vi.fn((value:any)=>{stored=value}),remove:vi.fn(()=>{stored=undefined})};const success=vi.fn();const form=useSampleSubmission({payload:structuredClone(payload),pending:false,storage,request,onSuccess:success});return{form,storage,request,success,finish:()=>resolve(result),saved:()=>stored}}
describe('sample submission lifecycle',()=>{
 it('keeps one request in flight and preserves the id for retry',async()=>{const s=setup();const first=s.form.submit();await s.form.submit();expect(s.request).toHaveBeenCalledTimes(1);expect(s.saved().payload.request_id).toBe('stable-id');s.finish();await first;expect(s.form.successID.value).toBe(42);expect(s.success).toHaveBeenCalledWith(42);expect(s.storage.remove).toHaveBeenCalledTimes(1)})
 it('does not emit into a departed page or remove a newer draft',async()=>{const s=setup();const first=s.form.submit();s.form.dispose();s.storage.set({payload:{...payload,request_id:'newer-id'},pending:true});s.finish();await first;expect(s.success).not.toHaveBeenCalled();expect(s.saved().payload.request_id).toBe('newer-id')})
 it('does not send if the request identity cannot be saved locally',async()=>{const s=setup();s.storage.set.mockImplementation(()=>{throw new Error('quota')});await s.form.submit();expect(s.request).not.toHaveBeenCalled();expect(s.form.busy.value).toBe(false);expect(s.form.pending.value).toBe(false);expect(s.form.error.value).toContain('保存')})
 it('retains the exact payload after an uncertain network outcome',async()=>{const s=setup();s.request.mockRejectedValueOnce(new Error('网络错误'));await s.form.submit();expect(s.form.pending.value).toBe(true);expect(s.saved().payload).toEqual(payload);expect(s.form.busy.value).toBe(false)})
})
