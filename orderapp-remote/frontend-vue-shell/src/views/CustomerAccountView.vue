<template>
  <div class="account-page">
    <section class="panel">
      <header>
        <div>
          <h2>{{ heading }}</h2>
          <p>
            {{ data.customer_name || customerContextLabel }} ·
            {{
              isStatement ? "当期订单及当前付款情况" : "购买、代发与历史订单"
            }}
          </p>
        </div>
        <button @click="load(1)" :disabled="loading">刷新</button>
      </header>
      <form class="filters" @submit.prevent="load(1)">
        <template v-if="isStatement"
          ><label
            >账期<select v-model="filters.period">
              <option value="month">月账单</option>
              <option value="week">周账单</option>
            </select></label
          ><label
            >账期内任意一天<input
              v-model="filters.anchor"
              type="date"
              required /></label
        ></template>
        <template v-else
          ><label
            >开始日期<input v-model="filters.date_from" type="date" /></label
          ><label>结束日期<input v-model="filters.date_to" type="date" /></label
        ></template>
        <label
          >搜索<input
            v-model.trim="filters.q"
            placeholder="订单号、商品、收件人、电话"
        /></label>
        <label
          >付款状态<select v-model="filters.pay_status">
            <option value="">全部</option>
            <option value="unpaid">未付款</option>
            <option value="partial">部分付款</option>
            <option value="paid">已付款</option>
          </select></label
        >
        <label v-if="isOrders"
          >发货状态<select v-model="filters.ship_status">
            <option value="">全部</option>
            <option v-for="status in shipStatuses" :key="status">
              {{ status }}
            </option>
          </select></label
        >
        <label v-if="isOrders" class="check"
          ><input
            v-model="filters.include_void"
            type="checkbox"
          />包含作废订单</label
        >
        <button class="primary" :disabled="loading">
          {{ loading ? "查询中…" : "查询" }}
        </button>
      </form>
      <p v-if="error" class="error" role="alert">{{ error }}</p>
      <p v-if="message" class="success" role="status">{{ message }}</p>
      <div class="metrics">
        <article>
          <span>订单总额</span
          ><strong>¥{{ money(data.summary?.total_cents) }}</strong>
        </article>
        <article>
          <span>已付金额</span
          ><strong>¥{{ money(data.summary?.paid_cents) }}</strong>
        </article>
        <article>
          <span>待付金额</span
          ><strong>¥{{ money(data.summary?.due_cents) }}</strong>
        </article>
        <article>
          <span>有效订单</span
          ><strong>{{ data.summary?.count || 0 }} 笔</strong>
        </article>
      </div>
      <p class="muted">
        {{ data.date_from || "全部历史"
        }}{{ data.date_to ? ` 至 ${data.date_to}` : "" }} · 查询时间
        {{ data.as_of || "—" }}。金额汇总包含全部符合筛选条件的有效订单。
      </p>
      <div v-if="isStatement" class="actions">
        <button @click="downloadStatement('xlsx')">下载 Excel 账单</button
        ><button @click="downloadStatement('pdf')">下载 PDF 账单</button>
      </div>
      <details class="help">
        <summary>查询与对账说明</summary>
        <p>
          周账单按周一至周日、月账单按自然月，以北京时间和下单日期查询。金额显示订单目前已付和待付情况，不代表当期收款流水。预付款计入已付金额；作废订单不计入合计。
        </p>
        <p>
          未发货且未作废的订单可以修改收件信息。粘贴完整地址后点击解析，检查姓名、电话和地址再保存。销售单使用工厂模板，可下载单笔或多笔订单的
          PDF、图片，合并导出不会合并原始订单。已有正式结算单仅供查看和下载，收款由工厂登记。
        </p>
      </details>
    </section>
    <section class="panel">
      <header>
        <h3>{{ isOrders ? "我的订单" : "订单费用明细" }}</h3>
        <div class="actions" v-if="isOrders">
          <button
            :disabled="selected.length < 2 || selected.length > 100"
            @click="salesIDs = [...selected]"
          >
            合并销售单（{{ selected.length }}）</button
          ><button v-if="selected.length" @click="showSelected = !showSelected">
            查看已选</button
          ><button v-if="selected.length" @click="selected = []">
            清空已选
          </button>
        </div>
      </header>
      <div v-if="showSelected && selected.length" class="selection">
        <span v-for="id in selected" :key="id"
          >{{ selectedNames[id] || id }}
          <button
            :aria-label="`取消选择 ${selectedNames[id] || id}`"
            @click="selected = selected.filter((v) => v !== id)"
          >
            ×
          </button></span
        >
      </div>
      <div class="table-scroll">
        <table>
          <thead>
            <tr>
              <th v-if="isOrders">
                <input
                  type="checkbox"
                  :checked="allSelected"
                  :indeterminate.prop="someSelected && !allSelected"
                  aria-label="选择当前页订单"
                  @change="selectPage"
                />
              </th>
              <th>订单 / 下单日期</th>
              <th v-if="isOrders">收件信息 / 物流</th>
              <th>商品金额</th>
              <th>运费</th>
              <th>优惠</th>
              <th>订单总额</th>
              <th>已付 / 待付</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="row in data.rows"
              :key="row.id"
              :class="{ void: row.is_void }"
            >
              <td v-if="isOrders">
                <input
                  v-model="selected"
                  :value="row.id"
                  :disabled="row.is_void"
                  type="checkbox"
                  :aria-label="`选择 ${row.order_no}`"
                  @change="rememberRows"
                />
              </td>
              <td>
                {{ row.order_no }}<small>{{ row.order_date }}</small>
              </td>
              <td v-if="isOrders">
                {{ row.receiver_name || "待补收件人" }} {{ row.receiver_phone
                }}<small>{{ row.receiver_address || "待补地址" }}</small
                ><small>{{ row.ship_tracking_no }}</small>
              </td>
              <td>{{ money(row.goods_cents) }}</td>
              <td>{{ money(row.shipping_cents) }}</td>
              <td>{{ money(row.discount_cents) }}</td>
              <td>{{ money(row.total_cents) }}</td>
              <td>{{ money(row.paid_cents) }} / {{ money(row.due_cents) }}</td>
              <td>
                {{ row.is_void ? "已作废" : accountStatus(row.payment_status)
                }}<small>{{ row.ship_status }}</small>
              </td>
              <td>
                <button @click="openDetail(row.id)">
                  详情{{ isOrders ? " / 收件信息" : "" }}</button
                ><button v-if="!row.is_void" @click="salesIDs = [row.id]">
                  销售单
                </button>
              </td>
            </tr>
            <tr v-if="!data.rows?.length">
              <td colspan="10">
                {{ loading ? "正在加载…" : "此范围没有订单" }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <form class="pager" @submit.prevent="load(jumpPage)">
        <span
          >共 {{ data.total || 0 }} 笔 · 第 {{ data.page || 1 }} /
          {{ data.total_pages || 1 }} 页</span
        ><button
          type="button"
          :disabled="loading || data.page <= 1"
          @click="load(data.page - 1)"
        >
          上一页</button
        ><button
          type="button"
          :disabled="loading || data.page >= data.total_pages"
          @click="load(data.page + 1)"
        >
          下一页</button
        ><label
          >每页<select v-model.number="limit" @change="load(1)">
            <option :value="20">20</option>
            <option :value="50">50</option>
            <option :value="100">100</option>
          </select></label
        ><input
          v-model.number="jumpPage"
          type="number"
          min="1"
          :max="data.total_pages || 1"
          aria-label="跳转页码"
        /><button :disabled="loading">跳转</button>
      </form>
    </section>
    <template v-if="!isOrders">
      <section class="panel">
        <h3>独立费用</h3>
        <p class="muted">
          按费用发生日期查询。订单费用与独立费用分列；正式结算单是来源费用的汇总，不重复累加。未关联订单的历史费用单独保留。
        </p>
        <div class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>发生日期</th>
                <th>关联订单</th>
                <th>费用</th>
                <th>金额</th>
                <th>结算单 / 状态</th>
                <th>付款状态</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="fee in data.fees" :key="fee.id">
                <td>{{ fee.occurred_at }}</td>
                <td>
                  <button v-if="fee.order_id" @click="openDetail(fee.order_id)">
                    {{ fee.order_no }}</button
                  ><span v-else>未关联订单</span>
                </td>
                <td>{{ accountFeeType(fee.fee_type) }}</td>
                <td>{{ money(fee.amount_cents) }} {{ fee.currency }}</td>
                <td>
                  {{ fee.settlement_no || "未入结算"
                  }}<small>{{ accountStatus(fee.settlement_status) }}</small>
                </td>
                <td>{{ accountStatus(fee.payment_status) }}</td>
              </tr>
              <tr v-if="!data.fees?.length">
                <td colspan="6">暂无独立费用</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
      <section v-if="isStatement" class="panel">
        <h3>已有正式结算单</h3>
        <p class="muted">全部历史正式结算单；草稿不在此列出。</p>
        <div class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>结算单号</th>
                <th>账期</th>
                <th>金额</th>
                <th>状态</th>
                <th>确认 / 付款时间</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="bill in data.settlements" :key="bill.id">
                <td>{{ bill.settlement_no }}</td>
                <td>{{ bill.period_from }} 至 {{ bill.period_to }}</td>
                <td>{{ money(bill.total_cents) }}</td>
                <td>{{ accountStatus(bill.status) }}</td>
                <td>
                  {{ bill.confirmed_at || "—"
                  }}<small>{{ bill.paid_at || "—" }}</small>
                </td>
                <td>
                  <button @click="openBill(bill.id)">详情</button
                  ><button
                    @click="download(url(`/settlements/${bill.id}.xlsx`))"
                  >
                    Excel</button
                  ><button
                    @click="download(url(`/settlements/${bill.id}.pdf`))"
                  >
                    PDF
                  </button>
                </td>
              </tr>
              <tr v-if="!data.settlements?.length">
                <td colspan="6">暂无正式结算单，可先下载上方实时账单</td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </template>
    <div v-if="detail" class="mask" @click.self="detail = null">
      <aside class="drawer">
        <header>
          <h3>{{ detail.order_no }}</h3>
          <button @click="detail = null">关闭</button>
        </header>
        <p>
          {{ detail.order_date }} · {{ detail.ship_status }} ·
          {{ accountStatus(detail.payment_status) }}
        </p>
        <p>
          总额 ¥{{ money(detail.total_cents) }}　已付 ¥{{
            money(detail.paid_cents)
          }}　待付 ¥{{ money(detail.due_cents) }}
        </p>
        <table>
          <thead>
            <tr>
              <th>商品 / 规格</th>
              <th>数量</th>
              <th>单价</th>
              <th>金额</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(item, index) in detail.items" :key="index">
              <td>
                {{ item.item_name }}<small>{{ item.spec }}</small>
              </td>
              <td>{{ item.qty }} {{ item.unit }}</td>
              <td>{{ item.unit_price }}</td>
              <td>{{ item.line_total }}</td>
            </tr>
          </tbody>
        </table>
        <h3>收件信息</h3>
        <template v-if="canEditRecipient"
          ><label
            >粘贴完整地址<textarea
              v-model="addressText"
              rows="3"
              placeholder="姓名、电话、详细地址"
            /></label
          ><button
            @click="
              Object.assign(recipient, mergeRecipient(recipient, addressText))
            "
          >
            解析地址
          </button></template
        ><label
          >收件人<input
            v-model.trim="recipient.receiver_name"
            :disabled="!canEditRecipient" /></label
        ><label
          >电话<input
            v-model.trim="recipient.receiver_phone"
            :disabled="!canEditRecipient" /></label
        ><label
          >地址<textarea
            v-model.trim="recipient.receiver_address"
            :disabled="!canEditRecipient"
            rows="3"
          />
        </label>
        <p v-if="!canEditRecipient" class="muted">
          已发货或作废订单不能修改收件信息。
        </p>
        <p v-if="detailError" class="error">{{ detailError }}</p>
        <button
          v-if="canEditRecipient"
          class="primary"
          :disabled="saving"
          @click="saveRecipient"
        >
          {{ saving ? "保存中…" : "保存收件信息" }}
        </button>
      </aside>
    </div>
    <div v-if="billDetail" class="mask" @click.self="billDetail = null">
      <aside class="drawer">
        <header>
          <h3>{{ billDetail.settlement_no }}</h3>
          <button @click="billDetail = null">关闭</button>
        </header>
        <p>
          {{ billDetail.period_from }} 至 {{ billDetail.period_to }} ·
          {{ accountStatus(billDetail.status) }}
        </p>
        <p>金额 ¥{{ money(billDetail.total_cents) }}</p>
        <p v-for="fee in billDetail.fees" :key="fee.id">
          {{ fee.order_no || "独立费用" }} ·
          {{ accountFeeType(fee.fee_type) }} · {{ money(fee.amount_cents) }}
          {{ fee.currency }} · {{ accountStatus(fee.payment_status) }}
        </p>
      </aside>
    </div>
    <div v-if="salesIDs.length" class="mask sales-mask">
      <div class="sales-dialog">
        <SalesOrderView
          :order-id="salesIDs.length === 1 ? salesIDs[0] : 0"
          :order-ids="salesIDs.length > 1 ? salesIDs : []"
          :customer-mode="true"
          :customer-id="customerAccountActor ? 0 : customerContextId"
          embedded
          @close="salesIDs = []"
        />
      </div>
    </div>
  </div>
</template>
<script setup>
import { computed, reactive, ref, watch } from "vue";
import { apiGet, apiSend } from "../api/client";
import { downloadCustomerFile } from "../api/customer-account";
import {
  mergeRecipient,
  togglePageSelection,
  accountStatus,
  accountFeeType,
} from "../lib/customer-account";
import SalesOrderView from "./SalesOrderView.vue";
const props = defineProps({
  portalPage: { type: String, default: "customerOrders" },
  customerAccountActor: Boolean,
  customerContextId: { type: [String, Number], default: 0 },
  customerContextLabel: { type: String, default: "" },
  viewParams: { type: Object, default: () => ({}) },
});
const isOrders = computed(() => props.portalPage === "customerOrders"),
  isStatement = computed(() => props.portalPage === "customerSettlement"),
  heading = computed(() =>
    isOrders.value ? "我的订单" : isStatement.value ? "往来账单" : "订单费用",
  );
const today = new Intl.DateTimeFormat("sv-SE", {
  timeZone: "Asia/Shanghai",
}).format(new Date());
const filters = reactive({
  period: "month",
  anchor: today,
  date_from: "",
  date_to: "",
  q: "",
  pay_status: "",
  ship_status: "",
  include_void: false,
});
const data = ref({ rows: [], summary: {}, page: 1, total_pages: 1 }),
  limit = ref(20),
  jumpPage = ref(1),
  loading = ref(false),
  error = ref(""),
  message = ref(""),
  selected = ref([]),
  selectedNames = reactive({}),
  showSelected = ref(false),
  salesIDs = ref([]),
  detail = ref(null),
  billDetail = ref(null),
  saving = ref(false),
  detailError = ref(""),
  addressText = ref(""),
  recipient = reactive({
    receiver_name: "",
    receiver_phone: "",
    receiver_address: "",
  });
const shipStatuses = [
  "未发货",
  "待发货",
  "部分发货",
  "已发货",
  "已出库",
  "已签收",
  "已收货",
  "已完成",
];
const canEditRecipient = computed(
  () =>
    detail.value &&
    !detail.value.is_void &&
    !["部分发货", "已发货", "已出库", "已签收", "已收货", "已完成"].includes(
      detail.value.ship_status,
    ),
);
const selectable = computed(() => data.value.rows.filter((r) => !r.is_void));
const allSelected = computed(
    () =>
      selectable.value.length > 0 &&
      selectable.value.every((r) => selected.value.includes(r.id)),
  ),
  someSelected = computed(() =>
    selectable.value.some((r) => selected.value.includes(r.id)),
  );
const money = (v) => (Number(v || 0) / 100).toFixed(2);
function url(path, params = {}) {
  const p = new URLSearchParams(params);
  if (!props.customerAccountActor)
    p.set(
      "customer_id",
      String(props.customerContextId || props.viewParams.customer_id || 0),
    );
  return `/api/customer-processing/portal${path}?${p}`;
}
function query() {
  const out = { q: filters.q, pay_status: filters.pay_status };
  if (isStatement.value) {
    out.period = filters.period;
    out.anchor = filters.anchor;
  } else {
    out.date_from = filters.date_from;
    out.date_to = filters.date_to;
  }
  if (isOrders.value) {
    out.ship_status = filters.ship_status;
    out.include_void = String(filters.include_void);
  }
  return out;
}
let sequence = 0;
async function load(page = 1) {
  const n = ++sequence;
  loading.value = true;
  error.value = "";
  try {
    const result = await apiGet(
      url(
        isOrders.value
          ? "/orders"
          : isStatement.value
            ? "/statements"
            : "/order-fees",
        {
          ...query(),
          page: Math.max(1, Number(page) || 1),
          limit: limit.value,
        },
      ),
    );
    if (n !== sequence) return;
    data.value = result;
    jumpPage.value = result.page;
    rememberRows();
  } catch (e) {
    if (n === sequence) {
      error.value = e.message;
      data.value = { rows: [], summary: {}, page: 1, total_pages: 1 };
    }
  } finally {
    if (n === sequence) loading.value = false;
  }
}
function rememberRows() {
  for (const r of data.value.rows) selectedNames[r.id] = r.order_no;
}
function selectPage() {
  selected.value = togglePageSelection(selected.value, data.value.rows);
  rememberRows();
}
async function openDetail(id) {
  error.value = "";
  detailError.value = "";
  try {
    detail.value = await apiGet(url(`/orders/${id}/detail`));
    for (const k of Object.keys(recipient))
      recipient[k] = detail.value[k] || "";
    addressText.value = "";
  } catch (e) {
    error.value = e.message;
  }
}
async function saveRecipient() {
  saving.value = true;
  detailError.value = "";
  try {
    await apiSend(url(`/orders/${detail.value.id}/recipient`), {
      method: "PATCH",
      body: recipient,
    });
    detail.value = null;
    message.value = "收件信息已保存，新生成销售单将使用新地址";
    await load(data.value.page);
  } catch (e) {
    detailError.value = e.message;
  } finally {
    saving.value = false;
  }
}
async function download(link) {
  error.value = "";
  try {
    await downloadCustomerFile(link);
  } catch (e) {
    error.value = e.message;
  }
}
function downloadStatement(format) {
  return download(url(`/statements.${format}`, query()));
}
async function openBill(id) {
  error.value = "";
  try {
    billDetail.value = await apiGet(url(`/settlements/${id}`));
  } catch (e) {
    error.value = e.message;
  }
}
watch(
  () => [
    props.portalPage,
    props.customerContextId,
    props.viewParams.customer_id,
  ],
  () => {
    selected.value = [];
    detail.value = null;
    billDetail.value = null;
    salesIDs.value = [];
    load(1);
  },
  { immediate: true },
);
</script>
<style scoped>
.account-page {
  display: grid;
  gap: 16px;
  color: #243c32;
}
.panel {
  background: #fff;
  border: 1px solid #dce5de;
  border-radius: 12px;
  padding: 20px;
  min-width: 0;
}
header,
.actions,
.pager {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}
h2,
h3 {
  margin: 0 0 10px;
}
p {
  line-height: 1.6;
}
.filters {
  display: flex;
  align-items: end;
  gap: 12px;
  flex-wrap: wrap;
  margin: 18px 0;
}
label {
  display: grid;
  gap: 6px;
  font-size: 14px;
}
.check {
  display: flex;
  align-items: center;
}
input,
select,
textarea {
  border: 1px solid #c8d7cb;
  border-radius: 6px;
  padding: 8px;
  font: inherit;
  max-width: 100%;
}
button {
  border: 1px solid #c8d7cb;
  background: #fff;
  border-radius: 6px;
  padding: 8px 12px;
  cursor: pointer;
  color: inherit;
}
button:disabled {
  opacity: 0.45;
  cursor: default;
}
.primary {
  background: #315d48;
  color: #fff;
}
.metrics {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.metrics article {
  background: #f0f5f1;
  padding: 16px;
  border-radius: 8px;
  display: grid;
  gap: 8px;
}
.metrics strong {
  font-size: 24px;
}
.muted,
small {
  color: #66766e;
  font-size: 13px;
}
small {
  display: block;
  max-width: 300px;
  white-space: normal;
  margin-top: 5px;
}
.table-scroll {
  overflow-x: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}
th,
td {
  padding: 12px 8px;
  border-bottom: 1px solid #e5ece7;
  text-align: left;
  vertical-align: top;
}
th {
  white-space: nowrap;
}
td button {
  font-size: 13px;
  margin: 0 5px 5px 0;
}
.void {
  opacity: 0.55;
}
.pager {
  justify-content: flex-end;
  margin-top: 16px;
  font-size: 14px;
}
.pager input {
  width: 70px;
}
.pager label {
  display: flex;
  align-items: center;
}
.error {
  color: #a52424;
}
.success {
  color: #286640;
}
.selection {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  background: #eef4ef;
  padding: 12px;
}
.mask {
  position: fixed;
  inset: 0;
  background: #142c2470;
  z-index: 120;
  display: flex;
  justify-content: flex-end;
}
.drawer {
  box-sizing: border-box;
  background: #fff;
  width: min(660px, 100%);
  padding: 24px;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.drawer label {
  width: 100%;
}
.sales-mask {
  justify-content: center;
  padding: 20px;
}
.sales-dialog {
  background: #fff;
  width: min(1280px, 100%);
  overflow: auto;
  border-radius: 12px;
}
.help {
  font-size: 14px;
  color: #52685b;
  margin-top: 16px;
}
@media (max-width: 700px) {
  .metrics {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .metrics strong {
    font-size: 20px;
  }
  .panel {
    padding: 14px;
  }
  .filters label {
    flex: 1 1 130px;
  }
  .sales-mask {
    padding: 0;
  }
  .drawer {
    padding: 18px;
  }
}
</style>
