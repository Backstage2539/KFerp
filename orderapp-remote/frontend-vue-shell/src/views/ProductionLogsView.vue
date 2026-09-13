<template>
  <section class="logs-workspace" aria-label="生产日志工作区">
    <ProductionReturnLink :source="viewParams.return_navigation" />
    <header class="workspace-header">
      <div>
        <div class="eyebrow">生产管理 · 完工记录与批次追溯</div>
        <h1>生产日志</h1>
        <p>查看每次生产的实际结果，按批次追溯投料、产出和库存变化。</p>
      </div>
      <div class="header-actions">
        <button type="button" @click="navigate('productionManual')">
          操作说明</button
        ><button type="button" :disabled="loading" @click="refreshLogs">
          刷新记录
        </button>
      </div>
    </header>
    <div class="summary-strip" aria-label="查询结果摘要">
      <div class="summary-mark">录</div>
      <div>
        <strong>{{
          hasLoaded ? `共 ${total} 条完工记录` : "生产完工记录"
        }}</strong>
        <p>{{ appliedRange }} · 按完成时间倒序</p>
      </div>
      <div class="summary-output">
        <span>本页已记录产出</span
        ><strong>{{ hasLoaded ? weight(pageOutput) : "—" }}</strong>
      </div>
    </div>
    <form class="filter-panel" @submit.prevent="queryLogs">
      <label class="search-field"
        ><span>搜索记录</span
        ><input
          v-model.trim="filters.q"
          type="search"
          placeholder="商品、订单号、生产或成品批次"
      /></label>
      <label
        ><span>开始日期</span><input v-model="filters.from" type="date"
      /></label>
      <label
        ><span>结束日期</span><input v-model="filters.to" type="date"
      /></label>
      <label
        ><span>商品</span
        ><SearchableSelect
          v-model="filters.product_id"
          :options="products"
          :option-label="(p) => p.name"
          placeholder="全部商品，可输入搜索"
      /></label>
      <label
        ><span>完成人</span
        ><input v-model.trim="filters.operator" placeholder="员工姓名（完整）"
      /></label>
      <div class="query-actions">
        <button class="primary" type="submit">查询</button
        ><button type="button" @click="resetFilters">清除筛选</button>
      </div>
      <div
        v-if="
          applied.batch_id || applied.running_item_id || applied.work_order_id
        "
        class="context-note"
      >
        {{
          applied.batch_id
            ? `已定位生产批次 ${applied.batch_id}`
            : "已定位来源单据的完工记录"
        }}<button class="text-button" type="button" @click="resetFilters">
          查看全部日志
        </button>
      </div>
      <p class="query-hint" :class="{ warning: filtersChanged }" role="status">
        {{
          filtersChanged
            ? "筛选条件已修改，点击“查询”后生效。"
            : "修改条件后点击查询；刷新记录沿用已查询条件。"
        }}
      </p>
    </form>
    <div v-if="error" class="notice" role="alert">
      {{ error }}<span v-if="hasLoaded"> · 下方保留上次查询结果。</span
      ><button class="text-button" type="button" @click="queryLogs">
        重试查询
      </button>
    </div>
    <div class="workspace-layout">
      <section class="records-panel" :aria-busy="loading">
        <div class="section-heading">
          <h2 ref="listHeading" tabindex="-1">完工记录</h2>
          <span>{{ loading ? "正在查询…" : `第 ${applied.page} 页` }}</span>
        </div>
        <div v-if="loading && !hasLoaded" class="empty-state" role="status">
          正在读取生产日志…
        </div>
        <div v-else-if="!rows.length" class="empty-state">
          <strong>{{
            error ? "暂时无法读取日志" : "没有符合条件的完工记录"
          }}</strong>
          <p>调整日期、商品或完成人后重新查询。</p>
        </div>
        <div v-else class="table-wrap">
          <table class="log-table">
            <thead>
              <tr>
                <th>商品 / 规格</th>
                <th>实际产出</th>
                <th>实际产出率</th>
                <th>完成人 / 时间</th>
                <th>记录</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in rows"
                :key="row.id"
                :class="{ selected: selected?.id === row.id }"
              >
                <td data-label="商品 / 规格">
                  <strong>{{ row.product_name || "未记录商品名称" }}</strong
                  ><span class="subline">{{ spec(row) }}</span
                  ><span class="batch-label">{{
                    row.batch_id || "生产批次未记录"
                  }}</span>
                </td>
                <td data-label="实际产出">
                  <strong :class="{ warning: needsReview(row) }">{{
                    needsReview(row)
                      ? "数量待核对"
                      : weight(row.finished_total_g)
                  }}</strong
                  ><span class="subline">{{ output(row) }}</span>
                </td>
                <td data-label="实际产出率">
                  <span :class="yieldClass(row)">{{
                    needsReview(row) ? "待核对" : yieldText(row)
                  }}</span>
                </td>
                <td data-label="完成人 / 时间">
                  <span>{{ row.finished_by || "未记录" }}</span
                  ><time class="subline">{{
                    row.finished_at || "未记录完成时间"
                  }}</time>
                </td>
                <td class="record-action">
                  <button
                    class="text-button"
                    type="button"
                    :aria-pressed="selected?.id === row.id"
                    :aria-label="`查看记录 ${row.batch_id || row.product_name}`"
                    @click="selectRecord(row)"
                  >
                    查看记录 <span aria-hidden="true">›</span>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <PaginationControls
          :page="applied.page"
          :page-size="applied.limit"
          :total="total"
          :disabled="loading"
          @change="changePage"
        />
      </section>
      <aside class="detail-panel" aria-label="生产记录详情">
        <div class="section-heading">
          <h2 ref="detailHeading" tabindex="-1">记录详情</h2>
          <span class="readonly-tag">只读</span>
        </div>
        <template v-if="selected">
          <div class="record-summary">
            <strong>{{ selected.product_name || "未记录商品名称" }}</strong>
            <p>{{ spec(selected) }}</p>
            <span>{{ selected.batch_id || "生产批次未记录" }}</span>
          </div>
          <section class="detail-section">
            <h3>本次生产结果</h3>
            <dl>
              <div>
                <dt>计划成品</dt>
                <dd>{{ weight(selected.planned_need_g) }}</dd>
              </div>
              <div>
                <dt>实际投料</dt>
                <dd>{{ weight(selected.input_g) }}</dd>
              </div>
              <div>
                <dt>实际产出</dt>
                <dd class="value-strong">
                  {{ weight(selected.finished_total_g) }}
                </dd>
              </div>
              <div>
                <dt>记录完成数量</dt>
                <dd>
                  {{ selected.finished_units ?? "未记录" }}
                  {{ needsReview(selected) ? "（单位待核对）" : "件" }}
                </dd>
              </div>
              <div>
                <dt>散装余料</dt>
                <dd>{{ weight(selected.finished_loose_g) }}</dd>
              </div>
              <div>
                <dt>实际产出率</dt>
                <dd :class="yieldClass(selected)">{{ yieldText(selected) }}</dd>
              </div>
            </dl>
            <p v-if="needsReview(selected)" class="detail-hint warning">
              旧记录有完成数量，但未记录可靠的产出重量和单位。以上保留原值，请核对原始单据。
            </p>
            <p
              v-else-if="!(Number(selected.input_g) > 0)"
              class="detail-hint warning"
            >
              未记录有效投料，产出率待核对。
            </p>
            <p v-else class="detail-hint">
              实际产出率依据本条完工记录，保留当时结果。
            </p>
          </section>
          <section class="detail-section">
            <h3>人员与时间</h3>
            <dl>
              <div>
                <dt>开始人</dt>
                <dd>{{ selected.started_by || "未记录" }}</dd>
              </div>
              <div>
                <dt>开始时间</dt>
                <dd>{{ selected.started_at || "未记录" }}</dd>
              </div>
              <div>
                <dt>完成人</dt>
                <dd>{{ selected.finished_by || "未记录" }}</dd>
              </div>
              <div>
                <dt>完成时间</dt>
                <dd>{{ selected.finished_at || "未记录" }}</dd>
              </div>
            </dl>
          </section>
          <section class="detail-section">
            <h3>库存变化</h3>
            <dl>
              <div>
                <dt>完成前</dt>
                <dd>
                  {{ selected.inventory_units_before ?? "未记录" }} 件<br />{{
                    weight(selected.inventory_loose_g_before)
                  }}
                  散装
                </dd>
              </div>
              <div>
                <dt>完成后</dt>
                <dd>
                  {{ selected.inventory_units_after ?? "未记录" }} 件<br />{{
                    weight(selected.inventory_loose_g_after)
                  }}
                  散装
                </dd>
              </div>
            </dl>
            <p class="detail-hint">这里是完成当时的库存快照。</p>
          </section>
          <section class="detail-section">
            <h3>订单与批次追溯</h3>
            <p class="detail-hint">来源订单</p>
            <p class="wrap-text">{{ selected.order_nos || "未关联订单" }}</p>
            <p class="detail-hint">成品批次</p>
            <button
              v-if="selected.finished_batch_code"
              class="text-button batch-link"
              type="button"
              @click="traceBatch(selected.finished_batch_code)"
            >
              {{ selected.finished_batch_code }}
              <span aria-hidden="true">↗</span>
            </button>
            <p v-else class="warning">历史记录未关联成品批次</p>
          </section>
          <section class="detail-section">
            <h3>
              本次用料
              <span class="muted">{{
                materials.length ? `${materials.length} 项` : ""
              }}</span>
            </h3>
            <ul v-if="materials.length" class="material-list">
              <li v-for="(item, index) in materials" :key="index">
                <strong>{{
                  item.material_name || item.name || "未记录物料名称"
                }}</strong
                ><span>{{ materialQuantity(item) }}</span
                ><button
                  v-if="item.batch_code || item.material_batch_code"
                  type="button"
                  class="text-button batch-link"
                  @click="
                    traceBatch(item.batch_code || item.material_batch_code)
                  "
                >
                  {{ item.batch_code || item.material_batch_code }} ↗</button
                ><small v-else class="warning">物料批次未记录</small>
              </li>
            </ul>
            <p v-else class="warning">
              用料明细待核对，历史记录未提供可读明细。
            </p>
            <details
              v-if="
                !materials.length &&
                selected.material_summary &&
                selected.material_summary !== '[]'
              "
            >
              <summary>查看原始记录</summary>
              <p class="wrap-text">{{ selected.material_summary }}</p>
            </details>
          </section>
          <button class="back-to-list" type="button" @click="returnToList">
            ↑ 返回记录列表
          </button>
        </template>
        <div v-else class="detail-empty">
          <div class="detail-icon" aria-hidden="true">≡</div>
          <strong>选择一条记录</strong>
          <p>查看投料、产出、人员、库存变化及关联批次。</p>
          <p class="detail-hint">
            生产日志记录完工结果；进行中的任务请在工位视图查看。
          </p>
        </div>
      </aside>
    </div>
    <footer class="workspace-footer">
      <span class="readonly-dot" aria-hidden="true"></span
      ><strong>历史完工记录</strong><span>此页用于查看和追溯。</span>
    </footer>
  </section>
