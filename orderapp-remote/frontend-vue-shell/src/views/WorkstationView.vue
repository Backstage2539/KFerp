<template>
  <div class="page workstation-view">

    <ProductionReturnLink :source="viewParams.return_navigation" />
    <section v-if="lastGroupResult" class="panel group-result"><strong>合并入库完成：{{ lastGroupResult.completed_qty }} {{ lastGroupResult.output_unit }}</strong><p v-for="item in lastGroupResult.items" :key="item.job_card.id">{{ item.work_order.work_order_no }} · 入库 {{ item.completion?.output_qty || 0 }} {{ item.completion?.output_unit }} · 单据 {{ item.completion?.stock_entry_ids?.join('、') || '无' }}</p><p>{{ lastGroupResult.remaining_qty > 0 ? `剩余 ${lastGroupResult.remaining_qty} 袋继续待办` : '本组任务已完成' }}</p><button class="secondary" @click="lastGroupResult=null">收起</button></section>
    <section v-if="lastAutomaticResult" class="panel"><strong>{{ lastAutomaticResult.message }} · {{ lastAutomaticResult.output_qty }} {{ lastAutomaticResult.output_unit }}</strong><p>关联单据：{{ lastAutomaticResult.stock_entry_ids?.join('、') || '工序交接记录' }}</p><button class="secondary" @click="lastAutomaticResult=null">收起</button></section>
    <section class="toolbar">
      <div>
        <h2>{{ scope === 'mine' ? '我的今日工位' : '全厂工位视图' }}</h2>
        <p>{{ todayRoster.date || overview.date || '今天' }} · {{ visibleSections.length }} 个工位 · {{ visibleTaskCount }} 个任务</p>
      </div>
      <div class="toolbar-actions">
        <div v-if="canViewAll" class="scope-switch"><button type="button" :disabled="startGroup.locked" :class="{ active: scope === 'mine' }" @click="switchScope('mine')">我的今日工位</button><button type="button" :disabled="startGroup.locked" :class="{ active: scope === 'all' }" @click="switchScope('all')">全厂总览</button></div>
        <label>
          <span>工位</span>
          <select v-model="selectedWorkstation" :disabled="startGroup.locked" @change="load">
            <option value="">全部工位</option>
            <option v-for="section in sections" :key="section.workstation" :value="section.workstation">{{ section.workstation }}</option>
          </select>
        </label>
        <button class="secondary" type="button" @click="showPackagingRemainder = true">包装余料</button>
        <button class="secondary" type="button" @click="openScheduling">查看生产排班</button>
      </div>
    </section>

    <PackagingRemainderPanel v-if="showPackagingRemainder" @close="showPackagingRemainder = false" @posted="load" />
    <div v-if="message" class="notice">{{ message }}</div>
    <div v-if="error" class="error">{{ error }}</div>
    <div v-if="pendingStart" class="panel" role="alert">上次开工结果尚未确认，请由原操作人员重试。 <button type="button" class="secondary" :disabled="Boolean(busyKey)" @click="retryPendingStart">重试本次开工</button></div>
    <section v-if="requestedTaskNotice" class="requested-task-notice" role="status">
      <strong>{{ requestedTaskNotice.operation }} · {{ requestedTaskNotice.work_order_no }} · {{ requestedTaskNotice.status_label }}</strong>
      <p>{{ requestedTaskNotice.message }}</p>
      <button v-if="requestedTaskNotice.disposition === 'unattended'" type="button" class="secondary" @click="openScheduling">去安排今日工位</button>
      <button v-if="requestedTaskNotice.work_order_id" type="button" class="secondary" @click="openExecutionHub(requestedTaskNotice, 'job_card')">查看工单</button>
    </section>

    <section class="station-grid" :class="{ 'single-station-grid': singleStationLayout }">
      <article v-for="section in visibleSections" :key="section.workstation" class="station-panel">
        <div class="station-head">
          <div>
            <h3>{{ section.workstation }}</h3>
            <p>{{ loadStatusLabel(stationLoad(section).load_status) }} · 队列 {{ stationLoad(section).queue_count || section.tasks.length }} · 阻塞 {{ stationLoad(section).blocked_count || 0 }} · 预计 {{ stationLoad(section).estimated_minutes || 0 }} 分钟</p>
            <p class="station-owner" :class="{ unattended: !section.assignment?.employee_id }">今日负责人：{{ section.assignment?.employee_name || '无人值班' }} · {{ rosterSourceLabel(section.assignment) }}</p>
          </div>
          <div class="station-head-actions"><span v-if="section.blockingReason" class="blocker">{{ section.blockingReason }}</span><button v-if="section.assignment?.handover_count" class="handover" type="button" :disabled="busyKey !== ''" @click="handoverStation(section)">接手工位（{{ section.assignment.handover_count }}）</button></div>
        </div>

        <div class="answer-grid">
          <div class="answer-block current">
            <span>当前任务 · 现在做</span>
            <strong>{{ taskTitle(section.currentTask) }}</strong>
            <small>{{ taskMeta(section.currentTask) }}</small>
          </div>
          <div class="answer-block next">
            <span>下一件</span>
            <strong>{{ taskTitle(section.nextTask) }}</strong>
            <small>{{ taskMeta(section.nextTask) }}</small>
          </div>
        </div>

        <div v-if="!section.tasks.length" class="no-task">{{ scope === 'mine' ? '今日负责此工位，暂无任务' : '今日工位暂无任务' }}</div>
        <button v-else-if="!selectedWorkstation" class="enter-station primary" type="button" @click="selectedWorkstation = section.workstation; load()">进入本工位</button>

        <div v-if="selectedWorkstation && issueTasks(section).length" class="batch-issue-toolbar">
          <span>已选 {{ issueSelectionState(issueTasks(section), selectedIssueTasks).count }} 个任务</span>
          <button type="button" class="primary" :disabled="!issueSelectionState(issueTasks(section), selectedIssueTasks).count || loading" @click="openBatchIssue(section)">合并领料</button>
          <small>多选可领任务后，同料汇总、一次领到 WIP</small>
        </div>
        <div v-if="selectedWorkstation && startableTasks(section).length" class="batch-start-toolbar">
          <label><input type="checkbox" aria-label="全选当前工位可开工工单" :checked="startSelectionState(section).checked" :indeterminate="startSelectionState(section).indeterminate" :disabled="startGroup.locked" @change="toggleStartSelection(section)" /> 全选可开工工单</label>
          <span>已选 {{ selectedStartRows(section).length }} 张</span>
          <button type="button" class="secondary" :disabled="!selectedStartRows(section).length || loading || Boolean(busyKey)" @click="openPackagingGroupIssue({tasks:selectedStartRows(section)})">补齐所选物料</button>
          <button type="button" class="primary" :disabled="selectedStartRows(section).length < 2 || loading || Boolean(busyKey) || startGroup.locked" @click="openSelectedStart(section)">合并开工</button>
          <small>不同规格、客户分别扣料和记录；请逐单核对客户工序备注。</small>
        </div>
        <OperationStartGroupForm v-if="startGroup.open && startGroup.key===section.workstation" :key="`group-start-${startGroup.key}`" :task-ids="startGroup.ids" :source-tasks="startGroup.tasks" @close="startGroup.open=false" @posted="groupStarted" @lock-change="startGroup.locked=$event" />
        <div v-if="selectedWorkstation && ungroupedTasks(section).length" class="task-table">
          <div class="task-row header">
            <span><input type="checkbox" aria-label="全选当前工位可领料任务" :checked="issueSelectionState(issueTasks(section), selectedIssueTasks).checked" :indeterminate="issueSelectionState(issueTasks(section), selectedIssueTasks).indeterminate" :disabled="!issueSelectionState(issueTasks(section), selectedIssueTasks).total" @change="selectedIssueTasks = toggleIssueSelection(issueTasks(section), selectedIssueTasks)" /> 任务</span>
            <span>状态</span>
            <span>执行人</span>
            <span>动作</span>
          </div>
          <div v-for="task in ungroupedTasks(section)"
            :key="taskKey(task)"
            class="task-row task-card"
            :class="{ focused: isRequestedTask(task) }"
            :data-task-key="taskKey(task)">
            <div class="task-title">
              <strong><input v-if="canSelectStart(task)" v-model="selectedStartTasks" :disabled="startGroup.locked" type="checkbox" :value="Number(task.job_card_id)" :aria-label="`选择开工工单 ${task.work_order_no}`" /><input v-if="!isSimplifiedTask(task)" v-model="selectedIssueTasks" type="checkbox" :value="Number(task.job_card_id)" :disabled="!workstationCanOpenIssue(task)" :aria-label="`选择任务 ${task.work_order_no} ${task.job_card_id}`" /><button v-if="isInlineBatchTask(task)" type="button" class="task-expand" :aria-expanded="isTaskExpanded(task)" @click="toggleFinishPanel(task)">{{ isTaskExpanded(task) ? '▾' : '▸' }} {{ taskTitle(task) }}</button><template v-else>{{ taskTitle(task) }}</template></strong>
              <small>{{ taskBatchLabel(task) }} · {{ task.work_order_no || '-' }} · P{{ task.priority || 0 }}</small>
              <small v-if="isPackagingTask(task)" class="task-spec">{{task.spec_label || '规格未记录'}}</small>
              <small v-if="isPackagingTask(task) && task.planned_output_inventory_qty != null && Number(task.planned_output_inventory_qty) >= 0 && task.inventory_unit" class="task-quantity">目标 {{ quantityText(packagingTargetQty(task)) }} {{ task.inventory_unit }} · 已完成 {{ quantityText(task.actual_output_qty) }} {{ task.inventory_unit }}</small>
              <small v-for="line in taskDisplayQuantityLines(task)" :key="line" class="task-quantity">{{ line }}</small>
              <small>工序要求：{{ task.process_requirement || '按冻结工艺路线执行' }}</small>
              <div v-if="task.related_orders?.length" class="task-related-orders" aria-label="关联订单与工序备注">
                <div v-for="order in task.related_orders" :key="`${order.order_id}:${order.order_item_id}`"><strong>{{order.order_no}} · {{order.customer_name || '客户未记录'}}</strong><span v-if="order.late_association"> · 后关联</span><span v-if="order.quantity"> · {{quantityText(order.quantity)}} {{order.sales_unit}}</span><p v-if="order.original_order_note">原订单备注：{{order.original_order_note}}</p><p v-if="order.original_item_note">原商品备注：{{order.original_item_note}}</p><p v-for="(note,index) in order.operation_notes || []" :key="index">{{note.operation_name || task.operation}}：{{note.note}}</p></div>
              </div>
              <div v-if="task.material_readiness?.length" class="material-readiness" :class="`material-state-${materialReadinessState(task)}`">
                <div v-if="hasIssuedMaterials(task)" class="issued-materials"><strong>物料已投入</strong><div v-for="material in task.material_readiness" :key="`issued-${material.reservation_id || material.material_id}`"><span>{{ material.material_name }}</span><span>{{ materialQuantity(material, 'issued') }}</span></div></div>
                <details>
                  <summary>{{ materialSummary(task) }}</summary>
                  <div class="material-row material-head"><span>物料名称</span><span>需求</span><span>已投入</span><span>剩余需求</span><span>WIP 可用</span><span>缺口</span></div>
                  <div v-for="material in task.material_readiness" :key="material.reservation_id || material.material_id" class="material-row"><strong>{{ material.material_name }}</strong><span data-label="需求">{{ materialQuantity(material, 'required') }}</span><span data-label="已投入">{{ materialQuantity(material, 'issued') }}</span><span data-label="剩余需求">{{ materialQuantity(material, 'remaining_required') }}</span><span data-label="WIP 可用">{{ materialQuantity(material, 'wip') }}</span><span data-label="缺口" :class="{ shortage: materialHasShortage(material) }">{{ materialQuantity(material, 'shortage') }}</span></div>
                  <button v-if="task.material_readiness.some(materialHasShortage)" type="button" class="secondary" :disabled="Boolean(busyKey)" @click="needsConfirmedStart(task) ? startTask(task, true) : openPackagingGroupIssue({tasks:[task]})">核对缺口并生成补料单</button>
                </details>
              </div>
              <div v-else-if="!task.has_predecessor && materialReadinessState(task) === 'unknown'" class="material-readiness material-state-unknown">用料待核对</div>
            </div>
            <span class="task-state"><span class="pill" :class="statusClass(task)">{{ displayTaskStatus(task) }}</span><small v-if="task.is_blocked && task.blocking_reason && task.blocking_reason !== displayTaskStatus(task)" class="task-blocking-reason">{{ task.blocking_reason }}</small></span>
            <span class="task-staff">{{ task.assigned_to || task.roster_employee_name || '无人值班' }}<small v-if="task.pending_handover">排班已换人，待接手工位</small></span>
            <div class="actions">
              <template v-if="isSimplifiedTask(task)">
                <button v-if="task.status === 'paused'" type="button" class="primary" :disabled="Boolean(busyKey) || loading || isTaskUnattended(task)" @click="handleTaskAction(task, 'resume')">恢复</button>
                <button v-else-if="needsConfirmedStart(task)" type="button" class="primary" :disabled="Boolean(busyKey) || loading || isTaskUnattended(task) || batchIssue.open" @click="startTask(task)">开工</button>
                <button v-else-if="!isInlineBatchTask(task) || isHistoricalBatchTask(task)" type="button" class="primary" :disabled="Boolean(busyKey) || loading || isTaskUnattended(task)" @click="openFinishPanel(task)">工序完成</button><button v-else type="button" class="primary" :disabled="!isFinishPanelForTask(task) || !['running','paused'].includes(task.status) || Boolean(busyKey) || loading || isTaskUnattended(task)" @click="finishForm?.[0]?.completeAll()">完成所有批次</button>
                <small v-if="isTaskUnattended(task)" class="schedule-required">今日无人值班，请先完成排班</small>
                <button type="button" class="secondary" @click="openExecutionHub(task, 'job_card')">查看工单</button>
              </template>
              <template v-else>
              <button type="button" class="secondary" @click="openExecutionHub(task, 'job_card')">查看工单</button>
              <button v-if="workstationCanOpenIssue(task)" type="button" class="primary" @click="openPicking(task)">领料</button>
              <button v-if="task.readiness_label === '待质检'" type="button" class="secondary" @click="openQuality(task)">查看质检</button>
              <details v-if="isFirstOperationTask(task) && task.status === 'running'" class="material-actions"><summary>物料操作</summary><button type="button" @click="openStockAction(task, 'consume')">耗料</button><button type="button" @click="openStockAction(task, 'return')">退料</button></details>
              <button
                v-for="action in workstationVisibleActions(task).filter(action => action !== 'pause')"
                :key="action"
                type="button"
                :class="{ primary: action === 'start' || action === 'complete' }"
                :disabled="Boolean(busyKey) || loading"
                @click="handleTaskAction(task, action)"
              >
                {{ actionLabel(action) }}
              </button>
              </template>
            </div>
            <div
              v-if="taskFeedback.taskKey === taskKey(task) && (taskFeedback.message || taskFeedback.error)"
              class="task-feedback"
              :class="{ error: Boolean(taskFeedback.error) }">
              {{ taskFeedback.error || taskFeedback.message }}
            </div>
            <div v-if="isIssuePanelForTask(task)" class="task-action-panel">
              <div class="section-title-row">
                <div>
                  <div class="section-title">{{ issue.mode === 'material_call' ? '呼叫补料' : '报异常' }}</div>
                  <p class="muted">{{ issue.title }}</p>
                </div>
                <button class="secondary" type="button" @click="closeIssue">关闭</button>
              </div>
              <label>
                <span>{{ issue.mode === 'material_call' ? '补料说明' : '异常原因' }}</span>
                <textarea v-model.trim="issue.note" rows="3"></textarea>
              </label>
              <button class="primary" type="button" @click="submitIssue" :disabled="busyKey !== ''">提交</button>
            </div>

            <ExecutionBatchPreview v-if="startPanel.open && sameTask(startPanel.task,task) && isInlineBatchTask(task)" :job-card-id="Number(task.job_card_id)" />
            <OperationCompletionForm v-if="isFinishPanelForTask(task)" ref="finishForm" :external-bulk="isInlineBatchTask(task)" :can-complete="['running','paused'].includes(task.status) && !isTaskUnattended(task)" :key="task.job_card_id" :job-card-id="Number(task.job_card_id)" :return-params="completionReturnParams(task)" @close="closeFinishPanel" @posted="automaticPosted" @top-up-posted="load" @batch-state="state => setBatchState(task,state)" />
          </div>
        </div>
      </article>
      <div v-if="!visibleSections.length" class="empty-state">
        <strong>{{ emptyState.title }}</strong>
        <span>{{ emptyState.hint }}</span>
      </div>
    </section>
    <WorkstationBatchIssueDrawer v-if="batchIssue.open" :task-ids="batchIssue.ids" :workstation="selectedWorkstation" :return-params="{scope}" :replenishment="batchIssue.replenishment" :start-shortages="batchIssue.startShortages" :start-input="batchIssue.startInput" @close="batchIssue.open = false" @posted="batchIssuePosted" @cancelled="batchIssueCancelled" />
    <ProductionExecutionHubDrawer
      :open="executionHub.open"
      :work-order-id="executionHub.workOrderId"
      :focus="executionHub.focus"
      :view-params="{ ...(props.viewParams || {}), work_order_id: executionHub.workOrderId, job_card_id: executionHub.jobCardId, focus: executionHub.focus }"
      @close="executionHub.open = false"
      @updated="load" />
  </div>
