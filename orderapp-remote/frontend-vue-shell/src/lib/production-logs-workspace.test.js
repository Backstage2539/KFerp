import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import { computed, reactive, ref, watch, nextTick } from "vue";
import * as helpers from "./production-logs.js";
import { viewNavigationURL } from "./url-state.js";

test("log quantities keep units, zero output and missing yield distinct", () => {
  assert.equal(helpers.productionLogWeight(1749), "1.749 kg");
  assert.equal(helpers.productionLogWeight(0), "0 g");
  assert.equal(
    helpers.productionLogYield({ input_g: 0, actual_yield_rate: 0 }),
    "待核对",
  );
  assert.equal(
    helpers.productionLogYield({ input_g: 1000, actual_yield_rate: 0 }),
    "0.00%",
  );
  assert.equal(
    helpers.productionLogOutput({
      spec_g: 454,
      finished_units: 2,
      finished_loose_g: 10,
      finished_total_g: 918,
    }),
    "2 件 · 余料 10 g",
  );
  assert.equal(
    helpers.productionLogOutput({
      spec_g: 0,
      finished_units: 0,
      finished_total_g: 4000,
    }),
    "散装产出",
  );
  const legacy = {
    spec_g: 0,
    finished_units: 23,
    finished_total_g: 0,
    input_g: 23000,
    actual_yield_rate: 1,
  };
  assert.equal(helpers.productionLogNeedsReview(legacy), true);
  assert.equal(helpers.productionLogOutput(legacy), "23（历史单位待核对）");
  assert.equal(
    helpers.productionLogMaterialQuantity({ deduct_g: 1000, unit: "kg" }),
    "1 kg",
  );
  assert.equal(
    helpers.productionLogMaterialQuantity({ deduct_units: 2, unit: "个" }),
    "2 个",
  );
});

function harness(apiGet) {
  const source = readFileSync(
    new URL("../views/ProductionLogsView.vue", import.meta.url),
    "utf8",
  );
  const script = source
    .split("<script setup>")[1]
    .split("</script>")[0]
    .replace(/^import[\s\S]*?from ['"][^'"]+['"];?$/gm, "");
  const context = vm.createContext({
    computed,
    reactive,
    ref,
    watch,
    nextTick,
    ...helpers,
    defineProps: () => ({ viewParams: {} }),
    onMounted() {},
    onBeforeUnmount() {},
    apiGet,
    URL,
    URLSearchParams,
    replaceHistoryURL() {},
    window: {
      location: {
        href: "http://localhost/vue-shell?view=produceLogs",
        origin: "http://localhost",
      },
      addEventListener() {},
      removeEventListener() {},
      dispatchEvent() {},
    },
    CustomEvent: class {
      constructor(type, data) {
        this.detail = data.detail;
      }
    },
  });
  vm.runInContext(script, context);
  return { run: (code) => vm.runInContext(code, context), source };
}

test("query is explicit; paging uses applied filters and errors preserve the last result", async () => {
  const calls = [];
  const h = harness(async (url) => {
    calls.push(String(url));
    if (calls.length === 3) throw new Error("network failure");
    return {
      rows: [{ id: 1, product_name: "coffee" }],
      total: 31,
      page: 1,
      limit: 20,
    };
  });
  h.run("filters.operator='A'");
  await h.run("queryLogs()");
  h.run("filters.operator='B'");
  await h.run("changePage({page:2,pageSize:20})");
  assert.equal(new URL(calls[1]).searchParams.get("operator"), "A");
  assert.equal(new URL(calls[1]).searchParams.get("page"), "2");
  await h.run("queryLogs()");
  assert.equal(h.run("rows.value[0].id"), 1);
  assert.equal(h.run("filters.operator"), "B");
  assert.match(h.run("error.value"), /network failure/);
});

test("late requests cannot replace newer log results", async () => {
  const pending = [];
  const h = harness((url) => new Promise((resolve) => pending.push(resolve)));
  h.run("filters.operator='A'");
  const old = h.run("queryLogs()");
  h.run("filters.operator='B'");
  const newer = h.run("queryLogs()");
  pending[1]({ rows: [{ id: 2 }], total: 1, page: 1, limit: 20 });
  await newer;
  pending[0]({ rows: [{ id: 1 }], total: 1, page: 1, limit: 20 });
  await old;
  assert.equal(h.run("rows.value[0].id"), 2);
});

test("log workspace has one navigation, responsive details and read-only actions", () => {
  const h = harness(async () => ({ rows: [] }));
  assert.doesNotMatch(h.source, /ProductionTopNav|min-width: 1680px/);
  assert.match(h.source, /ProductionReturnLink/);
  assert.match(h.source, /PaginationControls/);
  assert.doesNotMatch(h.source, /\.workspace-footer\s*\{\s*position: fixed/, "read-only footer must not cover paging buttons");
  assert.match(h.source, /查看记录/);
  assert.match(h.source, /returnNavigation/);
  assert.match(h.source, /aria-label="生产记录详情"/);
});

test("leaving logs does not leak its query or selected record to other pages", () => {
  const url = viewNavigationURL(
    new URL(
      "https://example.test/vue-shell?view=produceLogs&log_id=7&batch_id=PB-1&operator=A",
    ),
    "warehouseInventory",
    { batch: "FP-1" },
  );
  for (const key of ["log_id", "batch_id", "operator"])
    assert.equal(url.searchParams.has(key), false);
  assert.equal(url.searchParams.get("batch"), "FP-1");
});

test("batch trace navigation carries filters and selected record for return", async () => {
  const h = harness(async () => ({
    rows: [{ id: 8, batch_id: "PB-8" }],
    total: 1,
    page: 1,
    limit: 20,
  }));
  h.run("filters.q='PB-8'");
  await h.run("queryLogs()");
  await h.run("selectRecord(rows.value[0])");
  h.run("window.dispatchEvent = event => { window.lastEvent = event }");
  h.run("traceBatch('FP-8')");
  assert.equal(h.run("window.lastEvent.detail.key"), "warehouseInventory");
  assert.equal(h.run("window.lastEvent.detail.params.batch"), "FP-8");
  assert.equal(
    h.run("window.lastEvent.detail.returnNavigation.params.log_id"),
    8,
  );
  assert.equal(
    h.run("window.lastEvent.detail.returnNavigation.params.q"),
    "PB-8",
  );
});
