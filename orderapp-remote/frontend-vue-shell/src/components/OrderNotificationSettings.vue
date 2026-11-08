<script setup>
import {ref,onMounted} from 'vue'
import {fetchOrderNotifications,saveOrderNotifications,fetchOrderNotificationTemplates,testOrderNotification,retryOrderNotification,confirmOrderNotification,addNotificationDevice,revokeNotificationDevice} from '../api/message-center.js'
import {notificationEvents,noticeSources,templateFields,statusLabel,canRetry} from '../lib/order-notifications.js'
const cfg=ref({events:[],templates:{}}),devices=ref([]),deliveries=ref([]),templates=ref([]),busy=ref(false),error=ref(''),message=ref(''),ready=ref(false),pair=ref('')
const name=ref('生产通知 Mac'),group=ref(''),testChannel=ref('wechat_service_account'),mini=ref(''),device=ref(0),event=ref('created')
async function load(){const v=await fetchOrderNotifications();cfg.value=v.config;devices.value=v.devices||[];deliveries.value=v.deliveries||[];ready.value=v.official_ready;for(const [key] of notificationEvents) cfg.value.templates[key] ||= {id:'',fields:{}}}
async function run(fn){if(busy.value)return;busy.value=true;error.value='';message.value='';try{await fn()}catch(e){error.value=e.message||'操作失败'}finally{busy.value=false}}
function chooseTemplate(key){const t=templates.value.find(t=>t.template_id===cfg.value.templates[key].id);if(t)cfg.value.templates[key].fields=Object.fromEntries(templateFields(t.content).map(f=>[f,'']))}
async function save(){await saveOrderNotifications(cfg.value);await load();message.value='通知设置已保存。启用后仅发送新变化。'}
async function test(){await testOrderNotification({event:event.value,channel:testChannel.value,mini_user_id:Number(mini.value),device_id:Number(device.value)});await load();message.value='虚拟订单测试已排队，请刷新查看回执并核对微信中的消息。'}
async function pairDevice(){const v=await addNotificationDevice({name:name.value,group_name:group.value});pair.value=v.token;await load();message.value='配对凭据仅显示一次，请在专用 Mac 中保存。'}
async function resolve(d,alreadySent){if(!window.confirm(alreadySent?'已在微信核对这条完整通知确实存在？这将解除暂停，不会补发。':'已核对微信中没有这条通知？确认后会再次发送。'))return;await(alreadySent?confirmOrderNotification(d.id):retryOrderNotification(d.id,true));await load()}
onMounted(()=>run(load))
</script>
<template>
<section class="external-notices">
 <header><div><h3>订单微信通知</h3><p>六类状态变化 · 客户与相关员工服务号 · 公司生产群</p></div><button :disabled="busy" @click="run(load)">刷新记录</button></header>
 <p>服务号接收人须在小程序“我的 → 订单通知”中关注、绑定并开启通知。生产群使用专用 Mac，完成六类实测后才可启用。</p>
 <div class="message" role="status">{{message}}</div><div v-if="error" class="error" role="alert">{{error}}</div>
 <form @submit.prevent="run(save)">
  <div class="channels"><label><input type="checkbox" v-model="cfg.official_enabled" :disabled="!ready">服务号自动发送</label><span v-if="!ready">服务号尚未配置</span><label><input type="checkbox" v-model="cfg.desktop_enabled">生产群自动发送</label><select v-model.number="cfg.desktop_id"><option :value="0">选择生产群设备</option><option v-for="d in devices.filter(d=>d.enabled)" :value="d.id" :key="d.id">{{d.name}} / {{d.group_name}} / {{d.verified?'已完成实测':'未完成实测'}}</option></select></div>
  <button type="button" :disabled="busy||!ready" @click="run(async()=>{templates=(await fetchOrderNotificationTemplates()).templates||[]})">读取服务号模板</button>
  <p>选择订单业务模板，将模板字段对应到订单内容。模板中的全部字段均需配置。</p>
  <article v-for="[key,label] in notificationEvents" :key="key">
   <label><input type="checkbox" v-model="cfg.events" :value="key">{{label}}</label>
   <template v-if="cfg.templates[key]">
    <select v-model="cfg.templates[key].id" @change="chooseTemplate(key)"><option value="">选择服务号业务模板</option><option v-if="cfg.templates[key].id && !templates.some(t=>t.template_id===cfg.templates[key].id)" :value="cfg.templates[key].id">已保存模板</option><option v-for="t in templates" :key="t.template_id" :value="t.template_id" :disabled="!templateFields(t.content).length">{{t.title}}{{!templateFields(t.content).length?'（不适用）':''}}</option></select>
    <label v-for="(_,field) in cfg.templates[key].fields" :key="field">{{field}}<select v-model="cfg.templates[key].fields[field]"><option value="">选择内容</option><option v-for="[value,label] in noticeSources" :key="value" :value="value">{{label}}</option></select></label>
   </template>
  </article>
  <button :disabled="busy" class="primary">保存通知设置</button>
 </form>
 <details><summary>连接专用 Mac 到生产群</summary><p>群名称必须唯一且完全一致。凭据仅用于本设备，撤销后不能再取消息。微信不可读取、未登录或锁屏时会停止发送。</p><form @submit.prevent="run(pairDevice)"><input v-model.trim="name" required placeholder="设备名称" aria-label="设备名称"><input v-model.trim="group" required placeholder="微信群准确名称" aria-label="微信群准确名称"><button :disabled="busy">生成配对凭据</button></form><textarea v-if="pair" :value="pair" readonly aria-label="一次性配对凭据"/><button v-if="pair" @click="pair=''">隐藏凭据</button><ul><li v-for="d in devices" :key="d.id">{{d.name}} · {{d.group_name}} · {{d.enabled?d.health:'已撤销'}} · {{d.last_seen||'尚未连接'}} <button v-if="d.enabled" @click="run(async()=>{await revokeNotificationDevice(d.id);await load()})">撤销设备</button></li></ul></details>
 <form @submit.prevent="run(test)" class="test"><h4>逐项发送测试</h4><p>发送标注“测试”的虚拟订单，不修改业务订单。请选择明确的测试接收人。</p><select v-model="event" aria-label="测试节点"><option v-for="[key,label] in notificationEvents" :key="key" :value="key">{{label}}</option></select><select v-model="testChannel" aria-label="测试渠道"><option value="wechat_service_account">服务号</option><option value="wechat_desktop">生产群</option></select><input v-if="testChannel==='wechat_service_account'" type="number" min="1" v-model="mini" required placeholder="测试小程序账号编号" aria-label="测试小程序账号编号"><select v-else v-model.number="device" required aria-label="测试群设备"><option :value="0">选择测试设备</option><option v-for="d in devices.filter(d=>d.enabled)" :key="d.id" :value="d.id">{{d.name}} / {{d.group_name}}</option></select><button :disabled="busy">发送这项测试</button></form>
 <div class="table-wrap"><table><thead><tr><th>通知</th><th>渠道</th><th>结果</th><th>次数</th><th>说明 / 处理</th></tr></thead><tbody><tr v-for="d in deliveries" :key="d.id"><td>{{d.notice.test?'【测试】':''}}{{d.notice.order_no}}<small>{{d.notice.milestones.map(k=>notificationEvents.find(v=>v[0]===k)?.[1]).join('、')}} · {{d.created_at}}</small></td><td>{{d.channel==='wechat_desktop'?'生产群':'服务号'}}</td><td>{{statusLabel(d.status)}}</td><td>{{d.attempts}}</td><td>{{d.error}}<button v-if="canRetry(d.status)" @click="run(async()=>{await retryOrderNotification(d.id);await load()})">重试</button><template v-if="d.status==='unknown'"><button @click="run(()=>resolve(d,true))">已核对已发送</button><button @click="run(()=>resolve(d,false))">已核对未发送，补发</button></template></td></tr><tr v-if="!deliveries.length"><td colspan="5">暂无发送记录</td></tr></tbody></table></div>
 <details><summary>使用说明与异常处理</summary><p>先在服务号后台配置订单业务模板，再读取模板并保存字段对应。测试每一类消息后，到接收端核对内容及订单跳转。微信已受理表示仍在等待回执，不代表已经送达。取消关注、账号停用或权限变化会停止发送。</p><p>结果不明时，生产群队列会暂停；请按通知号在群中核对，再选择“已发送”或“未发送”。启用前的历史通知不会补发。完整手册：订单微信通知。</p></details>