</template>

<script setup>
import OperationCompletionForm from '../components/OperationCompletionForm.vue'
import PackagingRemainderPanel from '../components/PackagingRemainderPanel.vue'
import ExecutionBatchPreview from '../components/ExecutionBatchPreview.vue'
import OperationStartGroupForm from '../components/OperationStartGroupForm.vue'
import WorkstationBatchIssueDrawer from '../components/WorkstationBatchIssueDrawer.vue'
import { issueSelectionState, toggleIssueSelection } from '../lib/workstation-batch-issue.js'
import { issueSessionStorage, readPendingIssue } from '../lib/workstation-issue-recovery.js'
import { directStart, directStartStorage, readPendingStart } from '../lib/workstation-direct-start.js'
import ProductionReturnLink from '../components/ProductionReturnLink.vue'
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { fetchProductionWorkstationOverview, runProductionTaskAction } from '../api/production.js'
import { apiGet, apiSend } from '../api/client'
import { actorHasFullViewAccess } from '../lib/menu-permissions.js'
import ProductionExecutionHubDrawer from '../components/ProductionExecutionHubDrawer.vue'
import {
  productionTaskActionEndpoint,
  productionTaskActionErrorMessage,
  productionTaskActionNeedsRefresh,
  materialReadinessState,
  taskTitle,
  taskQuantityLines,
  isPackagingTask,
  isSimplifiedWorkstationTask,
  workstationCanOpenIssue,
  workstationVisibleActions,
  workstationTaskSections,
  workstationPackagingGroups,
  workstationFocusDisposition,
  workstationTaskRequiresSchedule,
} from '../lib/production-workstation.js'

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
  actor: { type: Object, default: null },
})

