import { test } from 'node:test'
import assert from 'node:assert/strict'
import { listSamples, sampleOptions, updateSample } from './samples.js'
test('staff sample APIs preserve scoped filters and optimistic revision', async () => {
 const seen = []; const original = globalThis.fetch
 globalThis.fetch = async (url, options) => { seen.push([String(url), options]); return new Response(JSON.stringify({rows: []}), {status: 200}) }
 try {
  await listSamples({q: '昵称 & 电话', page: 2, mini_user_id: 7})
  await sampleOptions(12, '客户名')
  await updateSample(12, 'confirm', {revision: 4, shipping_amount: 0})
  assert.match(seen[0][0], /mini_user_id=7/); assert.match(seen[0][0], /page=2/)
  assert.match(seen[1][0], /\/12\/options\?q=/)
  assert.equal(seen[2][0], '/api/sample-requests/12/confirm')
  assert.deepEqual(JSON.parse(seen[2][1].body), {revision: 4, shipping_amount: 0})
 } finally { globalThis.fetch = original }
})
