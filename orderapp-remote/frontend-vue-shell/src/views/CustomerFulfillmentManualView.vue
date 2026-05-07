<template>
  <div class="manual-page">
    <section class="intro panel">
      <div>
        <p class="eyebrow">客户履约账户操作手册</p>
        <h2>代加工、代发、库存和月结怎么跑</h2>
        <p class="lead">这页给 ERP 操作人员看：先准备客户资料，再按文件类型解析导入，确认没有关键错误后应用批次，最后核对托管库存、加工工单、代发订单、费用和结算。</p>
      </div>
      <button class="primary" type="button" @click="openAccount">打开客户履约账户</button>
    </section>

    <section class="panel">
      <div class="section-title">常用入口</div>
      <div class="table-wrap">
        <table>
          <tbody>
            <tr v-for="row in entries" :key="row.name">
              <th>{{ row.name }}</th>
              <td>{{ row.text }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">导入流程</div>
      <div class="flow">
        <div v-for="(step, index) in flow" :key="step.title" class="step">
          <span>{{ index + 1 }}</span>
          <strong>{{ step.title }}</strong>
          <p>{{ step.text }}</p>
        </div>
      </div>
      <div class="note-strip">解析导入只保存批次和错误行，不改变库存、订单或费用。应用最新批次才会写入业务数据。</div>
    </section>

    <section class="grid">
      <article v-for="item in workbookTypes" :key="item.title" class="panel compact">
        <div class="tag">{{ item.tag }}</div>
        <h3>{{ item.title }}</h3>
        <p>{{ item.text }}</p>
        <ul>
          <li v-for="point in item.points" :key="point">{{ point }}</li>
        </ul>
      </article>
    </section>

    <section class="panel">
      <div class="section-title">导入后核对</div>
      <div class="check-grid">
        <div v-for="group in checks" :key="group.title" class="check">
          <strong>{{ group.title }}</strong>
          <ul>
            <li v-for="item in group.items" :key="item">{{ item }}</li>
          </ul>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">生成月结</div>
      <div class="month-close">
        <div v-for="item in settlementChecks" :key="item.title" class="month-item">
          <strong>{{ item.title }}</strong>
          <span>{{ item.text }}</span>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">常见问题</div>
      <div class="qa-list">
        <div v-for="qa in faqs" :key="qa.q" class="qa">
          <strong>{{ qa.q }}</strong>
          <span>{{ qa.a }}</span>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">验收检查</div>
      <ul class="acceptance">
        <li v-for="item in acceptance" :key="item">{{ item }}</li>
      </ul>
    </section>
  </div>
</template>

<script setup>
const entries = [
  { name: '客户履约账户', text: '选择客户、上传 Excel、解析导入、应用批次，并查看库存、工单、代发、费用和结算。' },
  { name: '客户门户配置', text: '维护客户能力、代加工仓库和默认寄件人。' },
  { name: '产品设置', text: '维护客户专属 SKU；客户 SKU 不进入公共产品列表，也不被其他客户误选。' },
  { name: '仓库库存', text: '查看客户代加工仓、托管生豆、包材和客户成品。' },
  { name: '订单列表', text: '查看代发导入形成的订单，并继续发货和回填快递单号。' },
]

const flow = [
  { title: '选择客户 ID', text: '确认客户档案已经存在，并载入该客户履约账户。' },
  { title: '选择导入类型', text: '代加工工单、代发清单、结算单三选一，不要混选。' },
  { title: '解析导入', text: '上传 Excel 后先解析，查看有效行、错误行和业务摘要。' },
  { title: '修正错误', text: '表头、日期、数量、金额、产品名异常时，先改 Excel 后重新解析。' },
  { title: '应用最新批次', text: '确认解析结果后应用，系统写入库存、工单、订单和费用。' },
  { title: '刷新核对', text: '核对托管库存、加工工单、代发订单、费用明细和结算批次。' },
]

const workbookTypes = [
  {
    tag: 'processing_workbook',
    title: '代加工工单',
    text: '用于客户共享的烘焙工单、物料库存和完成情况表。',
    points: ['维护托管生豆、包材和客户产品库存', '生成或更新加工工单、包装任务', '记录烘焙费、磨粉费、包装费、新品测试费和仓储费'],
  },
  {
    tag: 'direct_ship_workbook',
    title: '代发清单',
    text: '用于客户每天发来的下游收件人订单。',
    points: ['创建客户代发订单和收件人快照', '保留商品明细、数量和外部订单号', '记录代发服务费和可结算运费'],
  },
  {
    tag: 'settlement_workbook',
    title: '结算单',
    text: '用于月度结算代加工费、运费、代发费、仓储费和调整项。',
    points: ['导入结算费用明细', '生成客户结算批次', '用于和客户对账，正式财务仍需复核'],
  },
]

const checks = [
  { title: '托管库存', items: ['生豆、包材、客户成品按客户独立记录', '数量方向和共享表余额一致', '客户自购与向棵凡购买的包材口径清楚'] },
  { title: '加工工单', items: ['工单号、日期、产品、计划数量和状态正确', '客户 SKU 属于当前客户', '重复应用不会重复增加工单或库存'] },
  { title: '代发订单', items: ['外部订单号、收件人、电话、地址、商品和数量正确', '下游收件人只进入订单快照，不进入客户档案', '订单能进入后续发货流程'] },
  { title: '费用明细', items: ['烘焙费、磨粉费、代发费、仓储费、物流费和调整项分类正确', '费用归属到正确客户和期间', '月结前确认金额和表格一致'] },
]

const settlementChecks = [
  { title: '先补齐工单', text: '当月加工工单先完成解析和应用。' },
  { title: '先补齐代发', text: '当月代发清单先完成解析和应用。' },
  { title: '核对运费', text: '快递月结金额或导入金额要能追到订单。' },
  { title: '核对额外费用', text: '仓储费、新品测试费和包材预付款要进入费用明细。' },
  { title: '生成月结', text: '填写结算开始和结束日期，点击生成月结后核对结算批次。' },
]

const faqs = [
  { q: '页面没有客户数据', a: '先确认客户 ID 是否正确、客户档案是否存在，并点击载入账户。' },
  { q: '解析后有错误行', a: '检查表头、日期、数量、金额、产品名和空行；修正 Excel 后重新解析。' },
  { q: '上传错类型', a: '不要应用该批次，改选正确类型后重新上传解析。' },
  { q: '应用后数据没变', a: '先刷新账户，再确认最新批次状态、有效行数量和客户 ID 是否正确。' },
  { q: '客户产品找不到', a: '确认产品已作为当前客户专属 SKU 维护，不能直接使用其他客户 SKU。' },
]

const acceptance = [
  '客户 ID 能载入客户履约账户。',
  '三类 Excel 都能先解析，解析结果显示有效行和错误行。',
  '解析失败不会改变客户库存、订单或费用。',
  '应用代加工工单后能看到托管生豆、包材、客户成品、加工工单和相关费用。',
  '应用代发清单后能看到代发订单、收件人快照、商品明细和代发服务费。',
  '生成月结后能看到结算批次和费用汇总。',
  '重复应用同一批次不会重复增加库存、订单或费用。',
]

function openAccount() {
  window.dispatchEvent(new CustomEvent('kferp:navigate-view', {
    detail: { key: 'customerFulfillment' },
  }))
}
</script>

<style scoped>
* { box-sizing: border-box; }
.manual-page { padding: 18px; display: grid; gap: 14px; color: #171717; background: #f7f7f5; }
.panel { border: 1px solid #d8dee4; border-radius: 8px; background: #fff; padding: 14px; }
.intro { display: flex; justify-content: space-between; gap: 16px; align-items: flex-start; }
.eyebrow { margin: 0 0 8px; color: #64748b; font-size: 13px; font-weight: 800; }
h2, h3 { margin: 0; }
.lead { max-width: 860px; margin: 8px 0 0; color: #475569; line-height: 1.7; }
.primary { border: 0; border-radius: 8px; background: #1f2937; color: #fff; padding: 10px 14px; font-weight: 800; cursor: pointer; white-space: nowrap; }
.section-title { font-weight: 900; margin-bottom: 10px; }
.table-wrap { overflow-x: auto; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eef2f7; padding: 10px 8px; text-align: left; vertical-align: top; font-size: 14px; }
th { width: 150px; background: #f8fafc; }
.flow { display: grid; grid-template-columns: repeat(6, minmax(0, 1fr)); gap: 10px; }
.step { border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; background: #fbfbfb; min-height: 150px; }
.step span { display: inline-flex; width: 28px; height: 28px; align-items: center; justify-content: center; border-radius: 999px; background: #1f2937; color: #fff; margin-bottom: 10px; }
.step strong { display: block; margin-bottom: 6px; }
.step p { margin: 0; color: #475569; line-height: 1.55; }
.note-strip { margin-top: 12px; border-left: 4px solid #1f2937; background: #f8fafc; padding: 10px 12px; color: #334155; line-height: 1.6; }
.grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 14px; }
.compact { display: grid; gap: 8px; }
.compact p { margin: 0; color: #475569; line-height: 1.6; }
.tag { width: fit-content; border: 1px solid #cbd5e1; border-radius: 999px; padding: 3px 8px; font-size: 12px; color: #475569; background: #f8fafc; }
ul { margin: 0; padding-left: 18px; line-height: 1.8; color: #334155; }
.check-grid { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 12px; }
.check { border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; background: #fbfbfb; }
.check strong { display: block; margin-bottom: 8px; }
.month-close { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; }
.month-item { border: 1px solid #e2e8f0; border-radius: 8px; padding: 12px; min-height: 112px; }
.month-item strong, .month-item span { display: block; }
.month-item span { margin-top: 6px; color: #475569; line-height: 1.55; }
.qa-list { display: grid; gap: 10px; }
.qa { border-bottom: 1px solid #eef2f7; padding-bottom: 10px; }
.qa strong, .qa span { display: block; }
.qa span { margin-top: 5px; color: #475569; line-height: 1.55; }
.acceptance { columns: 2; column-gap: 32px; }
@media (max-width: 1200px) {
  .flow { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .grid, .check-grid, .month-close { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 760px) {
  .manual-page { padding: 12px; }
  .intro { display: grid; }
  .flow, .grid, .check-grid, .month-close { grid-template-columns: 1fr; }
  .acceptance { columns: 1; }
}
</style>
