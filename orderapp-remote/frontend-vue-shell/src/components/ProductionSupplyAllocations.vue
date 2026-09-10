<template>
  <section v-if="rows.length" class="supply-allocations">
    <h4>关联在途供应</h4>
    <div class="table-wrap"><table><thead><tr><th>半成品</th><th>供应工单</th><th>产出仓</th><th>已分配</th><th>已兑现</th><th>状态</th></tr></thead><tbody>
      <tr v-for="(row, i) in rows" :key="row.id || i"><td>{{ row.material_name }}</td><td>{{ row.supplier_work_order_no }}</td><td>{{ row.warehouse }}</td><td>{{ quantity(row.required_g, row.required_units) }}</td><td>{{ quantity(row.delivered_g, row.delivered_units) }}</td><td>{{ labels[row.status] || row.status }}<strong v-if="row.status === 'shortfall'">：待补产 {{ quantity(row.required_g - row.delivered_g, row.required_units - row.delivered_units) }}</strong></td></tr>
    </tbody></table></div>
  </section>
</template>
<script setup>
defineProps({ rows: { type: Array, default: () => [] } })
const labels = { proposed: '待提交校验', reserved: '已分配', released: '已释放', shortfall: '上游少产' }
function quantity(g, units) { return [Number(g) > 0 ? `${Number((Number(g) / 1000).toFixed(6))} kg` : '', Number(units) > 0 ? `${units} 件` : ''].filter(Boolean).join(' / ') || '0' }
</script>
<style scoped>
.table-wrap{overflow:auto}table{width:100%;border-collapse:collapse}th,td{padding:8px;text-align:left;border-bottom:1px solid #e5e7eb}strong{color:#b45309}h4{margin:8px 0}
</style>
