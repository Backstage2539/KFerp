import {apiGet, apiSend} from './client.js'
const base='/api/sample-requests'
function query(values) { return new URLSearchParams(Object.entries(values).filter(([,value])=>value!==undefined&&value!==null&&value!=='').map(([key,value])=>[key,String(value)])).toString() }
export function listSamples(filters={}) { return apiGet(`${base}?${query(filters)}`) }
export function getSample(id) { return apiGet(`${base}/${Number(id)}`) }
export function sampleOptions(id=0,q='') { return apiGet(`${base}${id?`/${Number(id)}`:''}/options?${query({q})}`) }
export function updateSample(id,action,body) { if(!['assign','confirm','followup'].includes(action)) throw new Error('未知样品操作'); return apiSend(`${base}/${Number(id)}/${action}`,{body}) }
