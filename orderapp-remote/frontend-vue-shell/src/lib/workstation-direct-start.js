import { startDraft, startPayload } from './operation-start.js'

const key = 'kferp:workstation-direct-start:pending:v1'

export function directStartStorage() {
  try { if (!window.sessionStorage) throw Error(); return window.sessionStorage }
  catch { throw new Error('浏览器存储不可用，无法安全保存本次开工记录。') }
}

export function readPendingStart(storage) {
  try {
    const raw = storage.getItem(key)
    if (!raw) return null
    const value = JSON.parse(raw)
    if (value.version !== 1 || !Number.isSafeInteger(value.jobCardId) || value.jobCardId <= 0 || !Number.isSafeInteger(value.employeeId) || value.employeeId <= 0 || !value.payload?.request_id || !value.payload?.expected_version || !Array.isArray(value.payload.materials)) throw Error()
    return value
  } catch { throw new Error('无法读取上次开工记录，请联系管理员核对任务与操作日志。') }
}

function savePendingStart(storage, value) {
  if (readPendingStart(storage)) throw new Error('存在尚未确认的开工，请先恢复原次操作。')
  try {
    storage.setItem(key, JSON.stringify(value))
    if (JSON.stringify(readPendingStart(storage)) !== JSON.stringify(value)) throw Error()
  } catch { throw new Error('无法保存开工重试记录，本次未提交。') }
}

function clearPendingStart(storage, value) {
  if (JSON.stringify(readPendingStart(storage)) !== JSON.stringify(value)) throw new Error('开工重试记录已变化，请联系管理员核对。')
  try { storage.removeItem(key); if (storage.getItem(key) !== null) throw Error() }
  catch { throw new Error('无法清除开工重试记录，请重试原次操作。') }
}

// A replay always uses the originally saved body and operator, even after reload.
export async function directStart({ jobCardId, employeeId, storage, get, post, makeID, issueOnly = false, requirePending = false, confirmRefresh = () => false }) {
  let pending = readPendingStart(storage)
  if (requirePending && !pending) throw new Error('上次开工记录已变化，请重新进入工位核对。')
  if (pending && (pending.employeeId !== employeeId || pending.jobCardId !== jobCardId)) throw new Error('请由原操作人员先恢复尚未确认的开工。')
  if (pending && issueOnly) throw new Error('请先重试尚未确认的开工。')
  if (!pending) {
    if (!Number.isSafeInteger(employeeId) || employeeId <= 0) throw new Error('请先绑定员工，再开工。')
    const preview = await get(`/api/job-cards/${jobCardId}/start-preview`)
    const draft = startDraft(preview)
    const shortages = preview.predecessor_required ? [] : draft.materials.filter(m => m.warehouse === 'wip' && Number.isFinite(Number(m.available_qty ?? 0)) && Number(m.qty) > Number(m.available_qty ?? 0) + 1e-8).map(m => ({...m, shortage_qty:Number((Number(m.qty)-Number(m.available_qty ?? 0)).toFixed(6))}))
    const input = { expected_version:preview.version, actual_input_qty:draft.actual_input_qty, input_unit:preview.input_unit, materials:draft.materials.map(m=>({...m})), difference_reason:'' }
    if (shortages.length || issueOnly) return { shortages, input }
    if (preview.blocked_reason) throw new Error(preview.blocked_reason)
    if (preview.bom_refresh_pending && !await confirmRefresh(preview)) return {cancelled:true}
    const payload = startPayload(preview, draft, makeID())
    pending = { version:1, jobCardId, employeeId, payload }
    savePendingStart(storage, pending)
  }
  try {
    const result = await post(`/api/job-cards/${jobCardId}/start`, {body:pending.payload})
    if (result?.ok !== true) throw new Error('开工结果不完整')
    clearPendingStart(storage, pending)
    return {result}
  } catch (error) {
    if (error.not_committed || error.status === 401 || error.status === 403) {
      clearPendingStart(storage, pending)
      throw error
    }
    throw new Error('尚未收到开工结果，请重试原次操作；系统不会重复扣料。')
  }
}
