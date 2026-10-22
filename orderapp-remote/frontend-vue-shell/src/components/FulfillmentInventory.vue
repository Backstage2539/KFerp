<template>
  <section class="inventory-workspace">
    <div class="inventory-toolbar">
      <div class="inventory-tabs" role="tablist" aria-label="库存类型">
        <button v-for="item in types" :key="item.key" :class="{active: type === item.key}" role="tab" :aria-selected="type === item.key" @click="type = item.key">{{ item.label }} <span>{{ rows.filter(r => r.inventory_type === item.key).length }}</span></button>
      </div>
      <label class="inventory-search"><IconSearch :size="17" aria-hidden="true" /><input v-model="query" placeholder="搜索商品、规格或编码" aria-label="搜索客户库存" /></label>
    </div>
    <p class="inventory-note">可立即发货只包含已入库且可用的现货。已购待入库单独列出，可预订，不能直接发货。</p>
    <div class="inventory-table-wrap"><table>
      <thead><tr><th>商品 / 规格</th><th>所在仓库</th><th class="number">实物库存</th><th class="number">现货占用</th><th class="number">可立即发货</th><th class="number">已购待入库</th><th>操作</th></tr></thead>
      <tbody><tr v-for="row in pagedRows" :key="`${row.inventory_type}-${row.item_id}-${row.bom_spec_id}-${row.spec_g}`">
        <td><strong>{{ row.item_name }}</strong><small>{{ row.spec || '无规格' }} · {{ row.item_code || '未设置编码' }}</small></td>
        <td><span>{{ row.warehouses?.map(w => w.warehouse_name || w.warehouse_code).join('、') || '—' }}</span></td>
        <td class="number">{{ quantities(row).physical }} <small>{{ row.unit }}</small></td>
        <td class="number">{{ quantities(row).reserved }}</td>
        <td class="number available">{{ quantities(row).shippable }}</td>
        <td class="number">{{ quantities(row).pending }}<small v-if="quantities(row).pendingReserved">其中已预订 {{ quantities(row).pendingReserved }}</small></td>
        <td><button class="text-button" @click="selected = row">查看明细</button></td>
      </tr><tr v-if="!filtered.length"><td colspan="7" class="inventory-empty">{{ query ? '没有匹配的库存，试试其他关键词' : '此客户暂无这类库存' }}</td></tr></tbody>
    </table></div>
    <PaginationControls :page="page" :page-size="pageSize" :total="filtered.length" @change="changePage" />
    <p class="inventory-foot">{{ filtered.length }} 项 · 所有数量按各商品库存单位显示，跨单位不相加</p>
    <div v-if="selected" class="inventory-mask" @click.self="selected = null" @keydown.esc="selected = null">
      <aside ref="drawerElement" class="inventory-drawer" role="dialog" aria-modal="true" aria-label="客户库存明细" tabindex="-1">
        <header><div><p>客户库存明细</p><h3>{{ selected.item_name }}</h3><p>{{ selected.spec }} · {{ selected.unit }}</p></div><button @click="selected = null" aria-label="关闭库存明细"><IconX :size="20" /></button></header>
        <div class="inventory-detail-stats"><div><span>实物库存</span><strong>{{ quantities(selected).physical }}</strong></div><div><span>可立即发货</span><strong>{{ quantities(selected).shippable }}</strong></div><div><span>已购待入库</span><strong>{{ quantities(selected).pending }}</strong></div></div>
        <h4>仓库分布</h4><p v-for="warehouse in selected.warehouses" :key="warehouse.warehouse_code">{{ warehouse.warehouse_name || warehouse.warehouse_code }} · 可预订 {{ warehouse.available_qty }} · 已占用 {{ warehouse.occupied_qty }}</p>
        <h4>已入库批次</h4><table><thead><tr><th>批次 / 入库时间</th><th>仓库</th><th>实存</th><th>可用</th><th>状态</th></tr></thead><tbody><tr v-for="batch in selected.batches" :key="`${batch.batch_id}-${batch.warehouse_code}`"><td>{{ batch.batch_no }}<small>{{ batch.inbound_at || '未记录入库时间' }}</small></td><td>{{ batch.warehouse_name || batch.warehouse_code }}</td><td>{{ batch.total_qty }}</td><td>{{ batch.available_qty }}</td><td>{{ label(batch.quality_status) }}</td></tr><tr v-if="!selected.batches?.length"><td colspan="5" class="inventory-empty">暂无已入库批次</td></tr></tbody></table>
      </aside>
    </div>
  </section>
