import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const appSource = readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
const systemSettingsSource = readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
const groupTemplatesSource = readFileSync(new URL('../views/GroupTemplatesView.vue', import.meta.url), 'utf8')
const businessSettingsSource = readFileSync(new URL('../views/BusinessSettingsView.vue', import.meta.url), 'utf8')
const globalUnitsSource = readFileSync(new URL('../views/GlobalUnitDefinitionsView.vue', import.meta.url), 'utf8')

test('system settings and group templates use independent Vue pages', () => {
  assert.match(appSource, /import GroupTemplatesView from '.\/views\/GroupTemplatesView\.vue'/)
  assert.match(appSource, /groupManagement:\s*GroupTemplatesView/)
  assert.match(appSource, /groupTemplates:\s*GroupTemplatesView/)
  assert.match(appSource, /businessSettings:\s*BusinessSettingsView/)
  assert.match(appSource, /uiSettings:\s*UISettingsView/)

  assert.match(systemSettingsSource, /<h2>系统设置<\/h2>/)
  assert.match(systemSettingsSource, /通知设置/)
  assert.doesNotMatch(systemSettingsSource, /全局单位字典/)
  assert.doesNotMatch(systemSettingsSource, /data-section-mode="groupTemplates"/)
  assert.doesNotMatch(systemSettingsSource, /<h2>分组模板<\/h2>/)
  assert.doesNotMatch(systemSettingsSource, /\/api\/business-groups/)
  assert.doesNotMatch(systemSettingsSource, /\/api\/business-group-items/)

  assert.match(groupTemplatesSource, /<h2>分组模板<\/h2>/)
  assert.match(groupTemplatesSource, /新增分组模板/)
  assert.match(groupTemplatesSource, /新增大类/)
  assert.match(groupTemplatesSource, /新增小类/)
  assert.match(groupTemplatesSource, /\/api\/business-groups/)
  assert.match(groupTemplatesSource, /\/api\/business-group-items/)
  assert.doesNotMatch(groupTemplatesSource, /功能引用/)
  assert.doesNotMatch(groupTemplatesSource, /businessGroupUsageOptions/)
  assert.doesNotMatch(groupTemplatesSource, /groupTemplateForm\.usage_keys/)
  assert.doesNotMatch(groupTemplatesSource, /replace_usages/)
  assert.doesNotMatch(groupTemplatesSource, /\busages\s*:/)
  assert.doesNotMatch(groupTemplatesSource, /客户账户模式隐藏履约运营台/)
  assert.doesNotMatch(groupTemplatesSource, /全局单位字典/)
  assert.doesNotMatch(groupTemplatesSource, /fetchUISettings/)

  assert.match(businessSettingsSource, /分组模板/)
  assert.match(businessSettingsSource, /全局单位字典/)
  assert.match(globalUnitsSource, /全局单位字典/)
})
