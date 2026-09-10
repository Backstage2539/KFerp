import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import vm from 'node:vm'
import { parse } from '@babel/parser'
import { ref } from 'vue'
const source = readFileSync(new URL('../components/OrderConfirmationPanel.vue', import.meta.url), 'utf8').split('<script setup>')[1].split('</script>')[0]
const ast = parse(source, { sourceType: 'module' })
const functions = ast.program.body.filter(n => n.type === 'FunctionDeclaration').map(n => source.slice(n.start, n.end)).join('\n')
function setup() {
  const calls = [], events = []
  const state = { props: { orderId: 1 }, status: ref({ confirmation_status: 'pending', confirmation_revision: 2 }), editing: ref(true), busy: ref(false), error: ref(''), message: ref(''), reason: ref(''), emit: event => events.push(event),
    apiGet: async () => ({ confirmation_status: 'accepted', confirmation_revision: 2 }),
    apiSend: async (url, options) => { calls.push({ url, ...options }); return {} },
  }
  const context = vm.createContext(state)
  vm.runInContext(`let revision=0;${functions}`, context)
  return { state, calls, events, context }
}
test('status refresh preserves the open editor and only changes the read model', async () => {
  const { state, context } = setup()
  await context.load()
  assert.equal(state.editing.value, true)
  assert.equal(state.status.value.confirmation_status, 'accepted')
})
test('reject requires a reason, and an in-flight review cannot be submitted twice', async () => {
  const { state, calls, context } = setup()
  await context.review('rejected')
  assert.match(state.error.value, /拒绝原因/); assert.equal(calls.length, 0)
  let complete
  state.apiSend = async (url, options) => { calls.push({ url, ...options }); await new Promise(resolve => { complete = resolve }) }
  state.reason.value = ' 数量有误 '
  const task = context.review('rejected')
  await context.review('accepted')
  assert.equal(calls.length, 1); assert.equal(state.busy.value, true)
  assert.equal(calls[0].body.revision, 2); assert.equal(calls[0].body.reason, '数量有误')
  complete(); await task
  assert.equal(state.busy.value, false)
})
test('stale polling response cannot overwrite the latest order status', async () => {
  const { state, context } = setup()
  let first
  state.apiGet = () => new Promise(resolve => { first = resolve })
  const older = context.load()
  state.apiGet = async () => ({ confirmation_status: 'pending', confirmation_revision: 3 })
  await context.load(); first({ confirmation_status: 'accepted', confirmation_revision: 2 }); await older
  assert.equal(state.status.value.confirmation_revision, 3)
})
test('review failure stays in the operation area and preserves editable content', async () => {
  const { state, events, context } = setup()
  state.apiSend = async () => { throw new Error('价格表已更新') }
  await context.review('accepted')
  assert.equal(state.editing.value, true); assert.equal(state.status.value.confirmation_status, 'pending')
  assert.match(state.error.value, /价格表已更新/); assert.equal(state.busy.value, false); assert.equal(events.length, 0)
})
