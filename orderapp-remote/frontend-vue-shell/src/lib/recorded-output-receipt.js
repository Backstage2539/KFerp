export function recordedOutputReceiptPayload(expectedVersion, requestID) {
  const version=String(expectedVersion||'').trim()
  const id=String(requestID||'').trim()
  if(!version) throw new Error('请刷新工单后再入库')
  if(!id) throw new Error('缺少本次请求标识')
  return {expected_version:version,request_id:id}
}

function storageKey(workOrderID) { return `kferp:recorded-output-receipt:${Number(workOrderID)}` }

export function readRecordedOutputReceiptAttempt(storage, workOrderID) {
  try {
    const raw=storage?.getItem(storageKey(workOrderID))
    if(!raw)return null
    const attempt=JSON.parse(raw)
    return attempt?.expected_version&&attempt?.request_id?attempt:null
  } catch { return null }
}

export function saveRecordedOutputReceiptAttempt(storage, workOrderID, attempt) {
  try { storage?.setItem(storageKey(workOrderID),JSON.stringify(attempt)) } catch { /* Retry still works for the current page session. */ }
}

export function clearRecordedOutputReceiptAttempt(storage, workOrderID) {
  try { storage?.removeItem(storageKey(workOrderID)) } catch { /* Best-effort cleanup. */ }
}