const showPackagingRemainder=ref(false)
const startGroup=reactive({open:false,key:'',ids:[],tasks:[],locked:false})
async function groupStarted(_result,submittedIDs=startGroup.ids){selectedStartTasks.value=selectedStartTasks.value.filter(id=>!submittedIDs.includes(id));startGroup.open=false;await load()}
const startPanel=reactive({open:false,task:null,revision:0})
function needsConfirmedStart(task){return task.material_consumption_policy==='bom_start_v1' && ['pending','ready','released'].includes(task.status)}
async function startPosted(){startPanel.open=false;return await load()}
const pendingStart=ref(null)
function syncPendingStart(){try{pendingStart.value=readPendingStart(directStartStorage())}catch(e){error.value=e.message}}
async function startTask(task, issueOnly=false){
 if (busyKey.value || batchIssue.open || isTaskUnattended(task)) return
 busyKey.value=`${task.job_card_id}:start`;error.value='';message.value='';updateTaskFeedback(task)
 try {
  const result=await directStart({jobCardId:Number(task.job_card_id),employeeId:Number(props.actor?.employee_id || 0),storage:directStartStorage(),get:apiGet,post:apiSend,makeID:()=>crypto.randomUUID(),issueOnly,confirmRefresh:p=>window.confirm(`原配方不完整，本次开工将使用 ${p.effective_bom_version_no || '更新的 BOM'}：\n${(p.materials || []).map(m=>`${m.name} ${m.qty} ${m.unit}`).join('\n')}\n确认按以上用料开工？`)})
  if(result.shortages?.length){openStartIssue(task,result.shortages,result.input);updateTaskFeedback(task,{error:'开工用料不足，请先补齐物料。'})}
  else if(result.cancelled){updateTaskFeedback(task,{message:'已取消开工'})}
  else if(issueOnly){updateTaskFeedback(task,{message:'库存已更新，目前没有开工缺口。'});await load()}
  else if(await startPosted()){updateTaskFeedback(task,{message:'开工成功'})}
  else {error.value='开工已成功，但工位状态刷新失败，请重新进入工位核对。';updateTaskFeedback(task,{error:error.value})}
 } catch(e) {error.value=e.message;updateTaskFeedback(task,{error:e.message})}
 finally {syncPendingStart();busyKey.value=''}
}
async function retryPendingStart(){
 if(!pendingStart.value || busyKey.value)return
 const id=pendingStart.value.jobCardId
 busyKey.value=`${id}:start`;error.value=''
 try{await directStart({jobCardId:id,employeeId:Number(props.actor?.employee_id||0),storage:directStartStorage(),get:apiGet,post:apiSend,makeID:()=>crypto.randomUUID(),requirePending:true});if(await load())message.value='开工结果已确认';else error.value='开工已成功，但工位状态刷新失败，请重新进入工位核对。'}
 catch(e){error.value=e.message}
 finally{syncPendingStart();busyKey.value=''}
}
const selectedStartTasks=ref([])
watch(selectedStartTasks,()=>{
  if(!startGroup.open || startGroup.locked)return
  const section=sections.value.find(row=>row.workstation===startGroup.key)
  if(!section)return
  const tasks=selectedStartRows(section)
  startGroup.ids=tasks.map(task=>Number(task.job_card_id));startGroup.tasks=tasks
},{deep:true,flush:'sync'})
function canSelectStart(task){return isPackagingTask(task) && needsConfirmedStart(task) && !isTaskUnattended(task)}
function startableTasks(section){return (section.tasks || []).filter(canSelectStart)}
function selectedStartRows(section){return startableTasks(section).filter(task=>selectedStartTasks.value.includes(Number(task.job_card_id)))}
function startSelectionState(section){const total=startableTasks(section).length,count=selectedStartRows(section).length;return {checked:total>0&&count===total,indeterminate:count>0&&count<total}}
function toggleStartSelection(section){if(startGroup.locked)return;const ids=startableTasks(section).map(task=>Number(task.job_card_id));const selected=new Set(selectedStartTasks.value);const remove=startSelectionState(section).checked;for(const id of ids){if(remove)selected.delete(id);else selected.add(id)}selectedStartTasks.value=[...selected]}
function openSelectedStart(section){if(startGroup.locked)return;const tasks=selectedStartRows(section);if(tasks.length<2)return;completionGroup.open=false;startPanel.open=false;startGroup.key=section.workstation;startGroup.ids=tasks.map(task=>Number(task.job_card_id));startGroup.tasks=tasks;startGroup.open=true}
function completionGroups(section){return packagingGroups(section).filter(group=>group.tasks.every(task=>!needsConfirmedStart(task)))}
function packagingSummary(section){
  const rows=(section.tasks||[]).filter(task=>isPackagingTask(task)&&['pending','ready','released','blocked','running','paused'].includes(task.status)&&!['completed','cancelled'].includes(task.work_order_status))
  const orderTaskCounts=new Map()
  for(const task of rows){const order=task.work_order_id||task.work_order_no||`card:${task.job_card_id}`;orderTaskCounts.set(order,(orderTaskCounts.get(order)||0)+1)}
  const groups=new Map()
  for(const task of rows){
    const order=task.work_order_id||task.work_order_no||`card:${task.job_card_id}`
    const spec=String(task.spec_label||'').trim(),unit=String(task.inventory_unit||'').trim()
    const product=Number(task.product_id)>0?`product:${task.product_id}`:task.packaging_group_key||`order:${order}`
    const key=JSON.stringify([product,spec||`unknown-spec:${order}`,Number(task.bom_spec_id||0),Number(task.bom_variant_id||0),unit||`unknown-unit:${order}`])
    if(!groups.has(key))groups.set(key,{key,product_name:task.product_name||'产品未记录',spec_label:spec||'规格未记录',unit:unit||'单位未记录',remaining_qty:0,unknown_quantity_count:0,orders:new Set(),unscheduled:new Set()})
    const group=groups.get(key)
    group.orders.add(order)
    if(!String(task.planned_start_at||'').trim())group.unscheduled.add(order)
    const target=Number(task.planned_output_inventory_qty),totalInput=Number(task.operation_total_input_qty),taskInput=Number(task.planned_input_qty)
    const planned=Number.isFinite(target)&&target>=0?(totalInput>0&&taskInput>=0?target*taskInput/totalInput:orderTaskCounts.get(order)===1?target:null):null
    if(planned===null || !unit)group.unknown_quantity_count++
    else group.remaining_qty+=Math.max(0,planned-Math.max(0,Number(task.actual_output_qty)||0))
  }
  return [...groups.values()].map(({orders,unscheduled,...row})=>({...row,remaining_qty:Number(row.remaining_qty.toFixed(6)),order_count:orders.size,unscheduled_count:unscheduled.size}))
}
function packagingTargetQty(task){const target=Number(task.planned_output_inventory_qty||0),total=Number(task.operation_total_input_qty||0),input=Number(task.planned_input_qty||0);return total>0&&input>0?target*input/total:target}
function taskDisplayQuantityLines(task){return taskQuantityLines(task).map(line=>isInlineBatchTask(task)?line.replace(/^本批投料 /,'计划总投入 ').replace(/^目标产出 /,'工单目标产出 '):line)}
const selectedIssueTasks = ref([])
const batchIssue = reactive({open: false, ids: [], startShortages: [], startInput: null, recovering: false, replenishment: false})
const completionGroup = reactive({ open: false, key: '', ids: [] })
const lastGroupResult = ref(null)
function ungroupedTasks(section) { return section.tasks }
function isSimplifiedTask(task) { return isSimplifiedWorkstationTask(task) }
function issueTasks(section) { return ungroupedTasks(section).filter(task => !isSimplifiedTask(task)) }
function packagingGroups(section) { return workstationPackagingGroups(section.tasks) }
function openBatchIssueForTasks(tasks) { if (batchIssue.open) return; batchIssue.replenishment=false; batchIssue.recovering = false; batchIssue.startShortages = []; batchIssue.startInput=null; batchIssue.ids = tasks.filter(workstationCanOpenIssue).map(task => Number(task.job_card_id)); batchIssue.open = batchIssue.ids.length > 0 }
function openPackagingGroupIssue(group) {
  if (batchIssue.open) return
  batchIssue.replenishment=true
  // Simplified tasks have no legacy stock_action; use the same group as the source-stock preview.
  const ids = [...new Set((group?.tasks || []).map(task => Number(task.job_card_id)).filter(id => Number.isSafeInteger(id) && id > 0))]
  if (!ids.length) { error.value = '本组没有可补料的有效任务，请刷新工位后重试。'; return }
  error.value = ''
  batchIssue.recovering = false
  batchIssue.startShortages = []
  batchIssue.startInput=null
  batchIssue.ids = ids
  batchIssue.open = true
}
function openStartIssue(task, shortages, input) { if (batchIssue.open) return; batchIssue.replenishment=true; batchIssue.recovering = false; batchIssue.ids = [Number(task.job_card_id)]; batchIssue.startShortages = shortages; batchIssue.startInput = input; batchIssue.open = true }
async function batchIssuePosted(result) { if (batchIssue.startShortages.length && Number(result?.success_count || 0) > 0) startPanel.revision++; await load() }
async function batchIssueCancelled() { if (batchIssue.startShortages.length) startPanel.revision++; await load() }
function openBatchIssue(section) { openBatchIssueForTasks(issueTasks(section).filter(task => selectedIssueTasks.value.includes(Number(task.job_card_id)))) }
function openGroupCompletion(group) { completionGroup.key = group.key; completionGroup.ids = group.tasks.map(task => Number(task.job_card_id)); completionGroup.open = true; batchIssue.open = false }
async function groupCompleted(result) { lastGroupResult.value = result; completionGroup.open = false; message.value = `已完成并入库 ${result.completed_qty} ${result.output_unit}`; await load() }
function quantityText(value) { return Number(value || 0).toLocaleString('zh-CN', { maximumFractionDigits: 3 }) }
function warehouseLabel(code) { const key = String(code || ''); return warehouseNames.value.get(key) || ({ finished_goods: '成品仓', packaging: '包材仓', raw_materials: '原料仓', wip: 'WIP在制仓' })[key] || (key ? `仓库名待加载（${key}）` : '待确认') }
async function loadWarehouseNames() { try { const data = await apiGet('/api/stock/warehouses'); const rows = Array.isArray(data) ? data : (data.rows || data.warehouses || []); warehouseNames.value = new Map(rows.filter(row => row?.code).map(row => [String(row.code), String(row.name || row.code)])) } catch { warehouseNames.value = new Map() } }
const loading = ref(false)
const busyKey = ref('')
const error = ref('')
const message = ref('')
const selectedWorkstationValue = ref('')
const selectedWorkstation = computed({ get: () => selectedWorkstationValue.value, set: value => { selectedWorkstationValue.value = value } })
const overview = ref({ tasks: [] })
const todayRoster = ref({ assignments: [] })
const warehouseNames = ref(new Map())
const canViewAll = computed(() => actorHasFullViewAccess(props.actor))
const scope = ref(String(props.viewParams?.scope || 'mine') === 'all' && canViewAll.value ? 'all' : 'mine')
const issue = reactive({ open: false, mode: '', title: '', task: null, note: '' })
const executionHub = reactive({ open: false, workOrderId: 0, jobCardId: 0, focus: '' })
const requestedJobCardID = computed(() => Number(props.viewParams?.job_card_id || 0))
const taskFeedback = reactive({ taskKey: '', message: '', error: '' })
const finishPanel = reactive({open:false,task:null})
const finishForm = ref(null)
const batchStates = reactive({})
function isInlineBatchTask(task) { return task?.material_consumption_policy === 'bom_start_v1' && task?.reporting_type === 'batch' }
function isHistoricalBatchTask(task) { return Boolean(batchStates[task.job_card_id]?.reason?.includes('历史汇总')) }
function setBatchState(task,state) { batchStates[task.job_card_id]=state }
const requestedTaskNotice = ref(null)
let overviewLoadRevision = 0

