async function readJson(res) {
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error || '请求失败')
  return data
}

export async function fetchRunningProduction() {
  const res = await fetch('/api/produce/running')
  return readJson(res)
}

export async function finishRunningProduction(payload) {
  const res = await fetch('/api/produce/running/finish', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  return readJson(res)
}

export async function cancelRunningProduction(id) {
  const res = await fetch('/api/produce/running/cancel', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id }),
  })
  return readJson(res)
}