</template>

<script setup>
import {
  computed,
  nextTick,
  onMounted,
  onBeforeUnmount,
  reactive,
  ref,
  watch,
} from "vue";
import { apiGet } from "../api/client";
import ProductionReturnLink from "../components/ProductionReturnLink.vue";
import PaginationControls from "../components/PaginationControls.vue";
import SearchableSelect from "../components/SearchableSelect.vue";
import {
  parseProductionMaterialSummary,
  productionLogWeight,
  productionLogYield,
  productionLogOutput,
  productionLogDefaultFilters,
  productionLogFilters,
  productionLogDateError,
  productionLogNeedsReview,
  productionLogMaterialQuantity,
} from "../lib/production-logs";
import { replaceHistoryURL } from "../lib/url-state";

const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
});
const loading = ref(false);
const error = ref("");
const products = ref([]);
const rows = ref([]);
const total = ref(0);
const hasLoaded = ref(false);
const selected = ref(null);
const requestedLogId = ref(0);
const detailHeading = ref(null);
const listHeading = ref(null);
const filters = reactive(productionLogDefaultFilters());
const applied = reactive(productionLogDefaultFilters());
let requestVersion = 0;
let mounted = false;
const weight = productionLogWeight;
const yieldText = productionLogYield;
const output = productionLogOutput;
const needsReview = productionLogNeedsReview;
const materialQuantity = productionLogMaterialQuantity;
const materials = computed(() =>
  parseProductionMaterialSummary(selected.value?.material_summary),
);
const pageOutput = computed(() =>
  rows.value.reduce((sum, row) => sum + Number(row.finished_total_g || 0), 0),
);
const filtersChanged = computed(() =>
  Object.keys(filters).some(
    (key) => !["page", "limit"].includes(key) && filters[key] !== applied[key],
  ),
);
const appliedRange = computed(() =>
  applied.from || applied.to
    ? `${applied.from || "最早记录"} 至 ${applied.to || "至今"}`
    : "全部日期",
);
function spec(row) {
  return Number(row.spec_g) > 0
    ? `${row.spec_g} g / 件`
    : "散装 / 未指定包装规格";
}
function yieldClass(row) {
  return !needsReview(row) &&
    Number(row?.input_g) > 0 &&
    row?.actual_yield_rate != null
    ? "success-text"
    : "warning";
}