</template>
<script setup>
import { computed, ref, watch } from 'vue'
import PaginationControls from './PaginationControls.vue'
import { useDialogFocus } from '../lib/dialog-focus.js'
import { IconSearch, IconX } from '@tabler/icons-vue'
import { fulfillmentStockQuantities as quantities, fulfillmentBusinessLabel as label } from '../lib/fulfillment-operations.js'
const props = defineProps({ rows: { type: Array, default: () => [] } })
const type = ref('finished_product'), query = ref(''), selected = ref(null)
const page = ref(1), pageSize = ref(20)
const drawerElement=ref(null)
useDialogFocus(computed(()=>!!selected.value),drawerElement,()=>{selected.value=null})
const types = [{key:'finished_product',label:'成品'},{key:'green_bean',label:'原料'},{key:'packaging',label:'包材'},{key:'semi_finished',label:'半成品'}]
const filtered = computed(() => props.rows.filter(r => r.inventory_type === type.value && `${r.item_name} ${r.spec} ${r.item_code}`.toLowerCase().includes(query.value.trim().toLowerCase())))
const pagedRows = computed(() => filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
watch([query, type, () => props.rows], () => { page.value = 1 })
function changePage(value) { page.value = value.page; pageSize.value = value.pageSize }
</script>
<style scoped>
.inventory-workspace{background:white;border:1px solid #e1e7ee;border-radius:10px;padding:20px;color:#142033}.inventory-toolbar{display:flex;align-items:center;justify-content:space-between;gap:16px;flex-wrap:wrap}.inventory-tabs{display:flex;gap:4px}.inventory-tabs button{border:0;border-radius:6px;background:none;color:#718097;padding:9px 14px;font-size:13px;cursor:pointer}.inventory-tabs button.active{color:#278456;background:#e8f6ed;font-weight:650}.inventory-tabs span{margin-left:6px;font-size:11px}.inventory-search{display:flex;align-items:center;gap:7px;border:1px solid #e1e7ee;border-radius:7px;padding:7px 10px;color:#718097}.inventory-search input{border:0;outline:none;background:none;font:inherit;font-size:13px;width:190px}.inventory-note,.inventory-foot{font-size:12px;color:#718097;margin:18px 0}.inventory-foot{margin-bottom:0}.inventory-table-wrap{overflow:auto}table{width:100%;border-collapse:collapse;font-size:13px}th{text-align:left;color:#718097;font-size:12px;font-weight:500;background:#f7f9fb;white-space:nowrap}th,td{padding:14px 12px;border-bottom:1px solid #edf0f4;vertical-align:middle}td strong{font-weight:600}small{display:block;color:#718097;font-size:11px;margin-top:5px;font-weight:400}.number{text-align:right;font-variant-numeric:tabular-nums}.available{color:#278456;font-weight:650}.text-button{color:#278456;background:none;border:0;white-space:nowrap;font:inherit;cursor:pointer}.inventory-empty{text-align:center;padding:64px 12px;color:#718097}.inventory-mask{position:fixed;inset:0;z-index:1000;background:#14203340;display:flex;justify-content:flex-end}.inventory-drawer{width:min(720px,95vw);height:100%;box-sizing:border-box;background:#fff;padding:28px;overflow:auto;box-shadow:-8px 0 40px #14203312}.inventory-drawer header{display:flex;justify-content:space-between;align-items:flex-start}.inventory-drawer header button{background:none;border:0;cursor:pointer}.inventory-drawer h3{font-size:24px;margin:6px 0}.inventory-drawer p{font-size:12px;color:#718097}.inventory-detail-stats{display:grid;grid-template-columns:repeat(3,1fr);gap:12px;margin:25px 0}.inventory-detail-stats div{background:#f7f9fb;border-radius:8px;padding:16px}.inventory-detail-stats span{display:block;font-size:12px;color:#718097}.inventory-detail-stats strong{display:block;font-size:24px;margin-top:8px}.inventory-drawer h4{margin-top:26px}@media(max-width:700px){.inventory-workspace{padding:12px}.inventory-search{width:100%}.inventory-tabs button{padding:8px}.inventory-drawer{padding:18px}}
</style>
