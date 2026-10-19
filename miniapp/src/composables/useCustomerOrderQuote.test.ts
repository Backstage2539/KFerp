import { describe, expect, it, vi } from 'vitest'
import { useCustomerOrderQuote } from './useCustomerOrderQuote'
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>(r => { resolve = r }); return { resolve, promise } }
function form() {
  let available = 20
  const state = { preview: null as null | { can_submit: boolean; available: number }, error: '' }
  const quote = vi.fn(async () => ({ can_submit: available >= 22, available }))
  const flow = useCustomerOrderQuote({ validate: () => '', quote, clear: () => { state.preview = null }, apply: (value) => { state.preview = value }, error: (value) => { state.error = value } })
  return { state, flow, quote, setAvailable: (value: number) => { available = value } }
}
describe('补购返回重新获取目录并核价', () => {
  it('可用20、原单22，补购2后返回采用22的新报价', async () => {
    const f = form()
    await f.flow.refresh(async () => {})
    expect(f.state.preview?.can_submit).toBe(false)
    await f.flow.refresh(async () => { f.setAvailable(22) })
    expect(f.state.preview).toEqual({ available: 22, can_submit: true })
    expect(f.flow.previewing.value).toBe(false)
  })
  it('旧核价失效后立即结束忙碌状态，晚到响应不覆盖新报价', async () => {
    const f = form(); await f.flow.refresh(async () => {})
    const old = deferred<{ available: number; can_submit: boolean }>()
    f.quote.mockReturnValueOnce(old.promise)
    const running = f.flow.preview()
    expect(f.flow.previewing.value).toBe(true)
    f.flow.invalidate()
    expect(f.flow.previewing.value).toBe(false)
    await f.flow.refresh(async () => { f.setAvailable(22) })
    old.resolve({ available: 20, can_submit: false }); await running
    expect(f.state.preview?.available).toBe(22)
    expect(f.flow.previewing.value).toBe(false)
  })
  it('刷新失败不能沿用旧报价，草稿不受影响且可重试', async () => {
    const f = form(); await f.flow.refresh(async () => {})
    await f.flow.refresh(async () => { throw new Error('目录加载失败') })
    expect(f.state.preview).toBeNull(); expect(f.flow.ready.value).toBe(false)
    expect(f.state.error).toBe('目录加载失败')
    await f.flow.refresh(async () => { f.setAvailable(22) })
    expect(f.flow.ready.value).toBe(true); expect(f.state.preview?.can_submit).toBe(true)
  })
  it('多次返回仅采用最后一次目录，销毁后不再更新', async () => {
    const f = form(); const old = deferred<void>()
    const first = f.flow.refresh(async current => { await old.promise; if (current()) f.setAvailable(1) })
    await f.flow.refresh(async () => { f.setAvailable(22) })
    old.resolve(); await first
    expect(f.state.preview?.available).toBe(22)
    f.flow.dispose(); expect(f.flow.ready.value).toBe(false)
  })
})