const tasks = computed(() => overview.value.tasks || [])
async function convertedOldOrder(){closeFinishPanel();message.value='核对已保存，后续工序可直接完成';await load()}
const lastAutomaticResult=ref(null)
async function automaticPosted(data){message.value=data.completion?.message || '完成记录已保存';lastAutomaticResult.value={...data.completion};if(!data.batchPartial)finishPanel.open=false;await load()}
watch(selectedWorkstation, () => { selectedIssueTasks.value = []; if (!batchIssue.recovering) batchIssue.open = false })
watch(tasks, rows => { const available = new Set(rows.filter(workstationCanOpenIssue).map(task => Number(task.job_card_id))); selectedIssueTasks.value = selectedIssueTasks.value.filter(id => available.has(id)) })
const sections = computed(() => {
  const taskSections = workstationTaskSections(tasks.value)
  const byName = new Map(taskSections.map(section => [section.workstation, section]))
  for (const assignment of todayRoster.value.assignments || []) {
    const section = byName.get(assignment.workstation) || { workstation: assignment.workstation, tasks: [], currentTask: null, nextTask: null, blockingReason: '' }
    section.assignment = assignment
    byName.set(assignment.workstation, section)
  }
  for (const section of byName.values()) if (!section.assignment) section.assignment = assignmentForSection(section)
  return Array.from(byName.values()).sort((a, b) => a.workstation.localeCompare(b.workstation, 'zh-Hans-CN'))
})
const workstationLoad = computed(() => overview.value.workstation_load || [])
const visibleSections = computed(() => selectedWorkstation.value ? sections.value.filter((section) => section.workstation === selectedWorkstation.value) : sections.value)
const visibleTaskCount = computed(() => visibleSections.value.reduce((total, section) => total + section.tasks.length, 0))
const singleStationLayout = computed(() => visibleSections.value.length === 1)
const emptyState = computed(() => {
  if (scope.value !== 'mine') return { title: '暂无工位任务', hint: '当前筛选范围内没有待执行或执行中的任务。' }
  if (Number(todayRoster.value.roster_version || 0) === 0) return { title: '本周排班尚未保存', hint: '请先完成本周生产排班，再查看今天负责的工位。' }
  if (todayRoster.value.attendance === 'off') return { title: '今天休息', hint: '今天不参与工位自动安排。' }
  if (todayRoster.value.attendance === 'unplanned') return { title: '今天尚未排班', hint: '请联系排班人员确认今天上班或休息。' }
  return { title: '今天已上班，暂未负责工位', hint: '如需临时换岗，请由排班人员调整当天工位负责人。' }
})

