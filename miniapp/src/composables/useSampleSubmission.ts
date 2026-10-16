import {ref} from 'vue'
import {MiniRequestError} from '../api/client'
import type {SampleSubmit,SampleRequest} from '../api/samples'
import {sampleDraftID} from '../utils/samples'
type SavedDraft={payload:SampleSubmit;pending:boolean}
type SubmissionOptions={payload:SampleSubmit;pending:boolean;storage:{get:()=>SavedDraft|undefined;set:(value:SavedDraft)=>void;remove:()=>void};request:(value:SampleSubmit)=>Promise<SampleRequest>;onSuccess:(id:number)=>void}
export function useSampleSubmission(options:SubmissionOptions){
 const draft=ref(options.payload),busy=ref(false),error=ref(''),pending=ref(options.pending),successID=ref(0)
 let alive=true
 function dispose(){alive=false}
 async function submit(){
  if(busy.value||successID.value||!draft.value.receiver.name.trim()||!draft.value.receiver.phone.trim()||!draft.value.receiver.address.trim())return
  const payload:SampleSubmit=JSON.parse(JSON.stringify(draft.value))
  busy.value=true;error.value=''
  try{options.storage.set({payload,pending:true})}catch{error.value='无法保存本次申请，请释放存储空间后重试';busy.value=false;return}
  pending.value=true
  try{
   const result=await options.request(payload)
   // A previous page may finish after a different draft has been opened.
   try{if(options.storage.get()?.payload.request_id===payload.request_id)options.storage.remove()}catch{/* Replaying the saved ID still returns this same application. */}
   if(alive){successID.value=result.id;options.onSuccess(result.id)}
  }catch(cause){
   if(alive)error.value=cause instanceof Error?cause.message:'提交失败，请重试'
   if(cause instanceof MiniRequestError&&cause.statusCode>=400&&cause.statusCode<500&&cause.statusCode!==409){
    pending.value=false;draft.value.request_id=sampleDraftID()
    try{if(options.storage.get()?.payload.request_id===payload.request_id)options.storage.set({payload:draft.value,pending:false})}catch{/* A future submit must save successfully before sending. */}
   }
  }finally{busy.value=false}
 }
 return{draft,busy,error,pending,successID,submit,dispose}
}
