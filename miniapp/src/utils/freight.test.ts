import {describe,it,expect} from 'vitest'
import {freightLabel} from './freight'
describe('freight presentation',()=>{it('does not describe deferred or unknown freight as zero',()=>{expect(freightLabel({status:'pending_actual',estimated_total_cents:2000} as never)).toBe('预估运费 ¥20.00 · 待出账');expect(freightLabel({status:'free',estimate_known:false} as never)).toBe('包邮 · 快递成本待核');expect(freightLabel({status:'pending_actual',billing_status:'billed',billed_cents:2200} as never)).toContain('已出账 ¥22.00')})})
