<template>
  <div class="manual-page">
    <section class="intro">
      <div>
        <p class="eyebrow">生产流程用户手册</p>
        <h2>从原料入库到成品追溯</h2>
        <p class="lead">一线操作只按这一条主线走：先入库，再领到 WIP，开始生产，按工单执行，完工入库，最后按成品批次追溯。</p>
      </div>
    </section>

    <section class="panel">
      <div class="section-head">
        <h3>一张图看懂流程</h3>
      </div>
      <div class="flow-row" aria-label="生产流程">
        <div v-for="(step, index) in mainFlow" :key="step.title" class="flow-step">
          <div class="step-no">{{ index + 1 }}</div>
          <strong>{{ step.title }}</strong>
          <span>{{ step.note }}</span>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="section-head">
        <h3>每天怎么用</h3>
      </div>
      <div class="table-wrap">
        <table>
          <thead>
            <tr>
              <th>时间点</th>
              <th>去哪里</th>
              <th>做什么</th>
              <th>结果</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in dailyRows" :key="row.when">
              <td><strong>{{ row.when }}</strong></td>
              <td>{{ row.where }}</td>
              <td>{{ row.action }}</td>
              <td>{{ row.result }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section class="panel">
      <div class="section-head">
        <h3>仓库怎么分</h3>
      </div>
      <div class="warehouse-flow">
        <div class="warehouse-node raw">原料仓<span>未领到生产现场</span></div>
        <div class="arrow">WIP领料</div>
        <div class="warehouse-node wip">WIP在制仓<span>已领用，未消耗</span></div>
        <div class="arrow">完工扣料</div>
        <div class="warehouse-node done">生产完成<span>生成成品批次</span></div>
        <div class="warehouse-branch">
          <div class="warehouse-node finished">成品仓<span>默认成品库存</span></div>
          <div class="warehouse-node shop">门店成品仓<span>门店或前置库存</span></div>
        </div>
      </div>
      <div class="note-strip">WIP 不要求一次生产用完。先领用 60kg 生豆到 WIP，可以放多天，也可以被多个商品分批消耗；没用完的可以留在 WIP 或退回原料仓。</div>
    </section>

    <section class="steps-grid">
      <article v-for="step in operationSteps" :key="step.title" class="step-panel">
        <div class="step-label">{{ step.label }}</div>
        <h3>{{ step.title }}</h3>
        <p class="route">{{ step.route }}</p>
        <ul>
          <li v-for="item in step.items" :key="item">{{ item }}</li>
        </ul>
      </article>
    </section>

    <section class="panel">
      <div class="section-head">
        <h3>完成生产后系统自动做什么</h3>
      </div>
      <div class="auto-grid">
        <div v-for="item in autoActions" :key="item.title" class="auto-item">
          <strong>{{ item.title }}</strong>
          <span>{{ item.note }}</span>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="section-head">
        <h3>批次追溯</h3>
      </div>
      <div class="trace-line">
        <span>成品批次 FP-...</span>
        <span>生产日志</span>
        <span>生产工单</span>
        <span>原料消耗记录</span>
        <span>原料批次</span>
      </div>
      <p class="plain">入口：库存管理 -> 仓库库存。输入 `FP-...` 查成品生产链路；输入 `MB-...` 或 `LEGACY-MAT-...` 查原料批次和仓库位置。LEGACY-MAT 是系统升级按旧物料库存生成的期初批次。</p>
    </section>

    <section class="panel">
      <div class="section-head">
        <h3>常见问题</h3>
      </div>
      <div class="qa-list">
        <div v-for="qa in faqs" :key="qa.q" class="qa">
          <strong>{{ qa.q }}</strong>
          <span>{{ qa.a }}</span>
        </div>
      </div>
    </section>

    <section class="checklists">
      <div class="check-panel">
        <h3>现场检查清单：生产前</h3>
        <ul>
          <li v-for="item in beforeChecklist" :key="item">{{ item }}</li>
        </ul>
      </div>
      <div class="check-panel">
        <h3>现场检查清单：生产后</h3>
        <ul>
          <li v-for="item in afterChecklist" :key="item">{{ item }}</li>
        </ul>
      </div>
    </section>

    <section class="rules">
      <div v-for="(rule, index) in rules" :key="rule" class="rule">
        <span>{{ index + 1 }}</span>
        <strong>{{ rule }}</strong>
      </div>
    </section>
  </div>
</template>

<script setup>
const mainFlow = [
  { title: '生产验收', note: '开工前检查闭环' },
  { title: '基础资料', note: '商品 / BOM / 设备 / 物料' },
  { title: '原料入库', note: '库存作业' },
  { title: '领料到 WIP', note: '库存作业' },
  { title: '物料计划', note: '看 WIP 可用和建议领料' },
  { title: '开始生产', note: '冻结快照并占用 WIP' },
  { title: '生产工单', note: '查看烘焙建议并打印' },
  { title: '质检完工', note: '可部分完工并记录质检' },
  { title: '追溯复盘', note: '仓库库存 / 生产日志' },
]

const dailyRows = [
  { when: '开工检查', where: '生产管理 -> 生产验收', action: '看生产闭环检查项是否都具备', result: '知道今天是否缺基础数据或流程记录' },
  { when: '生产前', where: '商品与配方 / 设置', action: '确认商品、BOM 配方、设备产能', result: '生产计划能算出建议投料' },
  { when: '原料到货', where: '库存管理 -> 库存作业 -> 原料入库', action: '录入原料、数量、成本', result: '生成原料批次和原料仓库存' },
  { when: '准备生产', where: '库存管理 -> 库存作业 -> WIP领退/转仓', action: '把原料从原料仓领到 WIP', result: '生产现场有可消耗库存' },
  { when: '排产', where: '生产管理 -> 生产计划/开始生产', action: '选择要生产的商品，先看物料需求计划，再确认投料', result: '知道 WIP 可用、建议领到 WIP、缺料和本次占用' },
  { when: '生产中', where: '生产管理 -> 生产工单', action: '看烘焙建议、原料需求，打印工单', result: '现场按工单执行' },
  { when: '占用处理', where: '库存管理 -> 仓库库存 -> WIP占用', action: '按工单查看、调整或释放 WIP 占用', result: '异常工单不再锁住可用原料' },
  { when: '质检', where: '生产管理 -> 生产质检', action: '记录原料、工单或成品批次的通过/待处理/不合格', result: '质量记录可随单据追溯' },
  { when: '生产后', where: '生产管理 -> 生产中', action: '填实际产出和本次消耗投料，可选择部分完工', result: '扣 WIP 原料，生成成品库存，未完工部分继续保留' },
  { when: '复盘', where: '库存管理 -> 仓库库存', action: '查库存、查 FP 成品批次追溯；查 MB/LEGACY-MAT 原料批次位置', result: '看到成品用了哪些原料批次，也能解释旧库存期初批次在哪个仓库' },
]

const operationSteps = [
  {
    label: '第 0 步',
    title: '生产验收',
    route: '生产管理 -> 生产验收',
    items: ['查看仓库、原料、WIP、工单、日志、质检和追溯检查项', '缺数据时按页面入口补齐', '部署后先用一条小工单跑完整流程'],
  },
  {
    label: '第 1 步',
    title: '准备基础资料',
    route: '商品档案 / BOM配方维护 / 物料档案 / 设备产能配置',
    items: ['建好商品和规格', '启用 BOM 配方', '维护原料和包材', '维护设备产能'],
  },
  {
    label: '第 2 步',
    title: '原料入库',
    route: '库存管理 -> 库存作业 -> 原料入库',
    items: ['选择物料', '填写入库数量和成本', '提交后生成原料批次'],
  },
  {
    label: '第 3 步',
    title: '领料到 WIP',
    route: '库存管理 -> 库存作业 -> WIP领退/转仓',
    items: ['选择“领料到 WIP”', '选择原料批次', '填写领用数量', '未用完可退回原料仓'],
  },
  {
    label: '第 4 步',
    title: '开始生产',
    route: '生产管理 -> 生产计划/开始生产',
    items: ['选择要生产的行', '点击物料需求计划检查 WIP 可用和建议领到 WIP', '建议领到 WIP 大于 0 时先去库存作业领料', '检查建议投料和原料需求', '点击开始生产', '系统冻结 BOM 和原料快照并建立 WIP 占用'],
  },
  {
    label: '第 5 步',
    title: '处理 WIP 占用',
    route: '仓库库存 -> WIP占用 / 生产工单',
    items: ['按工单查看已占、已耗和剩余占用', '工单废弃或取消时释放剩余占用', '占用录错时调整为正确克重', '调整和释放都会写操作日志'],
  },
  {
    label: '第 6 步',
    title: '查看和打印生产工单',
    route: '生产管理 -> 生产工单',
    items: ['查看商品规格和订单信息', '查看烘焙建议', '确认原料摘要', '需要纸质单时直接打印'],
  },
  {
    label: '第 7 步',
    title: '质检和完成生产',
    route: '生产管理 -> 生产中',
    items: ['需要时先到生产质检记录工单检查结果', '填写实际产出件数', '如有散重，填写散重克数', '本次只完成一部分时勾选部分完工并填写本次消耗投料', '选择成品入库仓', '点击完成'],
  },
  {
    label: '第 8 步',
    title: '成品转仓',
    route: '库存管理 -> 库存作业 -> 成品转仓',
    items: ['选择商品和规格', '选择来源仓和目标仓', '填写转仓数量', '提交后只改变仓库位置'],
  },
  {
    label: '第 9 步',
    title: '查询和追溯',
    route: '库存管理 -> 仓库库存',
    items: ['按仓库查看库存', '展开批次余额', '输入 FP 成品批次追到工单、日志和原料消耗', '输入 MB 或 LEGACY-MAT 原料批次查看当前仓库位置', 'LEGACY-MAT 是系统升级按旧物料库存生成的期初批次'],
  },
]

const autoActions = [
  { title: '释放或保留 WIP 占用', note: '全部完成会释放工单占用；部分完工只减少本次消耗，剩余继续占用' },
  { title: '扣 WIP 原料', note: '按工单冻结的原料快照，从 WIP 批次扣料' },
  { title: '生成成品批次', note: '成品批次号通常是 FP-...' },
  { title: '增加成品库存', note: '进入你选择的成品仓或门店成品仓' },
  { title: '写生产日志', note: '记录实际产出和生产批次' },
  { title: '写成本记录', note: '生产成本页面可查看' },
  { title: '建立追溯链路', note: '成品批次可以追到原料批次' },
]

const faqs = [
  { q: 'WIP库存不足', a: '原料只在原料仓，没有领到 WIP。先去库存作业里做 WIP 领料。' },
  { q: 'WIP看起来有库存，但开始生产仍提示不足', a: '可能已被其他工单占用。先在物料需求计划里看 WIP 可用和建议领到 WIP，再到仓库库存的 WIP占用处理异常占用。' },
  { q: '物料档案显示有库存，但生产仍提示不足', a: '总库存充足不等于 WIP 充足。确认该物料在 WIP 有余额。' },
  { q: '工单废弃后仍占用 WIP', a: '到仓库库存的 WIP占用抽屉，按工单释放剩余占用。' },
  { q: '工单占用量录错', a: '到 WIP占用抽屉调整占用克重，不能低于已消耗数量，也不能超过 WIP 可用量。' },
  { q: '一张工单不能一次做完', a: '在生产中勾选部分完工，填写本次实际产出和本次消耗投料，剩余部分会留在生产中继续做。' },
  { q: '工单原料和最新 BOM 不一样', a: '工单开始时已经冻结快照，这是正常规则，新 BOM 只影响后续工单。' },
  { q: '找不到追溯', a: '先确认批次号存在。FP-... 查成品到工单的完整链路；MB-... / LEGACY-MAT-... 查原料批次和仓库位置。' },
  { q: '成品放错仓', a: '用库存作业里的成品转仓调整。' },
]

const beforeChecklist = [
  '商品档案已建好',
  'BOM 配方已启用',
  '原料已经入库',
  '生产要用的原料已经领到 WIP',
  '生产验收页没有关键异常',
  '物料需求计划没有无法处理的缺料，建议领到 WIP 已处理',
  '生产计划里已经开始生产',
  '生产工单已打印或现场可查看',
]

const afterChecklist = [
  '生产中记录已经完成',
  '成品进入正确仓库',
  'WIP 原料余额减少',
  '生产日志能看到本次记录',
  '需要质检的原料、工单或成品批次已录入质检结果',
  '生产成本能看到本次成本',
  '仓库库存里用 FP 批次能追溯到原料批次，用 MB/LEGACY-MAT 能查原料批次位置',
  '不再继续生产的工单已释放 WIP 占用',
]

const rules = [
  '原料入库只进原料仓，生产不能直接消耗原料仓。',
  '生产完工只消耗 WIP 在制仓，所以生产前必须先领料到 WIP。',
  '开始生产会占用 WIP 库存，避免多个工单同时抢同一批料。',
  '工单开始后会冻结 BOM 和原料快照，保证后续追溯按当时的生产依据走。',
]
</script>

<style scoped>
* { box-sizing: border-box; }
.manual-page { padding: 16px; display: grid; gap: 14px; color: #171717; background: #f7f7f5; }
.intro, .panel, .step-panel, .check-panel, .rules { background: #fff; border: 1px solid #e2e2dc; border-radius: 8px; }
.intro { padding: 18px; }
.eyebrow { margin: 0 0 6px; font-size: 12px; color: #6b7280; font-weight: 700; }
h2, h3 { margin: 0; letter-spacing: 0; }
h2 { font-size: 24px; }
h3 { font-size: 17px; }
.lead { max-width: 780px; margin: 8px 0 0; color: #4b5563; line-height: 1.6; }
.panel { padding: 14px; }
.section-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 12px; }
.flow-row { display: grid; grid-template-columns: repeat(9, minmax(120px, 1fr)); gap: 8px; overflow-x: auto; padding-bottom: 2px; }
.flow-step { min-width: 120px; border: 1px solid #d8ded6; border-radius: 8px; padding: 10px; background: #fbfcfa; position: relative; }
.flow-step:not(:last-child)::after { content: '>'; position: absolute; right: -8px; top: 50%; transform: translateY(-50%); color: #63715f; font-weight: 700; }
.step-no { width: 24px; height: 24px; display: grid; place-items: center; border-radius: 50%; background: #193b2a; color: #fff; font-size: 12px; margin-bottom: 8px; }
.flow-step strong, .flow-step span { display: block; }
.flow-step span { color: #5f6b5f; font-size: 12px; line-height: 1.45; margin-top: 4px; }
.table-wrap { overflow: auto; }
table { width: 100%; min-width: 920px; border-collapse: collapse; }
th, td { border-bottom: 1px solid #ecece7; padding: 9px; text-align: left; vertical-align: top; font-size: 13px; line-height: 1.45; }
th { background: #f5f5f1; color: #555; }
.warehouse-flow { display: grid; grid-template-columns: minmax(130px, 1fr) 70px minmax(130px, 1fr) 70px minmax(130px, 1fr) minmax(170px, 1.2fr); gap: 10px; align-items: center; }
.warehouse-node { min-height: 72px; border-radius: 8px; border: 1px solid #d7d7d0; padding: 12px; font-weight: 700; background: #fff; }
.warehouse-node span { display: block; margin-top: 6px; color: #5f6368; font-size: 12px; font-weight: 500; }
.raw { border-color: #c9d7c1; background: #f6fbf3; }
.wip { border-color: #c9d6e8; background: #f4f8fd; }
.done { border-color: #dfd5b7; background: #fffaf0; }
.finished, .shop { min-height: 66px; }
.shop { margin-top: 8px; border-color: #d9c9c9; background: #fff7f7; }
.arrow { text-align: center; color: #596157; font-size: 12px; font-weight: 700; }
.note-strip { margin-top: 12px; border-left: 4px solid #193b2a; background: #f3f7f2; padding: 10px 12px; line-height: 1.6; color: #38443a; }
.steps-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.step-panel { padding: 14px; }
.step-label { display: inline-flex; border: 1px solid #d6d3cc; border-radius: 999px; padding: 3px 8px; color: #5f5b53; font-size: 12px; margin-bottom: 8px; }
.route { margin: 7px 0 10px; color: #57616b; font-size: 13px; }
ul { margin: 0; padding-left: 18px; line-height: 1.7; }
li { margin: 2px 0; }
.auto-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
.auto-item { border: 1px solid #e3e3dd; border-radius: 8px; padding: 10px; background: #fcfcfb; }
.auto-item strong, .auto-item span { display: block; }
.auto-item span { margin-top: 5px; color: #5f6368; font-size: 13px; line-height: 1.5; }
.trace-line { display: grid; grid-template-columns: repeat(5, minmax(110px, 1fr)); gap: 8px; overflow-x: auto; }
.trace-line span { border: 1px solid #d8ded6; border-radius: 8px; padding: 10px; background: #fbfcfa; text-align: center; font-weight: 700; font-size: 13px; position: relative; }
.trace-line span:not(:last-child)::after { content: '>'; position: absolute; right: -8px; top: 50%; transform: translateY(-50%); color: #63715f; }
.plain { margin: 10px 0 0; color: #555; }
.qa-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.qa { border: 1px solid #e3e3dd; border-radius: 8px; padding: 10px; background: #fff; }
.qa strong, .qa span { display: block; }
.qa span { margin-top: 5px; color: #555; line-height: 1.5; }
.checklists { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 12px; }
.check-panel { padding: 14px; }
.check-panel h3 { margin-bottom: 10px; }
.rules { padding: 14px; display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 10px; }
.rule { border: 1px solid #d8ded6; border-radius: 8px; padding: 10px; background: #fbfcfa; display: grid; gap: 7px; }
.rule span { width: 24px; height: 24px; display: grid; place-items: center; border-radius: 50%; background: #193b2a; color: #fff; font-size: 12px; }
.rule strong { line-height: 1.5; }
@media (max-width: 1100px) {
  .flow-row, .warehouse-flow, .trace-line { grid-template-columns: none; grid-auto-flow: column; grid-auto-columns: minmax(150px, 1fr); overflow-x: auto; }
  .warehouse-branch { min-width: 170px; }
  .steps-grid, .auto-grid, .qa-list, .checklists, .rules { grid-template-columns: 1fr; }
}
@media (max-width: 700px) {
  .manual-page { padding: 10px; }
  .intro, .panel, .step-panel, .check-panel, .rules { padding: 12px; }
  h2 { font-size: 20px; }
  table { min-width: 780px; }
}
</style>
