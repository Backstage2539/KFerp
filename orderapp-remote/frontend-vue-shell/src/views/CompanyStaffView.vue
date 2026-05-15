<template>
  <div class="page">
    <section class="panel">
      <div class="panel-head">
        <h2>{{ departmentMode ? '部门维护' : '员工维护' }}</h2>
        <button class="secondary" type="button" @click="load" :disabled="loading">刷新</button>
      </div>
      <div v-if="error" class="error">{{ error }}</div>
      <div v-if="ok" class="ok">已保存</div>
    </section>

    <section class="panel">
      <div class="section-title">新增</div>
      <div v-if="departmentMode" class="form-grid dept">
        <label><span>部门名</span><input v-model.trim="departmentForm.name" /></label>
        <label class="checkline"><input v-model="departmentForm.active" type="checkbox" />启用</label>
        <button class="primary" type="button" @click="createDepartment" :disabled="saving">新增</button>
      </div>
      <div v-else class="form-grid emp">
        <label><span>姓名</span><input v-model.trim="employeeForm.name" /></label>
        <label><span>电话</span><input v-model.trim="employeeForm.phone" /></label>
        <label>
          <span>部门</span>
          <select v-model.number="employeeForm.department_id">
            <option :value="0">请选择</option>
            <option v-for="dept in departments" :key="dept.id" :value="dept.id">{{ dept.name }}</option>
          </select>
        </label>
        <label class="checkline"><input v-model="employeeForm.active" type="checkbox" />启用</label>
        <button class="primary" type="button" @click="createEmployee" :disabled="saving">新增</button>
      </div>
    </section>

    <section class="panel">
      <div class="table-wrap">
        <table v-if="departmentMode">
          <thead><tr><th>ID</th><th>部门</th><th>状态</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="row in departments" :key="row.id">
              <td>{{ row.id }}</td>
              <td><input v-model.trim="row.name" /></td>
              <td><input v-model="row.active" type="checkbox" /></td>
              <td><button class="secondary" type="button" @click="saveDepartment(row)" :disabled="saving">保存</button></td>
            </tr>
          </tbody>
        </table>
        <table v-else>
          <thead><tr><th>ID</th><th>姓名</th><th>电话</th><th>部门</th><th>状态</th><th>账号</th><th>内部权限</th><th>操作</th></tr></thead>
          <tbody>
            <tr v-for="row in employees" :key="row.id">
              <td>{{ row.id }}</td>
              <td><input v-model.trim="row.name" /></td>
              <td><input v-model.trim="row.phone" /></td>
              <td>
                <select v-model.number="row.department_id">
                  <option v-for="dept in departments" :key="dept.id" :value="dept.id">{{ dept.name }}</option>
                </select>
              </td>
              <td><input v-model="row.active" type="checkbox" /></td>
              <td class="account-cell">
                <label class="switch">
                  <input type="checkbox" :checked="accountOf(row.id).login_enabled" @change="setEnabled(row.id, $event.target.checked)" />
                  {{ accountOf(row.id).login_enabled ? '可登录' : '已停用' }}
                </label>
                <div class="password-row">
                  <input v-model.trim="passwordMap[String(row.id)]" type="password" :placeholder="passwordPlaceholder(row.id)" />
                  <button class="secondary" type="button" @click="savePassword(row.id)" :disabled="saving || !passwordMap[String(row.id)]">{{ passwordActionLabel(row.id) }}</button>
                </div>
              </td>
              <td class="roles-cell">
                <label v-for="role in roles" :key="role.code" class="role">
                  <input
                    type="checkbox"
                    :value="role.code"
                    :checked="selectedRoles(row.id).includes(role.code)"
                    @change="toggleRole(row.id, role.code, $event.target.checked)" />
                  {{ role.name }}
                </label>
              </td>
              <td class="action-cell">
                <button class="secondary" type="button" @click="saveEmployee(row)" :disabled="saving">保存资料</button>
                <button class="primary" type="button" @click="saveRoles(row.id)" :disabled="saving">保存权限</button>
              </td>
            </tr>
            <tr v-if="!employees.length"><td colspan="8" class="muted">暂无员工</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { apiGet, apiSend } from '../api/client'
import { fetchEmployeeRoles, fetchInternalAuthAccounts, fetchRoles, resetEmployeePassword, saveEmployeeRoles, setAccountState } from '../api/auth'

const props = defineProps({
  viewKey: { type: String, default: 'departments' },
})

