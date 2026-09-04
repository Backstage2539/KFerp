import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import { compileScript, parse } from '@vue/compiler-sfc'
import { createRenderer, h, nextTick, reactive } from 'vue'

// Compile the real SFCs and exercise their events with Vue's platform-neutral renderer.
async function loadComponent(name, replacements = {}) {
  const filename = new URL(`../components/${name}.vue`, import.meta.url)
  const { descriptor } = parse(fs.readFileSync(filename, 'utf8'), { filename: filename.pathname })
  let code = compileScript(descriptor, { id: name, inlineTemplate: true }).content
  const imports = {
    vue: import.meta.resolve('vue'),
    '@tabler/icons-vue': import.meta.resolve('@tabler/icons-vue'),
    '../lib/business-grouping.js': new URL('./business-grouping.js', import.meta.url).href,
    ...replacements,
  }
  code = code.replace(/from (['"])([^'"]+)\1/g, (match, quote, path) => (
    imports[path] ? `from ${JSON.stringify(imports[path])}` : match
  ))
  return `data:text/javascript;base64,${Buffer.from(code).toString('base64')}`
}
const controlsURL = await loadComponent('BusinessGroupControls')
const workspaceURL = await loadComponent('BusinessGroupInlineWorkspace', { './BusinessGroupControls.vue': controlsURL })
const { default: Workspace } = await import(workspaceURL)

function element(type, text = '') {
  return { type, text, props: {}, children: [], parent: null, scrollTop: 0, scrollIntoView() {}, querySelector() { return null } }
}
const renderer = createRenderer({
  createElement: element,
  createText: (text) => element('text', text),
  createComment: (text) => element('comment', text),
  setText: (node, text) => { node.text = text },
  setElementText: (node, text) => { node.text = text; node.children = [] },
  patchProp: (node, key, previous, next) => { node.props[key] = next },
  parentNode: (node) => node.parent,
  nextSibling: (node) => node.parent?.children[node.parent.children.indexOf(node) + 1],
  insert(node, parent, anchor = null) {
    if (node.parent) node.parent.children.splice(node.parent.children.indexOf(node), 1)
    node.parent = parent
    const index = anchor ? parent.children.indexOf(anchor) : -1
    if (index < 0) parent.children.push(node)
    else parent.children.splice(index, 0, node)
  },
  remove(node) { node.parent?.children.splice(node.parent.children.indexOf(node), 1); node.parent = null },
})
const groups = [
  { key: 'template', group_id: 1, is_template_group: true, label: '模板' },
  { key: 'parent', group_id: 1, group_item_id: 10, label: '大类', rows: [], total: 0, page: 1 },
  { key: 'child', group_id: 1, group_item_id: 11, parent_group_item_id: 10, label: '小类', rows: [{ id: 1 }], total: 11, page: 2, pageSize: 10 },
  { key: 'unclassified', unclassified: true, label: '未分类', rows: [], total: 0 },
]
function descendants(node) { return [node, ...node.children.flatMap(descendants)] }
function mount(t, initial = {}) {
  const state = reactive({ groups: structuredClone(groups), collapsedKeys: ['child'], moveActive: false, loading: false, searchQuery: '', selectedCount: 2, ...initial })
  const root = element('root')
  const updates = []
  const app = renderer.createApp({ render: () => h(Workspace, {
    ...state,
    'onUpdate:collapsedKeys': (keys) => { updates.push(keys); state.collapsedKeys = keys },
  }, { group: ({ group }) => h('div', { 'data-test-business-body': group.key }, '业务行与分页') }) })
  app.mount(root)
  t.after(() => app.unmount())
  const button = (label) => {
    const found = descendants(root).find((node) => node.type === 'button' && node.text === label)
    assert.ok(found, `missing ${label} button`)
    return found
  }
  return { state, root, updates, button, click: async (label) => { button(label).props.onClick(); await nextTick() } }
}

test('bulk expand and collapse reach every descendant without changing rows, pages or selection', async (t) => {
  const ui = mount(t)
  const dataBefore = JSON.stringify(ui.state.groups)
  await ui.click('全部收缩')
  assert.deepEqual([...ui.state.collapsedKeys], groups.map((group) => group.key))
  assert.equal(descendants(ui.root).filter((node) => node.props['data-test-business-body']).length, 0)
  await ui.click('全部展开')
  assert.deepEqual([...ui.state.collapsedKeys], [])
  assert.equal(descendants(ui.root).filter((node) => node.props['data-test-business-body']).length, 3)
  assert.equal(JSON.stringify(ui.state.groups), dataBefore)
  assert.equal(ui.state.selectedCount, 2)
})

test('bulk controls are guarded during loading and movement, including direct handler calls', async (t) => {
  for (const flag of ['loading', 'moveActive']) {
    const ui = mount(t, { [flag]: true })
    for (const label of ['全部展开', '全部收缩']) {
      assert.equal(ui.button(label).props.disabled, true)
      await ui.click(label)
    }
    assert.equal(ui.updates.length, 0)
  }
})

test('bulk controls safely disable on an empty workspace and cover the no-template flat group', async (t) => {
  const empty = mount(t, { groups: [] })
  assert.equal(empty.button('全部展开').props.disabled, true)
  assert.equal(empty.button('全部收缩').props.disabled, true)
  const flat = mount(t, { groups: [{ key: 'all', all: true, rows: [{ id: 2 }] }] })
  await flat.click('全部收缩')
  assert.deepEqual([...flat.state.collapsedKeys], ['all'])
  await flat.click('全部展开')
  assert.deepEqual([...flat.state.collapsedKeys], [])
})

test('leaving movement restores the prior bulk collapse state', async (t) => {
  const ui = mount(t)
  await ui.click('全部收缩')
  ui.state.moveActive = true
  await nextTick(); await nextTick()
  assert.equal(descendants(ui.root).filter((node) => 'data-inline-group-header' in node.props).length, 4)
  ui.state.moveActive = false
  await nextTick(); await nextTick()
  assert.deepEqual([...ui.state.collapsedKeys], groups.map((group) => group.key))
})

test('bulk toggles during search leave the pre-search snapshot intact', async (t) => {
  const ui = mount(t, { collapsedKeys: ['template', 'child'] })
  ui.state.searchQuery = '命中'
  await nextTick(); await nextTick(); await nextTick()
  assert.deepEqual([...ui.state.collapsedKeys], ['unclassified'])
  await ui.click('全部收缩')
  assert.deepEqual([...ui.state.collapsedKeys], groups.map((group) => group.key))
  await ui.click('全部展开')
  ui.state.searchQuery = ''
  await nextTick(); await nextTick()
  assert.deepEqual([...ui.state.collapsedKeys], ['template', 'child'])
})
