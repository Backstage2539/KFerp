<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>用户权限</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table>
          <thead>
            <tr><th>员工</th><th>电话</th><th>部门</th><th>账号</th><th>角色</th><th>操作</th></tr>
          </thead>
          <tbody>
            <tr v-for="employee in employees" :key="employee.id">
              <td>{{ employee.name }}</td>
              <td>{{ employee.phone }}</td>
              <td>{{ employee.department }}</td>
              <td class="account-cell">
                <label class="switch">
                  <input type="checkbox" :checked="accountOf(employee.id).login_enabled" @change="setEnabled(employee.id, $event.target.checked)" />
                  {{ accountOf(employee.id).login_enabled ? '可登录' : '已停用' }}
                </label>
                <div class="password-row">
                  <input v-model.trim="passwordMap[String(employee.id)]" type="password" :placeholder="passwordPlaceholder(employee.id)" />
                  <button class="secondary" type="button" @click="savePassword(employee.id)" :disabled="saving || !passwordMap[String(employee.id)]">{{ passwordActionLabel(employee.id) }}</button>
                </div>
              </td>
              <td>
                <label v-for="role in roles" :key="role.code" class="role">
                  <input
                    type="checkbox"
                    :value="role.code"
                    :checked="selectedRoles(employee.id).includes(role.code)"
                    @change="toggleRole(employee.id, role.code, $event.target.checked)" />
                  {{ role.name }}
                </label>
              </td>
              <td>
                <button class="primary" type="button" @click="save(employee.id)" :disabled="saving">保存</button>
              </td>
            </tr>
            <tr v-if="!employees.length"><td colspan="6" class="muted">暂无员工</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { onMounted, reactive, ref } from 'vue'
import { apiGet } from '../api/client'
import { fetchAuthAccounts, fetchEmployeeRoles, fetchRoles, resetEmployeePassword, saveEmployeeRoles, setAccountState } from '../api/auth'

const employees = ref([])
const roles = ref([])
const roleMap = reactive({})
const accountMap = reactive({})
const passwordMap = reactive({})
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)

function selectedRoles(employeeId) {
  return roleMap[String(employeeId)] || []
}

function accountOf(employeeId) {
  return accountMap[String(employeeId)] || { login_enabled: true, has_password: false }
}

function passwordActionLabel(employeeId) {
  const { has_password } = accountOf(employeeId)
  return has_password ? '重置密码' : '设置密码'
}

function passwordPlaceholder(employeeId) {
  const { has_password } = accountOf(employeeId)
  return has_password ? '新密码' : '设置密码'
}

function toggleRole(employeeId, roleCode, checked) {
  const key = String(employeeId)
  const current = new Set(roleMap[key] || [])
  if (checked) {
    current.add(roleCode)
  } else {
    current.delete(roleCode)
  }
  roleMap[key] = Array.from(current).sort()
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [employeeRows, roleRes, assignmentRes, accountRes] = await Promise.all([
      apiGet('/api/company/employees'),
      fetchRoles(),
      fetchEmployeeRoles(),
      fetchAuthAccounts(),
    ])
    employees.value = Array.isArray(employeeRows) ? employeeRows : []
    roles.value = roleRes.roles || []
    for (const key of Object.keys(roleMap)) delete roleMap[key]
    const assignments = assignmentRes.assignments || {}
    for (const [employeeId, roleCodes] of Object.entries(assignments)) {
      roleMap[String(employeeId)] = Array.isArray(roleCodes) ? [...roleCodes].sort() : []
    }
    for (const key of Object.keys(accountMap)) delete accountMap[key]
    for (const row of accountRes.rows || []) {
      accountMap[String(row.employee_id)] = {
        ...row,
        login_enabled: !row.login_disabled,
      }
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function setEnabled(employeeId, loginEnabled) {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await setAccountState(employeeId, loginEnabled)
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

async function savePassword(employeeId) {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await resetEmployeePassword(employeeId, passwordMap[String(employeeId)])
    passwordMap[String(employeeId)] = ''
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '密码保存失败'
  } finally {
    saving.value = false
  }
}

async function save(employeeId) {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await saveEmployeeRoles(employeeId, selectedRoles(employeeId))
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e1e5ea; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
h2 { margin: 0; font-size: 20px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 980px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eef1f4; padding: 10px 8px; text-align: left; vertical-align: top; font-size: 14px; }
th { background: #f8fafc; }
.role { display: inline-flex; align-items: center; gap: 5px; margin: 0 12px 8px 0; white-space: nowrap; }
.account-cell { min-width: 240px; }
.switch { display: inline-flex; align-items: center; gap: 6px; margin-bottom: 8px; white-space: nowrap; }
.password-row { display: grid; grid-template-columns: minmax(130px, 1fr) 88px; gap: 6px; }
.password-row input { height: 34px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 6px 8px; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) { .page { padding: 12px; } }
</style>