function updateUrl() {
  const url = new URL(window.location.href);
  url.searchParams.set("view", "produceLogs");
  for (const key of Object.keys(applied)) {
    if (applied[key]) url.searchParams.set(key, String(applied[key]));
    else url.searchParams.delete(key);
  }
  if (selected.value?.id)
    url.searchParams.set("log_id", String(selected.value.id));
  else url.searchParams.delete("log_id");
  replaceHistoryURL(url);
}
async function loadLogs(query) {
  const revision = ++requestVersion;
  const snapshot = { ...query };
  const dateError = productionLogDateError(snapshot);
  if (dateError) {
    error.value = dateError;
    loading.value = false;
    return;
  }
  loading.value = true;
  error.value = "";
  const selectedId = selected.value?.id || requestedLogId.value;
  try {
    const url = new URL("/api/produce/logs", window.location.origin);
    for (const [key, value] of Object.entries(snapshot))
      if (value) url.searchParams.set(key, String(value));
    const data = await apiGet(url);
    if (revision !== requestVersion) return;
    rows.value = data.rows || [];
    total.value = Math.max(0, Number(data.total ?? rows.value.length));
    products.value = (data.products || []).map((p) => ({
      id: Number(p.id || p.ID || 0),
      name: p.name || p.Name || "",
    }));
    Object.assign(applied, snapshot, {
      page: Number(data.page || snapshot.page),
      limit: Number(data.limit || snapshot.limit),
    });
    selected.value =
      rows.value.find((row) => Number(row.id) === Number(selectedId)) || null;
    requestedLogId.value = 0;
    hasLoaded.value = true;
    updateUrl();
  } catch (err) {
    if (revision === requestVersion)
      error.value = err.message || "日志读取失败，请重试";
  } finally {
    if (revision === requestVersion) loading.value = false;
  }
}
function queryLogs() {
  return loadLogs({ ...filters, page: 1, limit: applied.limit });
}
function refreshLogs() {
  return loadLogs({ ...applied });
}
function changePage({ page, pageSize }) {
  return loadLogs({ ...applied, page, limit: pageSize });
}
function resetFilters() {
  Object.assign(filters, productionLogDefaultFilters());
  requestedLogId.value = 0;
  selected.value = null;
  return queryLogs();
}
async function selectRecord(row) {
  selected.value = row;
  updateUrl();
  await nextTick();
  detailHeading.value?.focus();
}
function returnToList() {
  listHeading.value?.focus();
}
function navigate(key, params = {}) {
  window.dispatchEvent(
    new CustomEvent("kferp:navigate-view", {
      detail: {
        key,
        params,
        returnNavigation: {
          key: "produceLogs",
          label: "返回生产日志",
          params: { ...applied, log_id: selected.value?.id || 0 },
        },
      },
    }),
  );
}
function traceBatch(batch) {
  navigate("warehouseInventory", { batch });
}
function applyProductionContextParams() {
  const urlParams = new URL(window.location.href).searchParams;
  const query = productionLogFilters({
    ...Object.fromEntries(urlParams),
    ...props.viewParams,
  });
  Object.assign(filters, query);
  requestedLogId.value = Number(
    props.viewParams?.log_id || urlParams.get("log_id") || 0,
  );
  return loadLogs(query);
}
function restoreHistory() {
  return applyProductionContextParams();
}
watch(
  () => props.viewParams,
  () => {
    if (mounted) applyProductionContextParams();
  },
  { deep: true },
);
onMounted(() => {
  mounted = true;
  applyProductionContextParams();
  window.addEventListener("popstate", restoreHistory);
});
onBeforeUnmount(() => {
  mounted = false;
  requestVersion++;
  window.removeEventListener("popstate", restoreHistory);
});
</script>