const departmentMode = computed(() => props.viewKey === 'departments')
const departments = ref([])
const employees = ref([])
const loading = ref(false)
const saving = ref(false)
const error = ref('')
const ok = ref(false)
const roles = ref([])
const roleMap = reactive({})
const accountMap = reactive({})
const passwordMap = reactive({})
const departmentForm = reactive({ name: '', active: true })
const employeeForm = reactive({ name: '', phone: '', department_id: 0, active: true })

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
    const deps = await apiGet('/api/company/departments')
    departments.value = deps
    if (!departmentMode.value) {
      const [employeeRows, roleRes, assignmentRes, accountRes] = await Promise.all([
        apiGet('/api/company/employees'),
        fetchRoles(),
        fetchEmployeeRoles(),
        fetchInternalAuthAccounts(),
      ])
      employees.value = Array.isArray(employeeRows) ? employeeRows : []
      roles.value = (roleRes.roles || []).filter((role) => !String(role.code || '').startsWith('customer_'))
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
    }
  } catch (err) {
    error.value = err.message || '加载失败'
  } finally {
    loading.value = false
  }
}

async function createDepartment() {
  await save('/api/company/departments', 'POST', departmentForm)
  departmentForm.name = ''
}

async function saveDepartment(row) {
  await save(`/api/company/departments/${row.id}`, 'PUT', row)
}

async function createEmployee() {
  await save('/api/company/employees', 'POST', employeeForm)
  employeeForm.name = ''
  employeeForm.phone = ''
}

async function saveEmployee(row) {
  await save(`/api/company/employees/${row.id}`, 'PUT', row)
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

async function saveRoles(employeeId) {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await saveEmployeeRoles(employeeId, selectedRoles(employeeId))
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '权限保存失败'
  } finally {
    saving.value = false
  }
}

async function save(url, method, body) {
  saving.value = true
  error.value = ''
  ok.value = false
  try {
    await apiSend(url, { method, body })
    ok.value = true
    await load()
  } catch (err) {
    error.value = err.message || '保存失败'
  } finally {
    saving.value = false
  }
}

watch(() => props.viewKey, load)
onMounted(load)
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; }
.panel { border: 1px solid #e6e0d8; border-radius: 8px; background: #fff; padding: 14px; margin-bottom: 14px; }
.panel-head { display: flex; justify-content: space-between; align-items: center; gap: 12px; }
h2 { margin: 0; font-size: 20px; }
.section-title { font-weight: 700; margin-bottom: 10px; }
.form-grid { display: grid; gap: 10px; align-items: end; }
.form-grid.dept { grid-template-columns: 1fr 90px 84px; }
.form-grid.emp { grid-template-columns: 1fr 150px 180px 90px 84px; }
label span { display: block; color: #666; font-size: 12px; margin-bottom: 5px; }
input:not([type="checkbox"]), select { width: 100%; height: 38px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 7px 9px; font: inherit; background: #fff; }
.checkline { display: flex; gap: 6px; align-items: center; height: 38px; }
button { height: 38px; border-radius: 6px; border: 1px solid #1f1f1f; padding: 0 12px; font: inherit; cursor: pointer; }
.primary { background: #1f1f1f; color: #fff; }
.secondary { background: #fff; color: #1f1f1f; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 1180px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee8df; padding: 9px 8px; text-align: left; font-size: 14px; }
th { background: #fbfaf8; }
.account-cell { min-width: 240px; }
.roles-cell { min-width: 280px; }
.role { display: inline-flex; align-items: center; gap: 5px; margin: 0 12px 8px 0; white-space: nowrap; }
.switch { display: inline-flex; align-items: center; gap: 6px; margin-bottom: 8px; white-space: nowrap; }
.password-row { display: grid; grid-template-columns: minmax(130px, 1fr) 88px; gap: 6px; }
.password-row input { height: 34px; border: 1px solid #cfc8bf; border-radius: 6px; padding: 6px 8px; }
.action-cell { min-width: 100px; }
.action-cell button { width: 100%; margin-bottom: 8px; }
.muted { color: #666; text-align: center; }
.error, .ok { border-radius: 6px; padding: 9px; margin-top: 12px; }
.error { background: #fff0f0; border: 1px solid #e6b7b7; color: #8a1f1f; }
.ok { background: #f0fff0; border: 1px solid #b7d9b7; color: #246024; }
@media (max-width: 900px) { .page { padding: 12px; } .form-grid, .form-grid.dept, .form-grid.emp { grid-template-columns: 1fr; } }
</style>
