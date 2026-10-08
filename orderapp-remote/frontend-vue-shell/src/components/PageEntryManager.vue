<script setup>
import { computed, onMounted, onBeforeUnmount, ref, watch } from "vue";
import { apiGet, apiSend, apiFetch } from "../api/client";
import {
  newPageDraft,
  pageLinks,
  groupPageTargets,
  movePageBlock,
  pageKinds,
  pageState,
} from "../lib/page-entries";
import PageContent from "./PageContent.vue";
const props = defineProps({
  viewParams: { type: Object, default: () => ({}) },
});
const base = "/api/admin/page-entries";
const rows = ref([]),
  targets = ref([]),
  functions = ref([]),
  history = ref([]),
  search = ref(""),
  kind = ref(""),
  state = ref(""),
  section = ref("pages"),
  busy = ref(false),
  error = ref(""),
  notice = ref(""),
  selected = ref(null),
  draft = ref(newPageDraft()),
  preview = ref(null),
  images = ref({}),
  references = ref(null),
  deleteEntry = ref(null),
  dirty = ref(false);
const tables = computed(() => groupPageTargets(targets.value));
const tableScope = ref("");
const versions = computed(
  () =>
    tables.value.find((t) => t.table_scope === tableScope.value)?.versions ||
    [],
);
const filtered = computed(() =>
  rows.value.filter(
    (e) =>
      (!search.value ||
        e.draft.name.toLowerCase().includes(search.value.toLowerCase())) &&
      (!kind.value || e.draft.kind === kind.value) &&
      (!state.value ||
        (state.value === "published"
          ? e.enabled
          : state.value === "draft"
            ? e.has_draft
            : !e.enabled && e.published)),
  ),
);
function releasePreview() {
  for (const url of Object.values(images.value)) URL.revokeObjectURL(url);
  images.value = {};
  preview.value = null;
}
async function run(fn) {
  busy.value = true;
  error.value = "";
  notice.value = "";
  try {
    await fn();
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function load() {
  await run(async () => {
    const [a, b, c] = await Promise.all([
      apiGet(base),
      apiGet(base + "/targets"),
      apiGet(base + "/history"),
    ]);
    rows.value = a.rows;
    targets.value = b.rows;
    functions.value = b.functions;
    history.value = c.rows;
  });
}
function edit(e) {
  releasePreview();
  selected.value = e;
  draft.value = JSON.parse(JSON.stringify(e.draft));
  tableScope.value =
    targets.value.find((t) => t.id === draft.value.publication_id)
      ?.table_scope || "";
  dirty.value = false;
  deleteEntry.value = null;
}
function add() {
  releasePreview();
  selected.value = { key: "", revision: 0 };
  draft.value = newPageDraft();
  const prefill = targets.value.find(
    (t) => t.id === Number(props.viewParams.publication_id),
  );
  if (prefill) {
    draft.value.name = prefill.name;
    draft.value.publication_id = prefill.id;
    tableScope.value = prefill.table_scope;
  } else tableScope.value = "";
  dirty.value = true;
}
async function save() {
  await run(async () => {
    const saved = await apiSend(
      base + (selected.value.key ? "/" + selected.value.key : ""),
      {
        method: selected.value.key ? "PUT" : "POST",
        body: { draft: draft.value, revision: selected.value.revision },
      },
    );
    edit(saved);
    rows.value = (await apiGet(base)).rows;
    notice.value = "草稿已保存，预览后点击发布才对外生效。";
  });
}
async function showPreview() {
  await run(async () => {
    releasePreview();
    const data = await apiGet(base + "/" + selected.value.key + "/preview");
    const blobs = {};
    try {
      for (const block of data.document.blocks || []) {
        if (block.kind !== "image") continue;
        const res = await apiFetch(
          base + "/" + selected.value.key + "/images/" + block.asset_id,
        );
        if (!res.ok) throw new Error("预览图片无法加载");
        blobs[block.asset_id] = URL.createObjectURL(await res.blob());
      }
      images.value = blobs;
      preview.value = data;
    } catch (e) {
      Object.values(blobs).forEach(URL.revokeObjectURL);
      throw e;
    }
  });
}
async function change(e, action) {
  await run(async () => {
    const saved = await apiSend(
      base + "/" + e.key + (action === "delete" ? "" : "/" + action),
      {
        method: action === "delete" ? "DELETE" : "POST",
        body: { revision: e.revision },
      },
    );
    if (selected.value?.key === e.key) {
      if (action === "delete") {
        selected.value = null;
        releasePreview();
      } else edit(saved);
    }
    rows.value = (await apiGet(base)).rows;
    deleteEntry.value = null;
    notice.value =
      action === "publish"
        ? "页面已发布。"
        : action === "disable"
          ? "页面已停用。"
          : "页面已删除，旧标识不会复用。";
  });
}
async function askDelete(e) {
  await run(async () => {
    references.value = (await apiGet(base + "/" + e.key + "/references")).rows;
    deleteEntry.value = e;
  });
}
async function upload(event) {
  const file = event.target.files?.[0];
  if (!file) return;
  await run(async () => {
    const body = new FormData();
    body.append("file", file);
    const result = await apiSend(base + "/" + selected.value.key + "/images", {
      body,
    });
    draft.value.blocks.push({
      kind: "image",
      asset_id: result.asset_id,
      caption: "",
    });
    dirty.value = true;
  });
  event.target.value = "";
}
async function copy(e, type) {
  await run(async () => {
    await navigator.clipboard.writeText(
      pageLinks(e, window.location.origin)[type],
    );
    notice.value = "已复制" + (type === "mini" ? "小程序路径" : "网页链接");
  });
}
function changed() {
  dirty.value = true;
  releasePreview();
}
function addBlock(kind) {
  draft.value.blocks.push({ kind, text: "" });
  changed();
}
function move(index, delta) {
  draft.value.blocks = movePageBlock(draft.value.blocks, index, delta);
  changed();
}
function help() {
  window.dispatchEvent(
    new CustomEvent("kferp:navigate-view", {
      detail: {
        key: "pageEntryManual",
        returnNavigation: {
          key: "uiSettings",
          params: {
            tab: "pages",
            entry: selected.value?.key || "",
            section: section.value,
          },
          label: "返回页面入口管理",
        },
      },
    }),
  );
}
async function disableLegacy(e) {
  await run(async () => {
    await apiSend(base + "/history/" + e.key + "/disable", { body: {} });
    history.value = (await apiGet(base + "/history")).rows;
  });
}
onMounted(async () => {
  await load();
  section.value = props.viewParams.section === "history" ? "history" : "pages";
  const e = rows.value.find((e) => e.key === props.viewParams.entry);
  if (e) edit(e);
  else if (props.viewParams.publication_id) add();
});
watch(
  () => props.viewParams.entry,
  (key) => {
    const e = rows.value.find((e) => e.key === key);
    if (e) edit(e);
  },
);
onBeforeUnmount(releasePreview);
</script>
<template>
  <section class="manager">
    <header>
      <div>
        <h2>页面入口管理</h2>
        <p>手工创建固定入口；保存草稿后预览，发布后对外生效。</p>
      </div>
      <div class="actions">
        <button :disabled="busy || dirty" @click="help">操作说明</button
        ><button :disabled="busy" @click="load">刷新</button
        ><button class="primary" :disabled="busy" @click="add">新增页面</button>
      </div>
    </header>
    <p v-if="error" role="alert" class="error">{{ error }}</p>
    <p v-if="notice" role="status" class="success">{{ notice }}</p>
    <nav>
      <button
        :class="{ active: section === 'pages' }"
        @click="section = 'pages'"
      >
        手工页面</button
      ><button
        :class="{ active: section === 'history' }"
        @click="section = 'history'"
      >
        历史入口
      </button>
    </nav>
    <template v-if="section === 'pages'">
      <div class="filters">
        <input
          v-model="search"
          aria-label="搜索名称"
          placeholder="搜索页面名称"
        /><select v-model="kind" aria-label="页面类型">
          <option value="">全部类型</option>
          <option v-for="(label, id) in pageKinds" :key="id" :value="id">
            {{ label }}
          </option></select
        ><select v-model="state" aria-label="发布状态">
          <option value="">全部状态</option>
          <option value="published">已发布</option>
          <option value="draft">有草稿</option>
          <option value="disabled">已停用</option>
        </select>
      </div>
      <p v-if="!filtered.length" class="empty">
        暂无页面入口。点击“新增页面”添加需要的价格表、功能页或图文页。
      </p>
      <div class="table-wrap">
        <table v-if="filtered.length">
          <thead>
            <tr>
              <th>名称</th>
              <th>类型 / 目标</th>
              <th>可见范围</th>
              <th>发布状态</th>
              <th>更新时间</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="e in filtered" :key="e.key">
              <td data-label="名称">{{ e.draft.name }}</td>
              <td data-label="类型 / 目标">
                {{ pageKinds[e.draft.kind]
                }}<small>{{
                  e.draft.kind === "price"
                    ? targets.find((t) => t.id === e.draft.publication_id)
                        ?.name +
                      " · " +
                      (targets.find((t) => t.id === e.draft.publication_id)
                        ?.version || "不可用")
                    : functions.find((f) => f.key === e.draft.target)?.name
                }}</small>
              </td>
              <td data-label="可见范围">{{ e.draft.visibility === "public" ? "公开" : "需认证" }}</td>
              <td data-label="发布状态">{{ pageState(e) }}</td>
              <td data-label="更新时间">{{ new Date(e.updated_at).toLocaleString() }}</td>
              <td data-label="操作" class="actions">
                <button :disabled="busy" @click="edit(e)">编辑 / 预览</button
                ><button @click="copy(e, 'mini')">复制路径</button
                ><button v-if="pageLinks(e).web" @click="copy(e, 'web')">
                  复制网页</button
                ><button
                  v-if="e.enabled"
                  :disabled="busy"
                  @click="change(e, 'disable')"
                >
                  停用</button
                ><button :disabled="busy" @click="askDelete(e)">删除</button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
    <template v-else
      ><p>
        历史自动入口保留原路径、内容和权限，仅支持查看和停用，不进入新菜单选择器。
      </p>
      <div v-for="e in history" :key="e.key" class="history">
        <strong>{{ e.name }}</strong
        ><span
          >{{ e.table_name }} · {{ e.version || "未配置" }} ·
          {{ e.enabled ? "已启用" : "已停用" }} ·
          {{ e.visibility === "public" ? "公开" : "需认证" }}</span
        ><code>{{ e.page_path }}</code
        ><button :disabled="busy || !e.enabled" @click="disableLegacy(e)">
          停用
        </button>
      </div>
      <p v-if="!history.length">暂无历史入口</p></template
    >
    <section v-if="selected && section === 'pages'" class="editor">
      <header>
        <h3>{{ selected.key ? "编辑页面" : "新增页面" }}</h3>
        <button
          @click="
            selected = null;
            releasePreview();
          "
        >
          关闭
        </button>
      </header>
      <div class="editor-grid">
        <div @input="changed" @change="changed">
          <label>页面名称<input v-model="draft.name" maxlength="100" /></label
          ><label
            >页面类型<select v-model="draft.kind">
              <option v-for="(label, id) in pageKinds" :key="id" :value="id">
                {{ label }}
              </option>
            </select></label
          ><label
            >可见范围<select v-model="draft.visibility">
              <option value="authenticated">需认证</option>
              <option value="public">公开</option>
            </select></label
          >
          <template v-if="draft.kind === 'price'"
            ><label
              >价格表<select
                v-model="tableScope"
                @change="draft.publication_id = 0"
              >
                <option value="">请选择实际价格表</option>
                <option
                  v-for="table in tables"
                  :key="table.table_scope"
                  :value="table.table_scope"
                >
                  {{ table.type_name }} / {{ table.name
                  }}{{
                    table.owner_type === "customer"
                      ? "（客户专属 " + table.owner_key + "）"
                      : ""
                  }}
                </option>
              </select></label
            ><label
              >发布版本<select v-model.number="draft.publication_id">
                <option :value="0">请选择指定版本</option>
                <option v-for="v in versions" :key="v.id" :value="v.id">
                  {{ v.version }} · {{ v.name }}
                </option>
              </select></label
            >
            <p>新版本不会自动替换。客户专属表必须选择“需认证”。</p></template
          >
          <label v-if="draft.kind === 'function'"
            >功能页<select v-model="draft.target">
              <option value="">请选择功能</option>
              <option v-for="f in functions" :key="f.key" :value="f.key">
                {{ f.name }}
              </option>
            </select></label
          >
          <template v-if="draft.kind === 'article'"
            ><div
              v-for="(block, index) in draft.blocks"
              :key="index"
              class="block"
            >
              <label
                >{{
                  block.kind === "heading"
                    ? "标题"
                    : block.kind === "text"
                      ? "文字段落"
                      : "图片说明"
                }}<textarea
                  v-if="block.kind !== 'image'"
                  v-model="block.text"
                  :rows="block.kind === 'text' ? 4 : 2" /><input
                  v-else
                  v-model="block.caption"
              /></label>
              <div class="actions">
                <button :disabled="index === 0" @click="move(index, -1)">
                  上移</button
                ><button
                  :disabled="index === draft.blocks.length - 1"
                  @click="move(index, 1)"
                >
                  下移</button
                ><button
                  @click="
                    draft.blocks.splice(index, 1);
                    changed();
                  "
                >
                  删除此项
                </button>
              </div>
            </div>
            <div class="actions">
              <button @click="addBlock('heading')">插入标题</button
              ><button @click="addBlock('text')">插入段落</button
              ><label class="file"
                >插入图片<input
                  type="file"
                  accept="image/png,image/jpeg,image/gif,image/webp"
                  :disabled="!selected.key || busy"
                  @change.stop="upload"
              /></label>
            </div>
            <p v-if="!selected.key">先保存草稿，即可上传图片。</p></template
          >
        </div>
        <div>
          <p v-if="selected.key" class="path">
            固定路径：{{ pageLinks(selected).mini }}
          </p>
          <p>编辑不会改变已发布内容。请先保存草稿，再预览和发布。</p>
          <div class="actions">
            <button class="primary" :disabled="busy" @click="save">
              保存草稿</button
            ><button
              :disabled="busy || !selected.key || dirty"
              @click="showPreview"
            >
              手机预览</button
            ><button
              class="primary"
              :disabled="busy || !selected.key || dirty || !preview"
              @click="change(selected, 'publish')"
            >
              发布
            </button>
          </div>
          <p v-if="dirty">有未保存修改。保存后可查看操作说明并恢复编辑位置。</p>
          <div v-if="preview" class="phone">
            <PageContent :data="preview" :images="images" />
          </div>
        </div>
      </div>
    </section>
    <div
      v-if="deleteEntry"
      class="delete-confirm"
      role="dialog"
      aria-modal="true"
      aria-label="删除页面"
    >
      <h3>删除“{{ deleteEntry.draft.name }}”</h3>
      <p>删除后旧链接立即不可用，记录保留，标识不会复用。</p>
      <p v-if="!references?.length">
        未发现已保存菜单引用；已分享的链接仍可能受影响。
      </p>
      <ul v-else>
        <li v-for="(r, i) in references" :key="i">
          {{ r.name }} · {{ r.status }} · 菜单记录 {{ r.id }}
        </li>
      </ul>
      <button :disabled="busy" @click="change(deleteEntry, 'delete')">
        确认删除</button
      ><button @click="deleteEntry = null">取消</button>
    </div>
  </section>
</template>
<style scoped>
.manager {
  padding: 22px;
  background: #fff;
  color: #24362a;
  position: relative;
}
header,
.actions,
.filters,
nav {
  display: flex;
  gap: 10px;
  align-items: center;
  flex-wrap: wrap;
}
header {
  justify-content: space-between;
}
h2,
h3 {
  margin: 0;
}
p,
small {
  color: #647067;
  font-size: 13px;
}
small {
  display: block;
}
button,
input,
select,
textarea {
  font: inherit;
  border: 1px solid #ccd7cf;
  border-radius: 6px;
  padding: 9px;
  background: white;
  max-width: 100%;
  box-sizing: border-box;
}
button {
  cursor: pointer;
}
button:disabled {
  opacity: 0.5;
  cursor: default;
}
.primary,
.active {
  background: #28513d;
  color: white;
}
.filters,
nav {
  margin: 18px 0;
}
.table-wrap {
  overflow: auto;
}
table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}
th,
td {
  text-align: left;
  padding: 12px 8px;
  border-bottom: 1px solid #e3e8e4;
  min-width: 90px;
}
td.actions {
  min-width: 250px;
}
label {
  display: grid;
  gap: 7px;
  margin-bottom: 14px;
}
.empty {
  padding: 32px;
  text-align: center;
  background: #f4f7f4;
}
.editor {
  margin-top: 24px;
  border: 1px solid #d6e1d8;
  padding: 20px;
  border-radius: 12px;
}
.editor-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 24px;
  margin-top: 20px;
}
.block,
.history {
  border: 1px solid #dae3dc;
  border-radius: 8px;
  padding: 12px;
  margin: 14px 0;
}
.history {
  display: grid;
  gap: 8px;
}
.path,
code {
  overflow-wrap: anywhere;
  font-size: 12px;
}
.phone {
  max-width: 390px;
  margin: 20px auto;
  border: 7px solid #32463b;
  border-radius: 22px;
  overflow: hidden;
  max-height: 760px;
  overflow-y: auto;
}
.error {
  color: #b52e28;
}
.success {
  color: #2a6d43;
}
.delete-confirm {
  position: fixed;
  z-index: 1000;
  inset: 25% 20% auto;
  background: white;
  border: 2px solid #a13d33;
  box-shadow: 0 0 0 100vmax #0006;
  padding: 24px;
  border-radius: 12px;
}
.delete-confirm button {
  margin-right: 10px;
}
.file {
  font-size: 13px;
}
@media (max-width: 800px) {
  table, tbody, tr, td { display: block; width: 100%; }
  thead { display: none; }
  tr { border: 1px solid #dce5de; border-radius: 10px; margin: 12px 0; padding: 8px; box-sizing: border-box; }
  td { min-width: 0; box-sizing: border-box; display: grid; grid-template-columns: 80px minmax(0,1fr); gap: 10px; }
  td::before { content: attr(data-label); color: #647067; }
  td.actions { min-width: 0; display: flex; }
  td.actions::before { width: 100%; }

  .manager {
    padding: 12px;
  }
  .editor-grid {
    grid-template-columns: 1fr;
  }
  .delete-confirm {
    inset: 15% 12px auto;
  }
  .editor {
    padding: 12px;
  }
}
</style>