function loadStatusLabel(value) {
  return ({
    overloaded: '超负荷',
    blocked: '有待办',
    busy: '繁忙',
    normal: '正常',
    idle: '空闲',
  })[String(value || '').toLowerCase()] || '正常'
}

function taskKey(task) {
  return `${task.job_card_id || 0}:${task.work_order_id || 0}`
}

function isRequestedTask(task) {
  return requestedJobCardID.value > 0 && Number(task?.job_card_id || 0) === requestedJobCardID.value
}

const taskStatusLabels = { pending: '待执行', ready: '可开始', released: '待执行', running: '执行中', paused: '已暂停', blocked: '待处理', completed: '已完成', cancelled: '已取消' }
async function focusRequestedTask(options = {}) {
  const focus = String(props.viewParams?.focus || '')
  requestedTaskNotice.value = null
  if (focus !== 'workstation_task' || requestedJobCardID.value <= 0) return false
  const matchedTask = tasks.value.find((task) => Number(task?.job_card_id || 0) === requestedJobCardID.value)
  if (matchedTask) {
    selectedWorkstation.value = matchedTask.workstation
    const rosterStatus = isTaskUnattended(matchedTask) ? 'unattended' : matchedTask.roster_status
    const disposition = workstationFocusDisposition({ task: matchedTask, rosterStatus, canViewAll: canViewAll.value })
    const workOrderNo = matchedTask.work_order_no || `工单 #${matchedTask.work_order_id || props.viewParams?.work_order_id || ''}`
    const base = { ...matchedTask, work_order_no: workOrderNo, status_label: displayTaskStatus(matchedTask), disposition }
    if (disposition === 'record') {
      openExecutionHub(matchedTask, 'job_card')
      requestedTaskNotice.value = { ...base, message: '该工序已结束，已打开工单记录。' }
    } else if (disposition === 'unattended') {
      requestedTaskNotice.value = { ...base, message: `今日该工位无人值班，请先完成排班。目标工序当前状态：${base.status_label}。` }
    } else if (options.scopeRetry === true) {
      message.value = '已从全厂视图加载目标任务'
      requestedTaskNotice.value = { ...base, message: `目标工序当前状态：${base.status_label}。` }
    }
    window.requestAnimationFrame(() => {
      document.querySelector(`[data-task-key="${taskKey(matchedTask)}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
    })
    return true
  }
  if (canViewAll.value && scope.value === 'mine' && options.scopeRetry !== true) {
    scope.value = 'all'
    await load({ focusRequested: true, scopeRetry: true })
    return false
  }
  const workOrderID = Number(props.viewParams?.work_order_id || 0)
  if (workOrderID <= 0) {
    error.value = '缺少工单编号，无法确认目标工序状态'
    return false
  }
  try {
    const detail = await apiGet(`/api/produce/work-orders/${workOrderID}`)
    if (options.loadRevision !== overviewLoadRevision || requestedJobCardID.value !== Number(props.viewParams?.job_card_id || 0)) return false
    const operations = detail?.execution_hub?.operation_progress || []
    const operation = operations.find(row => Number(row.job_card_id || 0) === requestedJobCardID.value)
    if (!operation) {
      error.value = '工单中没有找到这个工序卡，请核对工单链接'
      return false
    }
    const rosterStatus = isTaskUnattended(operation) ? 'unattended' : operation.roster_status
    const disposition = workstationFocusDisposition({ task: operation, rosterStatus, canViewAll: canViewAll.value })
    const workOrderNo = detail?.execution_hub?.header?.work_order_no || `工单 #${workOrderID}`
    const base = { ...operation, work_order_id: workOrderID, work_order_no: workOrderNo, status_label: taskStatusLabels[operation.status] || operation.status || '状态未知', disposition }
    if (disposition === 'record') {
      executionHub.workOrderId = workOrderID
      executionHub.jobCardId = requestedJobCardID.value
      executionHub.focus = 'job_card'
      executionHub.open = true
      requestedTaskNotice.value = { ...base, message: '该工序已结束，已打开只读工单记录。' }
    } else if (disposition === 'unattended') {
      requestedTaskNotice.value = { ...base, message: '今日该工位无人值班，请先完成排班；排班完成后刷新即可继续。' }
    } else if (disposition === 'outside_scope') {
      requestedTaskNotice.value = { ...base, message: '工序仍未结束，但当前账号无权查看该工位任务。请联系排班负责人或使用有查看权限的账号。' }
    } else if (disposition === 'all_scope') {
      selectedWorkstation.value = String(operation.workstation || selectedWorkstation.value || '')
      requestedTaskNotice.value = { ...base, message: `目标工序当前状态：${base.status_label}。已尝试全厂视图，但待执行列表未返回该卡，请刷新工单状态。` }
    } else {
      requestedTaskNotice.value = { ...base, message: `目标工序当前状态：${base.status_label}。请刷新后核对。` }
    }
    return false
  } catch (err) {
    const message = String(err?.message || '')
    if (/403|permission|forbidden|权限/i.test(message)) error.value = '当前账号无权查看该工单状态，请联系管理员或排班负责人'
    else if (/404|not found|不存在/i.test(message)) error.value = '工单不存在或已删除，请检查工单链接'
    else error.value = message || '读取工单状态失败，请刷新后重试'
    return false
  }
}

function updateTaskFeedback(task, { message: nextMessage = '', error: nextError = '' } = {}) {
  taskFeedback.taskKey = taskKey(task)
  taskFeedback.message = nextMessage
  taskFeedback.error = nextError
}

function taskMeta(task) {
  if (!task) return '-'
  return `${taskQuantityLines(task).join(' · ')} · ${task.work_order_no || '-'} · ${task.next_handler || task.assigned_to || '-'} · ${task.planned_start_at || '未排时间'}`
}

function taskBatchLabel(task) {
  if (Number(task?.planned_batch_count || 0) > 1) return `工位安排 · 共 ${Number(task.planned_batch_count)} 批`
  return `第 ${Number(task?.batch_index || 1)} 批 / 共 ${Number(task?.batch_count || 1)} 批`
}

function hasIssuedMaterials(task) { return (task?.material_readiness || []).some(row => Number(row.issued_g || 0) > 0 || Number(row.issued_units || 0) > 0) }

function materialHasShortage(material) {
  return Number(material?.shortage_g || 0) > 0 || Number(material?.shortage_units || 0) > 0
}

function materialSummary(task) {
  if (materialReadinessState(task) === 'unknown') return '用料待核对'
  const rows = task?.material_readiness || []
  const shortages = rows.filter(materialHasShortage).length
  return shortages ? `缺料 ${shortages} 项，展开查看` : hasIssuedMaterials(task) ? '查看用料明细' : `本次用料 ${rows.length} 项，WIP 已齐套`
}

function materialQuantity(material, kind) {
  const prefix = kind === 'wip' ? 'wip_available' : kind
  const g = Number(material?.[`${prefix}_g`] || 0)
  const units = Number(material?.[`${prefix}_units`] || 0)
  const unit = String(material?.unit || '').toLowerCase()
  if (['kg','千克','公斤'].includes(unit)) return `${(g / 1000).toLocaleString('zh-CN', { maximumFractionDigits: 9 })} kg`
  if (g !== 0 || ['g','克'].includes(unit)) return `${g.toLocaleString('zh-CN', { maximumFractionDigits: 9 })} g`
  return `${units.toLocaleString('zh-CN', { maximumFractionDigits: 9 })}${material?.unit || '件'}`
}

function nextHandler(section) {
  return section.tasks.find((task) => task.blocking_reason)?.next_handler || '现场主管'
}

function stationLoad(section) {
  return workstationLoad.value.find((row) => row.workstation === section.workstation) || {}
}

function statusClass(task) {
  if (displayTaskStatus(task) === '异常') return 'danger'
  if (displayTaskStatus(task) === '执行中') return 'running'
  return ''
}

function displayTaskStatus(task) {
  const status = String(task?.status || '').trim().toLowerCase()
  if (status === 'paused') return '已暂停'
  if (status === 'running') return '执行中'
  return task?.status_label || taskStatusLabels[status] || task?.status || '-'
}

function actionLabel(action) {
  return {
    start: '开始本任务',
    pause: '暂停',
    resume: '继续',
    complete: '完成本工序',
    report_exception: '报异常',
    material_call: '呼叫补料',
  }[action] || action
}

function openProductionHelp() { window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'productionManual', returnNavigation: { key: 'workstationView', params: { ...props.viewParams }, label: '返回工位视图' } } })) }
function openScheduling() { window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'productionSchedule', params: { work_center: selectedWorkstation.value }, returnNavigation: { key: 'workstationView', params: { ...props.viewParams }, label: '返回工位视图' } } })) }

