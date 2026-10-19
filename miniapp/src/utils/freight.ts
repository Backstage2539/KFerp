import type {FreightSnapshot} from '../api/customerPortal'
export function freightLabel(f?:FreightSnapshot){if(!f)return '';if(f.status==='free')return f.estimate_known?'包邮':'包邮 · 快递成本待核';if(f.billing_status==='billed')return `实际运费已出账 ¥${(Number(f.billed_cents||0)/100).toFixed(2)}（另见账单）`;return `预估运费 ¥${(f.estimated_total_cents/100).toFixed(2)} · 待出账`}
