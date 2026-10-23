<template>
  <div class="operations-overview">
    <div class="overview-title"><div><h3>客户待办</h3><p>从待办开始，把每一单处理到位。</p></div><span>按当前客户全部业务统计</span></div>
    <div class="todo-cards"><button v-for="card in cards" :key="card.key" :data-todo="card.key" :class="['todo-card',card.tone]" @click="emit('navigate',card)">
      <div class="todo-label"><span>{{ card.label }}</span><component :is="icons[card.key]" :size="20" aria-hidden="true" /></div>
      <div class="todo-value">{{ card.value }}<small>{{ card.unit }}</small></div>
      <div class="todo-hint"><span>{{ card.hint }}</span><IconArrowUpRight :size="17" aria-hidden="true" /></div>
    </button></div>
    <div class="overview-columns">
      <section class="overview-panel"><header><h4>需要处理</h4><span>从这里直接进入对应业务</span></header>
        <button v-for="item in attention" :key="item.key" class="attention-row" :disabled="!item.value" @click="emit('navigate', item)"><span class="attention-icon"><component :is="item.icon" :size="20" aria-hidden="true" /></span><span class="attention-copy"><strong>{{ item.label }}</strong><small>{{ item.hint }}</small></span><strong class="attention-count">{{ item.value }}<small>单</small></strong><IconChevronRight :size="17" aria-hidden="true" /></button>
        <p class="overview-foot">同一订单可能有多项待办，各项数量不相加。</p>
      </section>
      <section class="overview-panel financial-summary"><header><h4>费用速览</h4><button @click="emit('navigate',{tab:'fees',queue:''})">查看费用<IconArrowUpRight :size="15" /></button></header>
        <div><span>订单待收</span><strong>¥ {{ money(totals.pending_order_cents) }}</strong><small>货款与已出账运费</small></div>
        <div><span>待结算费用</span><strong>¥ {{ money(totals.unsettled_fee_cents) }}</strong><small>尚未归入结算单</small></div>
        <p class="overview-foot">预估运费用于参考，实际运费出账后再收取。已归入结算单的费用不重复累计。</p>
      </section>
    </div>
  </div>
</template>
<script setup>
import { computed } from 'vue'
import { IconPackage, IconTruckDelivery, IconReceipt, IconArrowUpRight, IconChevronRight, IconMapPin, IconChecklist, IconWallet } from '@tabler/icons-vue'
import { fulfillmentTodoCards } from '../lib/fulfillment-operations.js'
const props=defineProps({totals:{type:Object,default:()=>({})}})
const emit=defineEmits(['navigate'])
const icons={stock_waiting:IconPackage,pending_shipment:IconTruckDelivery,settlement:IconReceipt}
const cards=computed(()=>fulfillmentTodoCards(props.totals))
const attention=computed(()=>[
 {key:'missing_recipient',label:'补齐收件信息',hint:'姓名、电话或地址待完善',value:Number(props.totals.missing_recipient||0),icon:IconMapPin,tab:'dispatch',queue:'missing_recipient'},
 {key:'pending_confirmation',label:'跟进订单确认',hint:'客户尚未确认的订单变更',value:Number(props.totals.pending_confirmation||0),icon:IconChecklist,tab:'orders',queue:'pending_confirmation'},
 {key:'pending_payment',label:'核对订单收款',hint:'有未收金额的履约订单',value:Number(props.totals.pending_payment||0),icon:IconWallet,tab:'orders',queue:'pending_payment'},
])
const money=value=>(Number(value||0)/100).toLocaleString('zh-CN',{minimumFractionDigits:2,maximumFractionDigits:2})
</script>
<style scoped>
.operations-overview{color:#142033}.overview-title{display:flex;align-items:end;justify-content:space-between;margin:24px 0 18px;gap:12px}.overview-title h3{font-size:21px;margin:0 0 7px;letter-spacing:-.4px}.overview-title p,.overview-title>span{color:#718097;font-size:12px;margin:0}.todo-cards{display:grid;grid-template-columns:repeat(3,1fr);gap:16px}.todo-card{text-align:left;background:#fff;border:1px solid #e1e7ee;border-radius:10px;padding:20px;cursor:pointer;color:inherit;transition:border-color .15s,box-shadow .15s}.todo-card:hover{border-color:#278456;box-shadow:0 3px 12px #14203308}.todo-label{display:flex;align-items:center;justify-content:space-between;font-size:13px}.todo-label svg{color:#718097}.amber .todo-label svg{color:#b1802b}.green .todo-label svg{color:#278456}.todo-value{font-size:34px;font-weight:650;letter-spacing:-.5px;margin:20px 0 18px;font-variant-numeric:tabular-nums}.todo-value small{font-size:12px;font-weight:400;color:#718097;margin-left:9px}.todo-hint{display:flex;justify-content:space-between;gap:8px;font-size:12px;color:#718097}.overview-columns{display:grid;grid-template-columns:minmax(0,1.65fr) minmax(260px,1fr);gap:16px;margin-top:22px}.overview-panel{background:#fff;border:1px solid #e1e7ee;border-radius:10px;padding:20px;min-width:0}.overview-panel header{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:12px}.overview-panel h4{font-size:15px;margin:0}.overview-panel header>span{font-size:11px;color:#718097}.attention-row{display:flex;align-items:center;gap:14px;width:100%;padding:20px 0;border:0;border-bottom:1px solid #edf0f4;background:#fff;color:inherit;text-align:left;cursor:pointer}.attention-row:disabled{cursor:default}.attention-icon{background:#f7f9fb;color:#718097;display:flex;align-items:center;justify-content:center;width:38px;height:38px;border-radius:8px}.attention-copy{flex:1}.attention-copy strong{font-size:13px;font-weight:550}.attention-copy small{display:block;font-size:11px;color:#718097;margin-top:6px}.attention-count{font-size:18px;font-variant-numeric:tabular-nums}.attention-count small{font-size:11px;font-weight:400;color:#718097;margin-left:6px}.attention-row>svg{color:#9ca6b2}.overview-foot{font-size:11px;color:#718097;line-height:1.8;margin:17px 0 0}.financial-summary header button{display:flex;align-items:center;gap:5px;background:none;border:0;color:#278456;font-size:12px;cursor:pointer}.financial-summary>div{padding:20px 0;border-bottom:1px solid #edf0f4}.financial-summary>div span{font-size:12px;color:#718097}.financial-summary>div strong{display:block;font-size:23px;font-weight:600;margin:9px 0}.financial-summary small{font-size:11px;color:#718097}@media(max-width:900px){.overview-columns{grid-template-columns:1fr}.overview-title>span{display:none}}@media(max-width:650px){.todo-cards{grid-template-columns:1fr}.todo-value{margin:12px 0}.todo-card{padding:16px}}
</style>