function completionReturnParams(task) {
  return {
    ...props.viewParams,
    scope: scope.value,
    workstation: task.workstation,
    workstation_id: Number(task.workstation_id || 0),
    station_id: Number(task.workstation_id || 0),
    work_order_id: Number(task.work_order_id || 0),
    job_card_id: Number(task.job_card_id || 0),
    focus: 'workstation_task',
  }
}

function assignmentForSection(section) { const task = section.tasks[0]; return task ? { workstation_id: task.workstation_id, workstation: section.workstation, employee_id: task.roster_employee_id, employee_name: task.roster_employee_name, source: task.roster_source, unattended: task.roster_status === 'unattended', override_invalid: task.roster_status === 'invalid_override', handover_count: section.tasks.filter(row => row.pending_handover).length } : null }
function rosterAssignmentForTask(task) { return (todayRoster.value.assignments || []).find(row => (Number(task?.workstation_id || 0) > 0 && Number(row.workstation_id || 0) === Number(task.workstation_id)) || (task?.workstation && row.workstation === task.workstation)) || null }
function isTaskUnattended(task) {
  return workstationTaskRequiresSchedule(task, rosterAssignmentForTask(task))
}
function rosterSourceLabel(row) { if (!row) return '尚未排班'; if (row.override_invalid) return '临时调整已失效'; if (row.unattended) return row.reason || '请先排班'; return ({ override: '临时调整', primary: '主负责人', backup: '替补接班' })[row.source] || '自动安排' }
async function switchScope(next) { scope.value = next; selectedWorkstation.value = ''; await load() }
async function handoverStation(section) { if (!section.assignment?.employee_id) return; busyKey.value = `handover:${section.assignment.workstation_id}`; error.value = ''; try { const result = await apiSend('/api/production-roster/handover', { body: { workstation_id: Number(section.assignment.workstation_id), work_date: todayRoster.value.date, employee_id: Number(section.assignment.employee_id), expected_version: Number(todayRoster.value.roster_version || 0), request_id: crypto.randomUUID() } }); message.value = `已接手 ${section.workstation}，共 ${result.job_card_ids?.length || 0} 项执行中任务`; await load() } catch (err) { error.value = err.message || '工位交接失败' } finally { busyKey.value = '' } }

function openPicking(task) {
  openStockAction(task, 'issue')
}

function isFirstOperationTask(task) {
  const sequences = tasks.value.filter((row) => Number(row.work_order_id) === Number(task.work_order_id)).map((row) => Number(row.sequence_no || 0)).filter((value) => value > 0)
  return sequences.length > 0 && Number(task.sequence_no || 0) === Math.min(...sequences)
}

function openStockAction(task, action) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'stockOperations', params: { tab: 'stockEntries', action, return_source: 'workstation_task', work_order_id: task.work_order_id, work_order_no: task.work_order_no, job_card_id: task.job_card_id }, returnNavigation: { key: 'workstationView', params: { scope: scope.value, workstation: selectedWorkstation.value || task.workstation, work_order_id: task.work_order_id, job_card_id: task.job_card_id, focus: 'workstation_task' }, label: '返回工位' } } }))
}

function openQuality(task) {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', { detail: { key: 'qualityInspections', params: { work_order_id: task.work_order_id, job_card_id: task.job_card_id, reference_no: task.work_order_no } } }))
}

function sameTask(a, b) {
  return Boolean(a && b) && taskKey(a) === taskKey(b)
}

function isIssuePanelForTask(task) {
  return issue.open && sameTask(issue.task, task)
}

function isFinishPanelForTask(task) {
  return finishPanel.open && sameTask(finishPanel.task, task)
}

function closeIssue() {
  issue.open = false
  issue.task = null
}

function closeFinishPanel() {
  finishPanel.open = false
  finishPanel.task = null
}

function openIssue(task, mode) {
  finishPanel.open = false
  finishPanel.task = null
  issue.open = true
  issue.mode = mode
  issue.task = task
  issue.title = `${taskTitle(task)} · ${task.work_order_no || ''}`
  issue.note = mode === 'material_call' ? '' : (task.blocking_reason || '')
}

function isTaskExpanded(task){return isFinishPanelForTask(task) || (startPanel.open && sameTask(startPanel.task,task))}
function toggleFinishPanel(task) { if(needsConfirmedStart(task)){if(startPanel.open&&sameTask(startPanel.task,task))startPanel.open=false;else {finishPanel.open=false;startPanel.task=task;startPanel.open=true};return};if (isFinishPanelForTask(task)) closeFinishPanel(); else openFinishPanel(task) }

function openFinishPanel(task) {
  issue.open = false
  issue.task = null
  finishPanel.open = true
  finishPanel.task = task

}

function openExecutionHub(task, focus = 'job_card') {
  const id = Number(task?.work_order_id || 0)
  if (!id) return
  executionHub.workOrderId = id
  executionHub.jobCardId = Number(task?.job_card_id || 0)
  executionHub.focus = focus
  executionHub.open = true
}

