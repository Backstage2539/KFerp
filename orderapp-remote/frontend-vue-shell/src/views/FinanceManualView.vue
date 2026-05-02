<template>
  <div class="page">
    <section class="panel">
      <h2>财务月结手册</h2>
      <div class="flow">
        <div v-for="item in flow" :key="item.title" class="step">
          <span>{{ item.no }}</span>
          <strong>{{ item.title }}</strong>
          <p>{{ item.text }}</p>
        </div>
      </div>
    </section>

    <section class="panel">
      <div class="section-title">入口</div>
      <table>
        <tbody>
          <tr><th>财务首页</th><td>查看当月收入、成本、费用、毛利、净利和税费估算。</td></tr>
          <tr><th>费用管理</th><td>补录期间费用或主营成本，强锁账月份结账后不可继续补录同月来源费用。</td></tr>
          <tr><th>月度结账</th><td>确认月度快照并结账，结账后通过金额调整修正。</td></tr>
          <tr><th>经营报告</th><td>查看老板简报、利润表、税费表，并导出 PDF 和 Excel。</td></tr>
          <tr><th>财务设置</th><td>维护企业类型、纳税人类型、税率参数和锁账模式。</td></tr>
        </tbody>
      </table>
    </section>

    <section class="grid">
      <div class="panel">
        <div class="section-title">结账后调整类型</div>
        <ul>
          <li>收入：影响不含税收入、毛利和净利。</li>
          <li>主营成本：影响主营成本、毛利和净利。</li>
          <li>期间费用：影响期间费用和净利。</li>
          <li>税费：影响税费合计和净利。</li>
          <li>其他：记录备查。</li>
        </ul>
      </div>
      <div class="panel">
        <div class="section-title">锁账模式</div>
        <ul>
          <li>强锁账：默认模式，结账后锁定来源费用。</li>
          <li>轻确认：试运行模式，结账后允许继续补录来源费用。</li>
          <li>切换入口只对白名单用户显示，接口同时校验权限和白名单。</li>
        </ul>
      </div>
    </section>
  </div>
</template>

<script setup>
const flow = [
  { no: '1', title: '设置口径', text: '选择企业类型、纳税人类型和税率参数。' },
  { no: '2', title: '补齐费用', text: '按日期录入期间费用或主营成本。' },
  { no: '3', title: '检查首页', text: '确认收入、成本、费用、毛利、净利和异常提醒。' },
  { no: '4', title: '月度结账', text: '确认快照后结账，强锁账月份锁定来源费用。' },
  { no: '5', title: '报告留档', text: '查看老板简报，导出 PDF 或 Excel。' },
]
</script>

<style scoped>
* { box-sizing: border-box; }
.page { padding: 18px; color: #171717; display: grid; gap: 14px; }
.panel { border: 1px solid #e3e3e3; border-radius: 8px; background: #fff; padding: 14px; }
h2 { margin: 0 0 14px; font-size: 20px; }
.section-title { font-weight: 800; margin-bottom: 10px; }
.flow { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 10px; }
.step { border: 1px solid #e3e3e3; border-radius: 8px; padding: 12px; background: #fbfbfb; min-height: 142px; }
.step span { display: inline-flex; width: 28px; height: 28px; align-items: center; justify-content: center; border-radius: 999px; background: #1f1f1f; color: #fff; margin-bottom: 10px; }
.step strong { display: block; margin-bottom: 6px; }
.step p { margin: 0; color: #666; line-height: 1.45; }
.grid { display: grid; grid-template-columns: 1fr 1fr; gap: 14px; }
table { width: 100%; border-collapse: collapse; }
th, td { border-bottom: 1px solid #eee; padding: 9px 8px; text-align: left; font-size: 14px; }
th { width: 140px; background: #fbfbfb; }
ul { margin: 0; padding-left: 18px; color: #333; line-height: 1.8; }
@media (max-width: 1100px) {
  .flow { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 900px) {
  .page { padding: 12px; }
  .flow, .grid { grid-template-columns: 1fr; }
}
</style>