</section>
</template>
<style scoped>
.external-notices{display:grid;gap:14px;background:#fff;border:1px solid #dde3e8;border-radius:8px;padding:20px;color:#24313d}header,.channels{display:flex;gap:16px;align-items:center;flex-wrap:wrap}header{justify-content:space-between}h3,h4,p{margin:0 0 8px}p,small{color:#677483;font-size:13px;line-height:1.6}form{display:grid;gap:12px}article{display:flex;gap:14px;padding:12px;background:#f6f8fa;align-items:center;flex-wrap:wrap}article>label:first-child{min-width:105px}label{display:inline-flex;gap:8px;align-items:center}input,select,button,textarea{font:inherit;padding:8px;border:1px solid #cdd5df;border-radius:4px;max-width:100%}input[type=checkbox]{width:auto}button{cursor:pointer;background:white}button:disabled{opacity:.5;cursor:default}.primary{background:#1f6feb;color:white;justify-self:start}.error{background:#fee2e2;color:#a51717;padding:10px}.message{color:#176339}.message:empty{display:none}small{display:block}.table-wrap{overflow:auto}table{width:100%;border-collapse:collapse;font-size:13px}td,th{padding:10px;text-align:left;border-bottom:1px solid #edf0f3;min-width:70px}td button{margin:4px}details{padding:12px;background:#f7f9fb}summary{cursor:pointer;margin-bottom:8px}.test{border-top:1px solid #ddd;padding-top:14px}
</style>