async function handleTaskAction(task, action) {
 if(action==='start' && needsConfirmedStart(task)){await startTask(task);return}
  if (action === 'report_exception' || action === 'material_call') {
    openIssue(task, action)
    return
  }
  if (action === 'complete') {
    openFinishPanel(task)
    return
  }
  const endpoint = productionTaskActionEndpoint(task, action)
  if (!endpoint) return
  busyKey.value = `${task.job_card_id}:${action}`
  error.value = ''
  message.value = ''
  updateTaskFeedback(task)
  try {
    await runProductionTaskAction(endpoint, {})
    const refreshed = await load()
    if (!refreshed) {
      const explanation = `${actionLabel(action)}已提交，但状态刷新失败，请手动刷新`
      error.value = explanation
      updateTaskFeedback(task, { error: explanation })
      return
    }
    message.value = `${actionLabel(action)}成功`
    updateTaskFeedback(task, { message: `${actionLabel(action)}成功，状态已刷新` })
  } catch (err) {
    const explanation = productionTaskActionErrorMessage(err, action)
    error.value = explanation
    updateTaskFeedback(task, { error: explanation })
    if (productionTaskActionNeedsRefresh(err)) await load()
  } finally {
    busyKey.value = ''
  }
}

async function submitIssue() {
  const task = issue.task
  const mode = issue.mode
  const endpoint = productionTaskActionEndpoint(task, mode)
  if (!endpoint) return
  busyKey.value = `${task.job_card_id}:${mode}`
  error.value = ''
  message.value = ''
  updateTaskFeedback(task)
  try {
    const payload = mode === 'material_call' ? { note: issue.note } : { exception_reason: issue.note }
    await runProductionTaskAction(endpoint, payload)
    closeIssue()
    const submittedMessage = mode === 'material_call' ? '呼叫补料' : '上报异常'
    const refreshed = await load()
    if (!refreshed) {
      const explanation = `${submittedMessage}已提交，但状态刷新失败，请手动刷新`
      error.value = explanation
      updateTaskFeedback(task, { error: explanation })
      return
    }
    message.value = `${submittedMessage}成功`
    updateTaskFeedback(task, { message: `${submittedMessage}成功，状态已刷新` })
  } catch (err) {
    const explanation = productionTaskActionErrorMessage(err, mode)
    error.value = explanation
    updateTaskFeedback(task, { error: explanation })
  } finally {
    busyKey.value = ''
  }
}

async function load(options = {}) {
  const revision = ++overviewLoadRevision
  loading.value = true
  if (options?.focusRequested === true) error.value = ''
  try {
    const [overviewData, rosterData] = await Promise.all([fetchProductionWorkstationOverview({ limit: 500, scope: scope.value }), apiGet(`/api/production-roster/today?scope=${scope.value}`)])
    if (revision !== overviewLoadRevision) return false
    overview.value = overviewData
    todayRoster.value = rosterData
    if (selectedWorkstation.value && !sections.value.some((section) => section.workstation === selectedWorkstation.value)) {
      selectedWorkstation.value = ''
    }
    if (options?.focusRequested === true || String(props.viewParams?.focus || '') === 'workstation_task') await focusRequestedTask({ ...options, loadRevision: revision })
    return true
  } catch (err) {
    if (revision === overviewLoadRevision) error.value = err.message || '加载失败'
    return false
  } finally {
    if (revision === overviewLoadRevision) loading.value = false
  }
}

watch(() => [props.viewParams?.focus, props.viewParams?.work_order_id, props.viewParams?.job_card_id, props.viewParams?.workstation, props.viewParams?.workstation_id, props.viewParams?.station_id, props.viewParams?.scope], async () => {
  const requestedScope = String(props.viewParams?.scope || '')
  if (requestedScope === 'mine' || (requestedScope === 'all' && canViewAll.value)) scope.value = requestedScope
  selectedWorkstation.value = String(props.viewParams?.workstation || tasks.value.find(row => Number(row.workstation_id || 0) === Number(props.viewParams?.workstation_id || props.viewParams?.station_id || 0))?.workstation || '')
  executionHub.open = false
  executionHub.workOrderId = 0
  executionHub.jobCardId = 0
  requestedTaskNotice.value = null
  if (String(props.viewParams?.focus || '') === 'workstation_task') await load({ focusRequested: true })
}, { flush: 'post' })

async function refocusWorkstationTask(event) {
  if (Number(event?.detail?.job_card_id || 0) <= 0) return
  await nextTick()
  executionHub.open = false
  await load({ focusRequested: true })
}
onBeforeUnmount(() => window.removeEventListener('kferp:focus-workstation-task', refocusWorkstationTask))

onMounted(async () => {
  syncPendingStart()
  window.addEventListener('kferp:focus-workstation-task', refocusWorkstationTask)
  try {
    const pending = readPendingIssue(issueSessionStorage())
    if (pending) { batchIssue.ids = pending.job_card_ids; batchIssue.startShortages = []; batchIssue.recovering = true; batchIssue.open = true }
  } catch (err) { error.value = err.message }
  void loadWarehouseNames()
  selectedWorkstation.value = String(props.viewParams?.workstation || '')
  await load({ focusRequested: true })
  const focus = String(props.viewParams?.focus || '')
  if (focus === 'workstation_task') return
  const id = Number(props.viewParams?.work_order_id || 0)
  if (!id) return
  executionHub.workOrderId = id
  executionHub.jobCardId = Number(props.viewParams?.job_card_id || 0)
  executionHub.focus = focus || (executionHub.jobCardId ? 'job_card' : 'summary')
  executionHub.open = true
})
</script>

