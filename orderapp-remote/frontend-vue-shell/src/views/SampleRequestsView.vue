<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import ProductionReturnLink from '../components/ProductionReturnLink.vue'
import { getSample, listSamples, sampleOptions, updateSample } from '../api/samples.js'
import { canConfirmSample, confirmRows, followupStatuses, sampleStatus, sampleStatuses, sampleTotal } from '../lib/samples.js'

const props = defineProps({ viewParams: { type: Object, default: () => ({}) } })
const rows = ref([]), total = ref(0), page = ref(1), q = ref(''), status = ref(''), isAdmin = ref(false)
const loading = ref(false), detailLoading = ref(false), saving = ref(false), error = ref(''), notice = ref(''), conflicted = ref(false)
const selected = ref(null), options = ref({}), assignee = ref(0), customerSearch = ref(''), customerMode = ref('existing')
const draft = ref({receiver: {}, items: [], customer_id: 0, new_customer: {}, shipping_amount: 0, note: ''})
const contacted = ref(false), followup = ref({status: 'following', note: ''})
let listGeneration = 0, detailGeneration = 0, customerGeneration = 0
const miniUserID = computed(() => Number(props.viewParams.mini_user_id || 0))
const terminal = computed(() => ['won', 'abandoned'].includes(selected.value?.followup_status))
const confirmable = computed(() => canConfirmSample(selected.value))
const selectedItems = computed(() => draft.value.items.filter(item => item.included))
const amount = computed(() => sampleTotal(selectedItems.value, draft.value.shipping_amount))
const customerTypeLabel = type => ({retail:'零售客户',ecommerce:'电商客户',wholesale:'批发客户',channel:'渠道客户'})[type] || type
const customerReady = computed(() => customerMode.value === 'existing' ? draft.value.customer_id > 0 : !!(draft.value.new_customer.name.trim() && draft.value.new_customer.customer_type && draft.value.new_customer.source_id > 0 && draft.value.new_customer.order_type_id > 0))
const money = value => Number(value || 0).toFixed(2)
const date = value => value ? new Date(value).toLocaleString('zh-CN', {hour12: false}) : '—'
const specsFor = productID => (options.value.specs || []).filter(spec => Number(spec.product_id) === Number(productID))
const activityLabel = kind => ({submitted:'客户提交申请',assigned:'分配／改派',confirmed:'确认寄样',followup:'回访记录',assign:'分配／改派',confirm:'确认寄样',submit:'客户提交申请'})[kind] || kind

async function loadList(nextPage = page.value) {
  const generation = ++listGeneration
  loading.value = true
  try {
    const result = await listSamples({q:q.value, status:status.value, page:nextPage, page_size:20, mini_user_id:miniUserID.value || undefined})
    if (generation !== listGeneration) return
    rows.value = result.rows || []; total.value = result.total; isAdmin.value = result.is_admin; page.value = nextPage
  } catch (e) { if (generation === listGeneration) error.value = e.message }
  finally { if (generation === listGeneration) loading.value = false }
}
function populate(result, choices) {
  selected.value = result; options.value = choices; assignee.value = result.assignee_id || 0; contacted.value = false
  draft.value = {revision:result.revision, customer_id:choices.suggested_customer_id || 0, receiver:{...result.receiver}, items:confirmRows(result.items, choices.specs || []), shipping_amount:0, note:'', new_customer:{name:'',customer_type:'',source_id:0,order_type_id:0}}
  customerMode.value = 'existing'; customerSearch.value = ''; followup.value = {status:result.followup_status || 'following',note:''}
  conflicted.value = false
}
async function openDetail(id) {
  const generation = ++detailGeneration; detailLoading.value = true; error.value = ''; notice.value = ''; selected.value = null
  try {
    const [result, choices] = await Promise.all([getSample(id),sampleOptions(id)])
    if (generation === detailGeneration) populate(result, choices)
  } catch(e) { if(generation === detailGeneration) error.value=e.message }
  finally { if(generation === detailGeneration) detailLoading.value=false }
}
async function searchCustomers() {
  const id = selected.value?.id
  if (!id) return
  const generation = ++customerGeneration
  try {
    const result=await sampleOptions(id,customerSearch.value)
    if(selected.value?.id!==id || generation!==customerGeneration) return
    const customers=result.customers || []
    options.value={...options.value,customers}
    if(!customers.some(customer=>customer.id===draft.value.customer_id)) draft.value.customer_id=0
    contacted.value=false
  } catch(e) { if(selected.value?.id===id && generation===customerGeneration) error.value=e.message }
}
async function mutate(action, body, success) {
  if (!selected.value || saving.value || conflicted.value) return
  saving.value=true; error.value=''; notice.value=''
  try {
    const result=await updateSample(selected.value.id,action,{...body,revision:selected.value.revision})
    selected.value=result; assignee.value=result.assignee_id; followup.value={status:result.followup_status || 'following',note:''}; contacted.value=false
    notice.value=success; await loadList()
  } catch(e) { error.value=e.message; conflicted.value=e.status===409 }
  finally { saving.value=false }
}
function assign() { return mutate('assign',{employee_id:Number(assignee.value)},'负责人已保存，已发送站内通知。') }
function confirmOrder() {
  if (!contacted.value || !confirmable.value || !customerReady.value || !selectedItems.value.length) return
  const payload={...draft.value,customer_id:customerMode.value==='existing'?Number(draft.value.customer_id):0,items:selectedItems.value.map(item=>({product_id:item.product_id,bom_spec_id:Number(item.bom_spec_id),qty:Number(item.qty),unit_price:Number(item.unit_price)})),shipping_amount:Number(draft.value.shipping_amount)}
  if(customerMode.value==='existing') delete payload.new_customer
  return mutate('confirm',payload,'已生成关联销售订单，请按原订单流程安排生产和发货。')
}
function saveFollowup() { return mutate('followup',followup.value,'回访记录已保存。') }
function navigate(key, params={}) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view',{detail:{key,params,returnNavigation:{key:'sampleRequests',label:'返回样品申请',params:{request_id:selected.value?.id,mini_user_id:miniUserID.value || undefined}}}}))
}
async function applyParams() { await loadList(1); if(props.viewParams.request_id) await openDetail(Number(props.viewParams.request_id)) }
watch(() => [props.viewParams.request_id, props.viewParams.mini_user_id], applyParams)
onMounted(applyParams)
</script>

