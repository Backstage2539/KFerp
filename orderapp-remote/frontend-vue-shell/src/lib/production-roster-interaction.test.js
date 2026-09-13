import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import vm from "node:vm";
import { computed, reactive, ref } from "vue";
import * as roster from "./production-roster.js";
import * as capacity from "./workstation-capacity-costing.js";
import { formatLocalDateInput } from "./local-date.js";

function harness(
  apiSend = async () => ({}),
  name = "ProductionScheduleView",
  apiGet = async () => ({}),
) {
  const source = readFileSync(
    new URL(`../views/${name}.vue`, import.meta.url),
    "utf8",
  );
  const script = source
    .split("<script setup>")[1]
    .split("</script>")[0]
    .replace(/^import[\s\S]*?from ['"][^'"]+['"];?$/gm, "");
  const state = {
    computed,
    reactive,
    ref,
    ...roster,
    ...capacity,
    formatLocalDateInput,
    defineProps: () => ({ viewParams: {} }),
    onMounted: () => {},
    onBeforeUnmount: () => {},
    apiGet,
    apiSend,
    window: {
      addEventListener() {},
      removeEventListener() {},
      confirm: () => true,
      dispatchEvent() {},
    },
    crypto: { randomUUID: () => "test-request" },
    setTimeout,
    clearTimeout,
    console,
  };
  const context = vm.createContext(state);
  vm.runInContext(script, context);
  return { run: (code) => vm.runInContext(code, context), source };
}
const base = () => ({
  week_start: "2026-09-14",
  version: 1,
  employees: [
    { id: 1, name: "A" },
    { id: 2, name: "B" },
    { id: 3, name: "C" },
  ],
  entries: [
    { employee_id: 1, work_date: "2026-09-14", status: "working" },
    { employee_id: 2, work_date: "2026-09-14", status: "working" },
    { employee_id: 3, work_date: "2026-09-14", status: "working" },
  ],
  overrides: [],
  assignments: [
    {
      workstation_id: 1,
      work_date: "2026-09-14",
      employee_id: 1,
      employee_name: "A",
      candidates: [
        { employee_id: 1, employee_name: "A", role: "primary" },
        { employee_id: 2, employee_name: "B", role: "backup" },
        { employee_id: 3, employee_name: "C", role: "other" },
      ],
    },
  ],
  issues: [],
});

test("attendance edit clears stale success and starts a fresh preview", async () => {
  const h = harness(async () => ({
    ...base(),
    assignments: [
      {
        workstation_id: 1,
        work_date: "2026-09-14",
        employee_id: 2,
        employee_name: "B",
        source: "backup",
      },
    ],
  }));
  h.run(
    `applyWeek(${JSON.stringify(base())}); message.value='已保存'; cycleEntry(1,'2026-09-14')`,
  );
  assert.equal(h.run("message.value"), "");
  await h.run("refreshPreview()");
  assert.equal(h.run("assignmentFor(1,'2026-09-14').employee_name"), "B");
  assert.equal(h.run("dirty.value"), true);
  h.run("cancelPreviewTimer()");
});
test("older preview response cannot overwrite a newer edit", async () => {
  const pending = [];
  const h = harness(() => new Promise((resolve) => pending.push(resolve)));
  h.run(`applyWeek(${JSON.stringify(base())}); entries['1:2026-09-14']='off'`);
  const first = h.run("refreshPreview()");
  h.run("entries['1:2026-09-14']='working'; invalidatePreview(false)");
  const second = h.run("refreshPreview()");
  pending[1]({
    ...base(),
    assignments: [
      { workstation_id: 1, work_date: "2026-09-14", employee_name: "new" },
    ],
  });
  await second;
  pending[0]({
    ...base(),
    assignments: [
      { workstation_id: 1, work_date: "2026-09-14", employee_name: "old" },
    ],
  });
  await first;
  assert.equal(h.run("assignmentFor(1,'2026-09-14').employee_name"), "new");
});
test("save failure keeps input and requires a fresh review after conflict", async () => {
  const h = harness(async () => {
    throw Object.assign(new Error("范围已变化"), { code: "preview_changed" });
  });
  h.run(
    `applyWeek(${JSON.stringify(base())}); entries['1:2026-09-14']='off'; preview.value={preview_fingerprint:'p'};reviewed.value=true`,
  );
  await h.run("saveRoster()");
  assert.equal(h.run("entryStatus(1,'2026-09-14')"), "off");
  assert.equal(h.run("reviewed.value"), false);
});
test("manual candidate selection includes working employees outside the station roster", () => {
  const h = harness();
  h.run(
    `applyWeek(${JSON.stringify(base())}); openOwnerEditor({id:1,name:'工位'},'2026-09-14')`,
  );
  assert.equal(
    h.run("ownerCandidates.value.some(p=>p.employee_id===3&&p.role==='other')"),
    true,
  );
  assert.match(h.source, /调整负责人/);
  assert.match(h.source, /整日换人/);
});

test("selecting a backup immediately joins the ordered saved list and survives reread", async () => {
  let persisted = {
    id: 7,
    name: "工位A",
    status: "active",
    primary_employee_id: 1,
    backup_employee_ids: [],
    applicable_operation_ids: [],
  };
  const h = harness(
    async (path, options) => {
      persisted = JSON.parse(JSON.stringify(options.body));
      return persisted;
    },
    "ManufacturingWorkstationsView",
    async (path) =>
      path === "/api/manufacturing-workstations"
        ? { rows: [persisted] }
        : path === "/api/company/employees"
          ? {
              rows: [
                { id: 1, name: "A" },
                { id: 2, name: "B" },
                { id: 3, name: "C" },
              ],
            }
          : { rows: [] },
  );
  h.run(
    `editWorkstation(${JSON.stringify(persisted)});newBackupEmployeeID.value=2;addBackup();newBackupEmployeeID.value=3;addBackup();moveBackup(1,-1)`,
  );
  assert.equal(h.run("newBackupEmployeeID.value"), 0);
  await h.run("saveWorkstation()");
  assert.deepEqual(persisted.backup_employee_ids, [3, 2]);
  h.run("newWorkstation()");
  await h.run("loadWorkstations()");
  h.run("editWorkstation(workstations.value[0])");
  assert.equal(h.run('form.backup_employee_ids.join(",")'), "3,2");
  assert.match(h.source, /@change="addBackup"/);
  assert.doesNotMatch(h.source, />添加替补</);
});

test('weekly handover review counts a running task once across dates',()=>{
 const h=harness(); const data=base();data.assignments=[{workstation_id:1,work_date:'2026-09-14',handover_count:1,handover_task_ids:[99]},{workstation_id:1,work_date:'2026-09-15',handover_count:1,handover_task_ids:[99]}];h.run(`applyWeek(${JSON.stringify(data)})`);assert.equal(h.run('handoverCount.value'),1)
})