<style scoped>
.batch-start-toolbar{display:flex;gap:12px;align-items:center;flex-wrap:wrap;padding:14px 0}.batch-start-toolbar label{display:flex;gap:7px;align-items:center}.batch-start-toolbar input{width:auto}.batch-start-toolbar small{width:100%;color:#69776f}.task-related-orders{margin-top:8px;padding:8px 10px;border-left:3px solid #c7ddce;overflow-wrap:anywhere;font-size:13px}.task-related-orders p{margin:4px 0;color:#58675f}.task-spec{font-weight:600}.packaging-completion-actions{margin:12px 0}.packaging-groups{display:grid;gap:16px;margin:18px 0}.packaging-group-card{border:1px solid #d7e2da;border-radius:12px;padding:18px;background:#fff}.packaging-group-head{display:flex;justify-content:space-between;align-items:flex-start;gap:14px}.packaging-group-head h4{margin:0 0 6px;font-size:18px;color:#203b2c}.packaging-group-head p,.group-source-orders{color:#69776f}.packaging-group-actions{display:flex;gap:8px;flex-wrap:wrap}.group-material-table{margin-top:14px}.group-material-table .material-row{grid-template-columns:minmax(130px,1fr) repeat(4,minmax(74px,.55fr))}.group-source-orders{margin-top:12px}.group-source-orders summary{cursor:pointer}.group-source-orders>div{padding:7px 0}.text-button{border:0;background:none;color:#267349;text-decoration:underline;cursor:pointer}.group-result{padding:16px}.group-result p{margin:.3rem 0}
.batch-issue-toolbar{display:flex;align-items:center;gap:14px;flex-wrap:wrap;margin:18px 0 12px}.batch-issue-toolbar small{color:#6c786f}.task-table input[type=checkbox]{width:16px;height:16px;vertical-align:middle;margin:0 8px 0 0;accent-color:#315740}

.page { padding: 20px; color: #252525; }
.toolbar, .panel, .station-panel {
  border: 1px solid #e2ded7;
  border-radius: 8px;
  background: #fff;
}
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 16px;
  margin-bottom: 14px;
}
h2, h3 { margin: 0; line-height: 1.25; letter-spacing: 0; }
h2 { font-size: 22px; }
h3 { font-size: 18px; }
p { margin: 4px 0 0; color: #666; }
button {
  min-height: 34px;
  border: 1px solid #cfc8bf;
  border-radius: 8px;
  background: #fff;
  padding: 6px 10px;
  color: #222;
  cursor: pointer;
  font: inherit;
}
button.primary { background: #2f8f5b; border-color: #2f8f5b; color: #fff; }
button.secondary { background: #f8f7f5; }
button:disabled { opacity: .55; cursor: not-allowed; }
.toolbar-actions {
  display: flex;
  align-items: end;
  gap: 10px;
  flex-wrap: wrap;
}
.scope-switch{display:flex;border:1px solid #cfdad3;border-radius:8px;overflow:hidden}.scope-switch button{border:0;border-radius:0}.scope-switch button.active{background:#e9f6ed;color:#217546;font-weight:700}.station-owner{color:#26814d!important;font-weight:700}.station-owner.unattended{color:#b36f19!important}.station-head-actions{display:flex;align-items:flex-start;gap:7px;flex-wrap:wrap;justify-content:flex-end}.handover{background:#fff5e5;color:#9a641d;border-color:#e8c890}.no-task{padding:22px;border:1px dashed #cfded4;border-radius:8px;background:#f7fbf8;color:#5e7967;text-align:center}
label { display: grid; gap: 5px; color: #555; font-size: 13px; }
select, input, textarea {
  min-height: 34px;
  border: 1px solid #cfc8bf;
  border-radius: 8px;
  padding: 6px 8px;
  background: #fff;
  font: inherit;
}
textarea { resize: vertical; }
.notice, .error {
  padding: 10px 12px;
  border-radius: 8px;
  margin-bottom: 12px;
}
.notice { border: 1px solid #b7dfc4; background: #effaf2; color: #175c2f; }
.error { border: 1px solid #efb9b9; background: #fff2f2; color: #9d2424; }
.station-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
}
.station-grid.single-station-grid { grid-template-columns: minmax(0, 1fr); }
.station-panel { padding: 14px; min-width: 0; }
.station-head {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.blocker {
  align-self: start;
  border: 1px solid #efb9b9;
  border-radius: 999px;
  padding: 4px 8px;
  background: #fff2f2;
  color: #9d2424;
  font-size: 12px;
}
.answer-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-bottom: 12px;
}
.answer-block {
  display: grid;
  gap: 5px;
  min-height: 94px;
  border: 1px solid #ebe7df;
  border-radius: 8px;
  padding: 10px;
}
.answer-block span { color: #777; font-size: 12px; }
.answer-block strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.answer-block small { color: #666; line-height: 1.35; }
    .answer-block.blocked:not(.empty) { border-color: #efb9b9; background: #fffafa; }
    .enter-station { width: 100%; margin-bottom: 12px; }
    .task-table {
  display: grid;
  gap: 16px;
  overflow-x: auto;
  overflow-y: visible;
  -webkit-overflow-scrolling: touch;
  overscroll-behavior-inline: contain;
}
.task-row {
  display: grid;
  grid-template-columns: minmax(180px, 1.4fr) 80px minmax(155px, .7fr) minmax(180px, 1.2fr);
  min-width: 610px;
  gap: 10px;
  align-items: center;
  padding: 10px;
  border: 1px solid #d9e4dc;
  border-radius: 10px;
  background: #fff;
}
.task-state,.task-staff{display:grid;gap:5px;align-content:start;min-width:0}.task-state small,.task-staff small{display:block;font-size:12px;color:#657269;line-height:1.6}
.task-row.header {
  border-top: 0;
  background: #faf9f7;
  color: #666;
  font-size: 13px;
  font-weight: 700;
}
.task-row.focused {
  background: #fffbea;
  box-shadow: inset 3px 0 0 #d97706;
}
.task-expand{border:0;background:transparent;padding:0;color:inherit;font:inherit;cursor:pointer;text-align:left}.task-title { display: grid; gap: 3px; min-width: 0; }
.task-title strong, .task-title small { overflow-wrap: anywhere; white-space: normal; }
.task-title small { color: #777; }
.task-title .task-quantity { color:#294d39;font-weight:650; }
.task-title>details{margin-top:5px}.material-readiness summary{cursor:pointer;color:#a85a08;font-size:12px}.material-row{display:grid;grid-template-columns:minmax(130px,1fr) repeat(5,minmax(70px,.55fr));gap:7px;padding:6px 0;border-top:1px solid #eee;font-size:12px}.material-row.material-head{color:#707a75}.material-row .shortage{color:#b85d0a;font-weight:700}.bulk-assignment{display:grid;grid-template-columns:minmax(240px,1fr) minmax(180px,260px) auto;gap:12px;align-items:end}.bulk-assignment p{font-size:12px}.assignment-panel{grid-template-columns:minmax(180px,280px) auto auto;align-items:end}
.issued-materials{display:grid;gap:5px;padding:10px;background:#edf8ef;border-radius:7px;color:#246b40}.issued-materials>div{display:flex;justify-content:space-between;gap:10px}.material-readiness details{margin-top:8px}.material-readiness.material-state-ready summary{color:#23824d;font-weight:700}.material-readiness.material-state-shortage summary{color:#b85d0a;font-weight:700}.material-state-unknown{margin-top:5px;color:#7b8580;font-size:12px}
.pill {
  justify-self: start;
  border: 1px solid #d8d2c8;
  border-radius: 999px;
  padding: 3px 8px;
  background: #faf9f7;
  font-size: 12px;
}
.pill.running { border-color: #bdd8f0; background: #eef7ff; }
.pill.danger { border-color: #efb9b9; background: #fff2f2; color: #9d2424; }
.actions { display: flex; gap: 6px; flex-wrap: wrap; }
.actions button { min-height: 30px; padding: 4px 8px; font-size: 12px; }
.material-actions{position:relative}.material-actions summary{list-style:none;border:1px solid #cfc8bf;border-radius:8px;padding:5px 8px;cursor:pointer;font-size:12px}.material-actions button{display:block;width:100%;margin-top:4px;background:#fff}
.task-feedback {
  grid-column: 1 / -1;
  border: 1px solid #b7dfc4;
  border-radius: 6px;
  padding: 7px 9px;
  background: #effaf2;
  color: #175c2f;
  font-size: 13px;
}
.task-feedback.error {
  border-color: #efb9b9;
  background: #fff2f2;
  color: #9d2424;
}
.empty-state {
  grid-column: 1 / -1;
  display: grid;
  gap: 6px;
  border: 1px solid #e2ded7;
  border-radius: 8px;
  padding: 18px;
  color: #777;
  background: #fff;
}
.empty-state strong { color: #222; font-size: 16px; }
.task-action-panel {
  grid-column: 1 / -1;
  display: grid;
  gap: 10px;
  margin-top: 4px;
  padding: 12px;
  border: 1px solid #ebe7df;
  border-radius: 8px;
  background: #fffdf8;
}
.section-title-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}
.section-title { font-weight: 800; font-size: 16px; line-height: 1.25; }
.muted { color: #777; font-size: 13px; }
.form-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  align-items: end;
}
.form-grid .span-2 { grid-column: span 2; }
.packaging-summary { margin: 16px 0; padding: 14px; border: 1px solid #d7e4dc; border-radius: 10px; background: #f5faf7; }
.packaging-summary h4 { margin: 0 0 8px; }
.packaging-summary > p { margin: 0 0 12px; color: #607568; font-size: 13px; }
.packaging-summary-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 10px; }
.packaging-summary-grid article { display: grid; gap: 7px; padding: 12px; background: white; border: 1px solid #dce7e0; border-radius: 8px; min-width: 0; overflow-wrap: anywhere; }
.packaging-summary-grid span { color: #246a45; font-weight: 600; }
.packaging-summary-grid small { color: #607568; }

@media (max-width: 1100px) {
  .station-grid { grid-template-columns: 1fr; }
}

@media (max-width: 760px) {
  .page { padding: 14px; }
  .toolbar, .station-head { align-items: stretch; flex-direction: column; }
  .answer-grid, .form-grid { grid-template-columns: 1fr; }
  .packaging-summary-grid { grid-template-columns: minmax(0, 1fr); }
  .form-grid .span-2 { grid-column: auto; }
  .task-row { grid-template-columns: 1fr; min-width: 0; align-items: start; }
  .task-row.header { display: none; }
  .bulk-assignment,.assignment-panel,.material-row{grid-template-columns:1fr}.material-row.material-head{display:none}
  .material-row:not(.material-head)>span{display:flex;justify-content:space-between;gap:12px}
  .material-row:not(.material-head)>span::before{content:attr(data-label);font-weight:400;color:#707a75}
}
</style>