<template>
  <section class="samples-page">
    <ProductionReturnLink :source="viewParams.return_navigation" />
    <header class="page-head"><div><p class="eyebrow">客户联系与寄样</p><h1>样品申请与回访</h1><p>{{ isAdmin ? '查看全部申请，分配负责人并持续跟进。' : '处理本人负责的样品工单，记录每一次联系结果。' }}</p></div><div class="actions"><button @click="navigate('sampleRequestsManual')">操作手册</button><button @click="navigate('miniappEmployeeManual')">员工小程序手册</button></div></header>
    <p v-if="miniUserID" class="info">当前仅显示所选登记用户的样品申请。</p>
    <p v-if="error" class="error" role="alert">{{ error }} <button v-if="conflicted && selected" :disabled="saving" @click="openDetail(selected.id)">重新载入工单</button><button v-else-if="!selected" :disabled="loading" @click="applyParams">重试</button></p>
    <p v-if="notice" class="success" role="status">{{ notice }}</p>
    <div class="workspace" :class="{ 'has-detail': selected || detailLoading }">
      <section class="card request-list">
        <form class="filters" @submit.prevent="loadList(1)"><label>查找申请<input v-model="q" placeholder="申请编号、昵称或电话" /></label><label>状态<select v-model="status"><option value="">全部状态</option><optgroup label="处理进度"><option v-for="(label, key) in sampleStatuses" :key="key" :value="key">{{ label }}</option></optgroup><optgroup label="回访进度"><option v-for="(label, key) in followupStatuses" :key="key" :value="key">{{ label }}</option></optgroup></select></label><button :disabled="loading">查询</button></form>
        <div v-if="loading" class="empty" aria-live="polite">正在加载申请…</div>
        <div v-else-if="!rows.length" class="empty">暂无符合条件的样品申请</div>
        <button v-for="row in rows" v-else :key="row.id" class="request" :disabled="saving" :class="{active:selected?.id===row.id}" @click="openDetail(row.id)"><span class="request-top"><strong>{{ row.number }}</strong><span class="badge">{{ sampleStatus(row) }}</span></span><span>{{ row.nickname || row.receiver?.name }} · {{ row.items?.length || 0 }} 件商品</span><span class="muted">{{ row.publication_name }} {{ row.publication_version }}</span><span class="request-top muted"><span>{{ row.assignee_name || '尚未分配' }} · {{ followupStatuses[row.followup_status] || '等待分配' }}</span><span>{{ date(row.created_at) }}</span></span></button>
        <nav class="pagination" aria-label="样品申请翻页"><button :disabled="loading || page<=1" @click="loadList(page-1)">上一页</button><span>{{ page }} / {{ Math.max(1,Math.ceil(total/20)) }} · 共 {{ total }} 条</span><button :disabled="loading || page*20>=total" @click="loadList(page+1)">下一页</button></nav>
      </section>
      <div v-if="detailLoading" class="card empty">正在加载工单…</div>
      <section v-else-if="selected" class="detail">
        <div class="card"><div class="section-head"><div><p class="eyebrow">{{ selected.number }}</p><h2>{{ sampleStatus(selected) }}</h2></div><button :disabled="saving" @click="selected=null">关闭详情</button></div><dl class="facts"><div><dt>登记昵称／已验证电话</dt><dd>{{ selected.nickname }} · {{ selected.verified_phone }}</dd></div><div><dt>申请价格表</dt><dd>{{ selected.publication_name }} · {{ selected.publication_version }}</dd></div><div><dt>申请收件信息</dt><dd>{{ selected.receiver.name }} · {{ selected.receiver.phone }}<br>{{ selected.receiver.address }}</dd></div><div><dt>客户留言</dt><dd>{{ selected.note || '无' }}</dd></div></dl><ul class="products"><li v-for="item in selected.items" :key="item.key">{{ item.name }} <span class="muted">{{ item.code }}</span></li></ul><p class="muted">收件电话可在确认寄样时调整，不会覆盖账号已验证手机号。</p></div>
        <form v-if="isAdmin && !terminal" class="card" @submit.prevent="assign"><h2>{{ selected.assignee_id ? '负责人／改派' : '分配负责人' }}</h2><label>启用的销售或管理员<select v-model.number="assignee" required><option :value="0" disabled>请选择负责人</option><option v-for="employee in options.employees" :key="employee.id" :value="employee.id">{{ employee.name }}</option></select></label><p class="muted">首次分配生成工单并开始 7 天回访计时。改派保留原提醒时间和历史订单责任记录。</p><button class="primary" :disabled="saving || conflicted || !assignee || assignee===selected.assignee_id">{{ saving ? '保存中…' : '保存负责人' }}</button></form>
        <form v-if="confirmable" class="card confirm-form" @submit.prevent="confirmOrder">
          <h2>已联系客户，确认寄样</h2><p class="muted">请先与客户核对以下信息。确认成功后直接生成正式销售订单。</p>
          <fieldset :disabled="saving || conflicted"><legend>客户档案</legend><div class="inline"><label><input v-model="customerMode" type="radio" value="existing"/> 选择已有客户</label><label><input v-model="customerMode" type="radio" value="new"/> 新建客户</label></div>
          <template v-if="customerMode==='existing'"><div class="inline"><input v-model="customerSearch" aria-label="搜索候选客户" placeholder="搜索客户名称或电话" @keydown.enter.prevent="searchCustomers"/><button type="button" @click="searchCustomers">查找</button></div><label>确认客户<select v-model.number="draft.customer_id" required><option :value="0" disabled>请选择客户</option><option v-for="customer in options.customers" :key="customer.id" :value="customer.id">{{ customer.name }}</option></select></label><p class="muted">请确认匹配候选属于本次申请客户。没有匹配时选择新建客户。</p></template>
          <div v-else class="form-grid"><label>客户名称<input v-model.trim="draft.new_customer.name" required maxlength="200"/></label><label>客户类型<select v-model="draft.new_customer.customer_type" required><option value="" disabled>请选择</option><option v-for="type in options.customer_types" :key="type" :value="type">{{ customerTypeLabel(type) }}</option></select></label><label>来源<select v-model.number="draft.new_customer.source_id" required><option :value="0" disabled>请选择</option><option v-for="source in options.sources" :key="source.id" :value="source.id">{{ source.name }}</option></select></label><label>默认订单类型<select v-model.number="draft.new_customer.order_type_id" required><option :value="0" disabled>请选择</option><option v-for="type in options.order_types" :key="type.id" :value="type.id">{{ type.name }}</option></select></label><p class="muted wide">新客户负责人为本工单负责人。建档不会自动开放客户门户和其他订单权限。</p></div>
          </fieldset>
          <fieldset :disabled="saving || conflicted"><legend>本次收件信息</legend><div class="form-grid"><label>收件人<input v-model.trim="draft.receiver.name" required maxlength="100"/></label><label>收件电话<input v-model.trim="draft.receiver.phone" required maxlength="32" type="tel"/></label><label class="wide">完整地址<textarea v-model.trim="draft.receiver.address" required maxlength="500" rows="2"/></label></div></fieldset>
          <fieldset :disabled="saving || conflicted"><legend>商品规格与费用</legend><div v-for="item in draft.items" :key="item.product_id" class="confirm-item"><label class="check"><input v-model="item.included" type="checkbox"/> 寄送 {{ item.name }}</label><fieldset class="item-grid" :disabled="!item.included"><label>已发布规格<select v-model.number="item.bom_spec_id" required><option :value="0" disabled>请选择规格</option><option v-for="spec in specsFor(item.product_id)" :key="spec.id" :value="spec.id">{{ spec.name }} · {{ spec.unit }}</option></select></label><label>数量<input v-model.number="item.qty" type="number" min="1" max="100000" step="1" required/></label><label>单价（元）<input v-model.number="item.unit_price" type="number" min="0" step="0.01" required/></label></fieldset><p v-if="item.included && !specsFor(item.product_id).length" class="error">该商品暂无可用的已发布 BOM 规格，请先在商品创建器配置并发布，再刷新工单。</p></div><div class="form-grid"><label>运费（元）<input v-model.number="draft.shipping_amount" type="number" min="0" step="0.01" required/></label><label>寄样说明<input v-model.trim="draft.note" maxlength="2000"/></label></div></fieldset>
          <div class="total"><span>商品 ¥{{ money(sampleTotal(selectedItems)) }} ＋ 运费 ¥{{ money(draft.shipping_amount) }}</span><strong>订单合计 ¥{{ money(amount) }}</strong></div><label class="check"><input v-model="contacted" type="checkbox" :disabled="saving || conflicted" required/> 已联系客户，确认客户档案、收件信息、规格、数量和全部费用</label><button class="primary" :disabled="saving || conflicted || !contacted || !selectedItems.length || selectedItems.some(item=>!item.bom_spec_id) || !customerReady">{{ saving ? '正在生成订单…' : '确认并生成销售订单' }}</button>
        </form>
        <div v-if="selected.order_id" class="card"><h2>关联销售订单</h2><p><strong>{{ selected.order_no }}</strong> · {{ selected.shipment_status || '待发货' }}</p><p>订单继续按现有生产、库存和发货流程处理。回访结束不会取消订单。</p><button @click="navigate('orders',{highlight_order_id:selected.order_id})">查看关联订单</button><template v-if="selected.confirmation"><h3>确认寄样记录</h3><p>{{ selected.confirmation.receiver.name }} · {{ selected.confirmation.receiver.phone }}<br>{{ selected.confirmation.receiver.address }}</p><ul class="products"><li v-for="item in selected.confirmation.items" :key="item.product_id + ':' + item.bom_spec_id">{{ selected.items.find(row => row.product_id === item.product_id)?.name }} · {{ item.spec_name || '历史规格（见订单）' }} · {{ item.qty }} {{ item.unit }} × ¥{{ money(item.unit_price) }}</li></ul><p>商品 ¥{{ money(sampleTotal(selected.confirmation.items)) }} ＋ 运费 ¥{{ money(selected.confirmation.shipping_amount) }} ＝ 合计 ¥{{ money(sampleTotal(selected.confirmation.items,selected.confirmation.shipping_amount)) }}</p></template></div>
        <div v-if="selected.work_order_id" class="card"><h2>持续回访</h2><p><span class="badge">{{ followupStatuses[selected.followup_status] }}</span> <span v-if="selected.next_reminder_at">下次提醒：{{ date(selected.next_reminder_at) }}</span><span v-else>已停止回访提醒</span></p><form v-if="!terminal" @submit.prevent="saveFollowup"><label>回访状态<select v-model="followup.status" :disabled="saving || conflicted"><option v-for="(label,key) in followupStatuses" :key="key" :value="key">{{ label }}</option></select></label><label>简短说明<textarea v-model.trim="followup.note" required maxlength="2000" rows="3" :disabled="saving || conflicted" placeholder="记录联系结果、客户反馈或放弃原因"/></label><p class="muted">保存有效回访后重新计算 7 天。成交或放弃停止提醒；查看页面、读消息、改派和发货均不重置计时。</p><p v-if="['won','abandoned'].includes(followup.status)" class="info">本次保存将关闭样品跟进；已生成订单仍按原订单流程处理。</p><button class="primary" :disabled="saving || conflicted || !followup.note">保存回访</button></form></div>
        <div class="card"><h2>处理记录</h2><ol class="timeline"><li v-for="activity in selected.activities || []" :key="activity.id"><strong>{{ activityLabel(activity.kind) }} <span v-if="followupStatuses[activity.status]">· {{ followupStatuses[activity.status] }}</span></strong><p>{{ activity.note }}</p><small>{{ activity.actor }} · {{ date(activity.created_at) }}</small></li></ol><p v-if="!selected.activities?.length" class="muted">暂无处理记录</p></div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.samples-page{max-width:1600px;margin:auto;padding:24px;color:#203c30}.page-head,.section-head,.actions,.inline,.request-top,.pagination{display:flex;gap:12px;align-items:center;justify-content:space-between}.page-head{align-items:flex-start;margin-bottom:24px}.page-head p{margin:8px 0;color:#67786e}.eyebrow{font-size:12px;letter-spacing:.06em;color:#718277;margin:0 0 8px}h1{font-size:27px;margin:0}h2{font-size:18px;margin:0 0 16px}h3{font-size:15px}.actions,.inline{justify-content:flex-start;flex-wrap:wrap}.workspace{display:grid;gap:20px}.workspace.has-detail{grid-template-columns:minmax(300px,.8fr) minmax(480px,1.4fr)}.card{background:#fff;border:1px solid #dce5de;border-radius:12px;padding:22px}.request-list{align-self:start}.detail{display:grid;gap:18px;min-width:0}.filters{display:flex;align-items:end;gap:10px;flex-wrap:wrap;margin-bottom:20px}.filters label{flex:1;min-width:130px}.request{width:100%;display:grid;gap:9px;text-align:left;border-radius:8px;padding:16px;margin-bottom:10px;background:#fafcf9;color:#243d30;cursor:pointer;overflow-wrap:anywhere}.request.active{border-color:#2d8059;box-shadow:inset 3px 0 #2d8059;background:#f2f8f1}.request-top{flex-wrap:wrap}.request-top.muted{font-size:12px}.muted,dt,small{color:#718075;line-height:1.6}.badge{display:inline-block;border-radius:6px;background:#eaf2e9;color:#3f6748;font-size:12px;padding:4px 8px}.empty{text-align:center;padding:48px 20px;color:#718075}.pagination{margin-top:20px;font-size:13px;flex-wrap:wrap}.facts{display:grid;grid-template-columns:1fr 1fr;gap:16px;margin:0}.facts dt{font-size:12px}.facts dd{margin:5px 0 0;line-height:1.6;overflow-wrap:anywhere}.products{padding:16px 20px;background:#f5f8f3;border-radius:8px;line-height:1.9}.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:14px}.wide{grid-column:1/-1}label{display:flex;flex-direction:column;gap:7px;font-size:13px;margin-bottom:14px}button,input,select,textarea{font:inherit;border:1px solid #ccd9d0;border-radius:7px;padding:10px 12px;color:#294635;background:white;box-sizing:border-box;max-width:100%;min-width:0}input,select,textarea{width:100%}button{cursor:pointer;white-space:normal}button.primary{background:#256347;border-color:#256347;color:white}button:disabled{opacity:.5;cursor:not-allowed}fieldset{border:0;padding:0;margin:16px 0 24px}legend{font-size:15px;font-weight:600;margin-bottom:14px}.inline label,.check{display:flex;flex-direction:row;align-items:center}.inline input[type=radio],.check input{width:auto}.inline>input{flex:1}.item-grid{display:grid;grid-template-columns:2fr 1fr 1fr;gap:12px;margin-top:12px}.confirm-item{border-bottom:1px solid #e5ebe5;margin-bottom:16px;padding-bottom:4px}.total{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;background:#eef5ec;border-radius:8px;padding:16px;margin-bottom:16px}.total strong{font-size:19px}.error,.success,.info{padding:12px 14px;border-radius:8px;line-height:1.6;overflow-wrap:anywhere}.error{background:#fff1eb;color:#984a34}.success{background:#edf7ef;color:#28663e}.info{background:#f3f5e7;color:#68662d}.timeline{list-style:none;padding:0;margin:0}.timeline li{border-left:2px solid #dce7dc;padding:0 0 22px 16px}.timeline li:last-child{padding-bottom:0}.timeline p{margin:8px 0;white-space:pre-wrap;overflow-wrap:anywhere}@media(max-width:1100px){.workspace.has-detail{grid-template-columns:1fr}.request-list{max-height:650px;overflow:auto}}@media(max-width:650px){.samples-page{padding:12px}.page-head{display:block}.actions{margin-top:16px}.card{padding:16px}.facts,.form-grid{grid-template-columns:1fr}.item-grid{grid-template-columns:1fr 1fr}.item-grid label:first-child{grid-column:1/-1}.filters>button{width:100%}.request-top{align-items:flex-start}.pagination{justify-content:center}}
</style>
