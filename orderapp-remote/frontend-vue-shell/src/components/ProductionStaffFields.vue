<template>
  <div class="staff-fields">
    <label class="lead-field"><span>{{ defaults ? '默认负责人' : '负责人' }} <b>*</b></span>
      <select :value="modelValue.assigned_employee_id || ''" :disabled="disabled" @change="updateLead($event.target.value)">
        <option value="">请选择本工序负责人</option>
        <option v-for="person in eligible" :key="person.id" :value="person.id">{{ person.name }}</option>
      </select>
    </label>
    <fieldset :disabled="disabled"><legend>{{ defaults ? '默认协作人员' : '协作人员' }} <small>可多选，选填</small></legend>
      <label v-for="person in eligible.filter(p => Number(p.id) !== Number(modelValue.assigned_employee_id))" :key="person.id" class="staff-choice" :class="{ selected: selected(person.id) }">
        <input type="checkbox" :checked="selected(person.id)" @change="toggle(person.id, $event.target.checked)" /><span>{{ person.name }}</span>
      </label>
      <p v-if="eligible.length < 2" class="muted">暂无其他可执行员工</p>
    </fieldset>
    <p v-if="!eligible.length" class="warning"><template v-if="defaults">请先勾选上方的可执行员工。</template><template v-else>本工序还没有可执行员工。<button type="button" class="text-action" @click="$emit('configure')">去配置人员</button></template></p>
  </div>
</template>
<script setup>
import { computed } from 'vue'
const props = defineProps({ modelValue: { type: Object, required: true }, employees: { type: Array, default: () => [] }, disabled: Boolean, defaults: Boolean })
const emit = defineEmits(['update:modelValue', 'configure'])
const eligible = computed(() => props.employees.filter(person => person.active !== false))
const selected = id => (props.modelValue.collaborator_employee_ids || []).map(Number).includes(Number(id))
function updateLead(value) { const id = Number(value || 0); emit('update:modelValue', { ...props.modelValue, assigned_employee_id: id, collaborator_employee_ids: (props.modelValue.collaborator_employee_ids || []).filter(personID => Number(personID) !== id) }) }
function toggle(id, checked) { const ids = new Set(props.modelValue.collaborator_employee_ids || []); if (checked) ids.add(Number(id)); else ids.delete(Number(id)); emit('update:modelValue', { ...props.modelValue, collaborator_employee_ids: [...ids] }) }
</script>
<style scoped>
.staff-fields { display:grid; gap:14px; min-width:0; }.lead-field { display:grid; gap:6px; }.lead-field span,legend { font-size:13px; color:#445b50; }b{color:#bc6618}select{width:100%;min-height:38px;border:1px solid #cfdad3;border-radius:8px;background:white;padding:7px 9px;font:inherit}fieldset{border:0;padding:0;margin:0;min-width:0;display:flex;gap:7px;flex-wrap:wrap}legend{margin-bottom:8px}legend small,.muted{color:#7b8780;font-size:12px}.staff-choice{display:flex;align-items:center;gap:6px;border:1px solid #dce3de;border-radius:7px;padding:7px 9px;background:#fff;cursor:pointer}.staff-choice.selected{background:#edf8f0;border-color:#87bc9b;color:#247346}.staff-choice input{width:15px;height:15px;accent-color:#2f8f5b;margin:0}.warning{padding:10px;background:#fff8e9;color:#9c5f20;font-size:13px;border-radius:7px}.text-action{border:0;background:none;color:#2470aa;padding:0;font:inherit;cursor:pointer}
</style>