<style scoped>
* {
  box-sizing: border-box;
}
.logs-workspace {
  padding: 24px;
  color: #243c31;
  background: #fff;
  min-height: 100%;
  font-size: 14px;
}
.workspace-header,
.header-actions,
.section-heading,
.summary-strip,
.query-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.workspace-header {
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 22px;
}
.eyebrow {
  font-size: 12px;
  color: #7b8a80;
  letter-spacing: 0.06em;
}
h1 {
  font-size: 28px;
  line-height: 1.3;
  color: #172d22;
  margin: 8px 0;
}
h2 {
  font-size: 17px;
  margin: 0;
  color: #253b2e;
}
h3 {
  font-size: 14px;
  margin: 0 0 12px;
}
p {
  line-height: 1.6;
  margin: 7px 0;
  color: #7a877f;
}
.header-actions {
  flex-shrink: 0;
}
.summary-strip {
  background: #eff8f2;
  border: 1px solid #e0ece4;
  border-radius: 10px;
  padding: 18px;
  margin-bottom: 20px;
}
.summary-mark {
  background: #4d8d64;
  color: #fff;
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  font-weight: 700;
}
.summary-strip strong {
  font-size: 18px;
}
.summary-strip p {
  font-size: 12px;
  margin: 5px 0 0;
}
.summary-output {
  margin-left: auto;
  text-align: right;
  display: grid;
  gap: 4px;
}
.summary-output span {
  font-size: 12px;
  color: #718378;
}
.filter-panel {
  display: grid;
  grid-template-columns: minmax(220px, 2fr) repeat(2, minmax(140px, 1fr));
  gap: 12px 16px;
  border: 1px solid #e1e8e3;
  border-radius: 10px;
  padding: 16px;
  margin-bottom: 20px;
}
label {
  min-width: 0;
}
label > span {
  display: block;
  font-size: 12px;
  color: #6d7b72;
  margin-bottom: 6px;
}
input,
button {
  font: inherit;
}
input {
  width: 100%;
  min-width: 0;
  height: 38px;
  padding: 8px 10px;
  border: 1px solid #d7e0da;
  border-radius: 6px;
  background: #fff;
  color: #283e31;
}
button {
  border: 1px solid #d3ddd6;
  background: #fff;
  color: #395442;
  border-radius: 6px;
  padding: 8px 12px;
  min-height: 36px;
  cursor: pointer;
}
button:hover {
  background: #f1f7f3;
}
button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
button:focus-visible,
input:focus-visible,
summary:focus-visible {
  outline: 2px solid #3d865c;
  outline-offset: 3px;
}
.primary {
  background: #4d8d64;
  color: #fff;
  border-color: #4d8d64;
}
.primary:hover {
  background: #397b51;
}
.query-actions {
  align-self: end;
  justify-content: flex-end;
}
.query-hint,
.context-note {
  grid-column: 1/-1;
  font-size: 12px;
  margin: 0;
}
.context-note {
  display: flex;
  align-items: center;
  gap: 8px;
  background: #f3f7fa;
  padding: 8px;
  color: #45657c;
}
.workspace-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 330px;
  gap: 22px;
  align-items: start;
}
.section-heading {
  justify-content: space-between;
  margin-bottom: 14px;
}
.section-heading > span {
  font-size: 12px;
  color: #829086;
}
.records-panel,
.detail-panel {
  min-width: 0;
}
.detail-panel {
  border-left: 1px solid #e0e7e2;
  padding-left: 22px;
}
.detail-panel h2:focus,
.records-panel h2:focus {
  outline: none;
}
.table-wrap {
  overflow: auto;
  border: 1px solid #e1e8e3;
  border-radius: 8px;
}
.log-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}
.log-table th {
  background: #f3f6f4;
  color: #64756b;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  padding: 12px 10px;
}
.log-table td {
  padding: 14px 10px;
  border-top: 1px solid #e5ece7;
  vertical-align: top;
  line-height: 1.5;
  overflow-wrap: anywhere;
}
.log-table td:first-child {
  width: 31%;
}
.log-table strong {
  font-size: 13px;
  color: #263c2e;
}
.log-table .selected {
  background: #eef8f1;
}
.subline {
  display: block;
  font-size: 12px;
  color: #879188;
  margin-top: 4px;
}
.batch-label {
  display: block;
  font-size: 11px;
  color: #789081;
  line-height: 1.5;
  margin-top: 5px;
  overflow-wrap: anywhere;
}
.text-button {
  color: #287daf;
  background: none;
  border: 0;
  min-height: 30px;
  padding: 3px 0;
  text-align: left;
  font-size: 13px;
}
.text-button:hover {
  color: #16597e;
  background: none;
  text-decoration: underline;
}
.record-action {
  white-space: nowrap;
}
.success-text {
  color: #27824d;
  font-weight: 600;
}
.warning {
  color: #b8791c !important;
}
.muted {
  color: #8a968d;
  font-size: 12px;
  font-weight: 400;
}
.empty-state {
  border: 1px dashed #d9e3dc;
  border-radius: 8px;
  text-align: center;
  padding: 48px 20px;
  color: #849087;
}
.empty-state strong {
  display: block;
  color: #53695b;
}
.detail-empty {
  text-align: center;
  padding: 36px 14px;
  color: #7c8c82;
  border: 1px dashed #d8e4dc;
  border-radius: 8px;
}
.detail-empty strong {
  color: #576d5e;
}
.detail-icon {
  margin: 0 auto 18px;
  width: 48px;
  height: 48px;
  background: #eff6f1;
  border-radius: 12px;
  color: #6a8b76;
  font-size: 28px;
  line-height: 48px;
}
.readonly-tag {
  border: 1px solid #dce7df;
  background: #f2f7f4;
  border-radius: 4px;
  padding: 3px 7px;
}
.record-summary {
  padding: 14px;
  background: #eff8f2;
  border-radius: 8px;
  line-height: 1.6;
  overflow-wrap: anywhere;
}
.record-summary > strong {
  font-size: 16px;
}
.record-summary > span {
  font-size: 12px;
  color: #789082;
}
.detail-section {
  border-bottom: 1px solid #e7ece8;
  padding: 18px 0;
}
dl {
  margin: 0;
}
dl > div {
  display: flex;
  justify-content: space-between;
  gap: 14px;
  margin: 10px 0;
}
dt {
  color: #7c8b80;
  flex-shrink: 0;
  font-size: 12px;
}
dd {
  margin: 0;
  text-align: right;
  color: #42584a;
  font-size: 13px;
  overflow-wrap: anywhere;
}
.value-strong {
  font-weight: 700;
  color: #27824d;
}
.detail-hint {
  font-size: 12px;
  color: #8a968e;
  line-height: 1.6;
}
.wrap-text,
.batch-link {
  overflow-wrap: anywhere;
  white-space: pre-wrap;
  max-width: 100%;
}
.material-list {
  padding: 0;
  margin: 0;
  list-style: none;
}
.material-list li {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 5px 10px;
  margin: 0 0 16px;
}
.material-list strong {
  font-weight: 500;
}
.material-list .batch-link,
.material-list small {
  grid-column: 1/-1;
  font-size: 12px;
}
.back-to-list {
  margin-top: 18px;
  width: 100%;
}
.notice {
  padding: 12px 14px;
  color: #ad6c16;
  background: #fff7e9;
  border: 1px solid #efd9b2;
  border-radius: 8px;
  margin-bottom: 16px;
}
.notice button {
  margin-left: 12px;
}
.workspace-footer {
  margin-top: 24px;
  background: rgba(255, 255, 255, 0.97);
  border-top: 1px solid #e3e9e5;
  padding: 16px 24px;
  display: flex;
  gap: 10px;
  align-items: center;
  color: #7d8c81;
  font-size: 12px;
}
.workspace-footer strong {
  color: #4c6755;
}
.readonly-dot {
  width: 8px;
  height: 8px;
  background: #69a17c;
  border-radius: 50%;
}
:deep(.list-pagination-controls) {
  font-size: 12px;
  gap: 8px;
}
:deep(.pagination-summary) {
  font-size: 12px;
}
:deep(.pagination-actions) {
  gap: 5px;
}
:deep(.pagination-actions button) {
  font-size: 12px;
  padding: 6px 8px;
}
:deep(.searchable-select input) {
  height: 38px;
}
@media (max-width: 1200px) {
  .workspace-layout {
    grid-template-columns: minmax(0, 1fr) 290px;
    gap: 16px;
  }
  .detail-panel {
    padding-left: 16px;
  }
  .logs-workspace {
    padding: 20px 18px 90px;
  }
  .log-table th,
  .log-table td {
    padding: 10px 7px;
    font-size: 12px;
  }
}
@media (max-width: 1000px) {
  .workspace-layout {
    grid-template-columns: 1fr;
  }
  .detail-panel {
    border: 1px solid #e0e7e2;
    border-radius: 10px;
    padding: 18px;
  }
  .detail-empty {
    padding: 20px;
  }
  .workspace-footer {
    left: 0;
  }
  .header-actions {
    flex-wrap: wrap;
  }
  .filter-panel {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .search-field {
    grid-column: 1/-1;
  }
}
@media (max-width: 620px) {
  .logs-workspace {
    padding: 16px 12px 100px;
  }
  .workspace-header {
    flex-direction: column;
    margin-bottom: 16px;
  }
  h1 {
    font-size: 25px;
  }
  .summary-strip {
    padding: 13px;
    gap: 10px;
    flex-wrap: wrap;
  }
  .summary-strip strong {
    font-size: 16px;
  }
  .summary-output {
    padding-left: 52px;
    width: 100%;
    margin: 0;
    text-align: left;
    display: flex;
    gap: 10px;
    align-items: baseline;
  }
  .filter-panel {
    padding: 12px;
    gap: 12px;
  }
  .query-actions {
    grid-column: 1/-1;
    justify-content: stretch;
  }
  .query-actions button {
    flex: 1;
  }
  .context-note {
    display: block;
  }
  .log-table,
  .log-table tbody,
  .log-table tr,
  .log-table td {
    display: block;
    width: 100% !important;
  }
  .log-table thead {
    display: none;
  }
  .table-wrap {
    overflow: visible;
    border: 0;
  }
  .log-table tr {
    border: 1px solid #dce6df;
    border-radius: 9px;
    margin-bottom: 12px;
    padding: 12px;
  }
  .log-table td {
    border: 0;
    padding: 5px 0;
    display: flex;
    justify-content: space-between;
    gap: 10px;
  }
  .log-table td:first-child {
    display: block;
  }
  .log-table td:not(:first-child):not(.record-action)::before {
    content: attr(data-label);
    color: #88968d;
    font-size: 12px;
    margin-right: auto;
  }
  .log-table td[data-label="实际产出"],
  .log-table td[data-label="完成人 / 时间"] {
    flex-wrap: wrap;
  }
  .log-table td .subline {
    margin-top: 0;
  }
  .record-action {
    justify-content: flex-end;
  }
  .workspace-footer {
    padding: 14px 12px;
    flex-wrap: wrap;
    gap: 7px;
  }
  .detail-panel {
    padding: 14px;
  }
  .notice {
    font-size: 12px;
  }
  .header-actions button {
    font-size: 12px;
  }
}
</style>
