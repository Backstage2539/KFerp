<script setup>
import { onMounted, ref } from 'vue'
import { apiGet, apiSend } from '../api/client'
import {
  menuToEditor,
  menuFromEditor,
  setMenuAction,
  menuAction,
} from '../lib/wechat-official'
import WechatMenuButton from './WechatMenuButton.vue'

const status = ref({}),
  tab = ref('status'),
  error = ref(''),
  notice = ref(''),
  busy = ref(false),
  entries = ref([]),
  bindings = ref([]),
  history = ref([]),
  groups = ref([]),
  preview = ref(null)
const base = '/api/customer-portal/admin/wechat'
const labels = {
  price: '历史价格表入口',
  page: '页面入口',
  orders: '查看全部订单',
  recent1: '回复最近一次订单',
  recent3: '回复最近三次订单',
  order: '进入下单',
  home: '小程序首页',
  view: '打开网页',
  media_id: '发送素材',
  view_limited: '查看图文',
}
function removeGroup(index) {
  groups.value.splice(index, 1)
  changed()
}
function removeChild(group, index) {
  group.sub_button.splice(index, 1)
  changed()
}
function changed() {
  preview.value = null
}
async function run(fn) {
  busy.value = true
  error.value = ''
  notice.value = ''
  try {
    await fn()
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
async function loadMenuEntries() {
  entries.value = ((await apiGet('/api/admin/page-entries')).rows || []).filter(
    (entry) => entry.enabled && entry.published && !entry.deleted,
  )
}
async function selectTab(key) {
  tab.value = key
  if (key === 'menu') await run(loadMenuEntries)
}
async function load() {
  await run(async () => {
    status.value = await apiGet(base + '/status')
    await loadMenuEntries()
    bindings.value = (await apiGet(base + '/bindings')).rows
    history.value = (await apiGet(base + '/menus')).rows
    if (!history.value.length && status.value.enabled) {
      const imported = await apiSend(base + '/menus/import', {
        method: 'POST',
        body: {},
      })
      groups.value = menuToEditor(imported.menu)
      history.value = (await apiGet(base + '/menus')).rows
    } else if (!groups.value.length && history.value.length)
      groups.value = menuToEditor(history.value[0].menu)
  })
}
function addGroup() {
  if (groups.value.length < 3) {
    groups.value.push({ name: '新菜单', sub_button: [] })
    changed()
  }
}
function addChild(group) {
  if (!group.sub_button) group.sub_button = []
  if (group.sub_button.length >= 5) return
  const b = { name: '新子菜单' }
  setMenuAction(b, 'recent1', status.value.mini_app_id)
  group.sub_button.push(b)
  delete group.type
  delete group.url
  delete group.pagepath
  delete group.appid
  delete group.key
  changed()
}
async function importCurrent() {
  await run(async () => {
    const data = await apiSend(base + '/menus/import', {
      method: 'POST',
      body: {},
    })
    groups.value = menuToEditor(data.menu)
    changed()
    history.value = (await apiGet(base + '/menus')).rows
    notice.value = '已读取公众号当前菜单，请检查后编辑。'
  })
}
async function saveDraft() {
  await run(async () => {
    await apiSend(base + '/menus/draft', {
      method: 'POST',
      body: { menu: menuFromEditor(groups.value) },
    })
    notice.value = '菜单草稿已保存'
    history.value = (await apiGet(base + '/menus')).rows
  })
}
async function showPreview() {
  await run(async () => {
    preview.value = await apiSend(base + '/menus/preview', {
      method: 'POST',
      body: { menu: menuFromEditor(groups.value) },
    })
  })
}
async function publish() {
  await run(async () => {
    await apiSend(base + '/menus/publish', {
      method: 'POST',
      body: {
        menu: menuFromEditor(groups.value),
        preview_token: preview.value?.preview_token,
      },
    })
    notice.value = '微信已接收菜单，请在手机公众号中核对实际菜单。'
    preview.value = null
    history.value = (await apiGet(base + '/menus')).rows
  })
}
async function unbind(b) {
  await run(async () => {
    await apiSend(base + `/bindings/${encodeURIComponent(b.openid)}`, {
      method: 'DELETE',
    })
    bindings.value = (await apiGet(base + '/bindings')).rows
    notice.value = '已解除绑定，公众号订单查询立即停止。'
  })
}
async function restore(row) {
  await run(async () => {
    await loadMenuEntries()
    groups.value = menuToEditor(row.menu)
    changed()
    tab.value = 'menu'
    notice.value = '历史菜单已载入编辑区，预览后发布才会生效。'
  })
}
function openManual() {
  window.dispatchEvent(
    new CustomEvent('kferp:navigate-view', {
      detail: {
        key: 'wechatOfficialManual',
        returnNavigation: {
          key: 'uiSettings',
          params: { tab: 'wechat' },
          label: '返回公众号管理',
        },
      },
    }),
  )
}
onMounted(load)
</script>
<template>
  <section class="wechat-panel">
    <header>
      <div>
        <h2>公众号管理</h2>
        <p>配置公众号菜单和认证后的订单查询。页面请在“页面入口管理”中维护。</p>
      </div>
      <div>
        <button type="button" @click="openManual">操作说明</button>
        <button type="button" :disabled="busy" @click="load">刷新</button>
      </div>
    </header>
    <nav>
      <button
        v-for="[key, label] in [
          ['status', '接入状态'],
          ['menu', '公众号菜单'],
          ['bindings', '客户绑定'],
          ['history', '发布记录'],
        ]"
        :key="key"
        :class="{ active: tab === key }"
        type="button"
        @click="selectTab(key)"
      >
        {{ label }}
      </button>
    </nav>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="notice" class="success" role="status">{{ notice }}</p>
    <div v-if="tab === 'status'" class="status">
      <h3>{{ status.enabled ? '公众号已启用' : '公众号尚未启用' }}</h3>
      <p>
        正式菜单发布前，需完成服务号权限、开放平台绑定、第三方消息服务及小程序正式版本核对。
      </p>
      <dl>
        <dt>公众号标识</dt>
        <dd>{{ status.app_id || '尚未配置' }}</dd>
        <dt>服务端凭证</dt>
        <dd>
          {{
            status.secret_configured && status.token_configured
              ? '已配置'
              : '待配置'
          }}
        </dd>
        <dt>消息加密</dt>
        <dd>{{ status.encryption_configured ? '已配置' : '待配置' }}</dd>
        <dt>跨平台身份识别</dt>
        <dd>
          {{
            status.unionid_enabled ? '已开启' : '待核对，绑定码可作为备用方式'
          }}
        </dd>
        <dt>网页授权</dt><dd>{{ status.web_oauth_ready ? '已开启' : '未启用或配置未齐' }}</dd><dt>网页授权回调</dt><dd>{{ status.web_oauth_callback || '待配置授权域名' }}</dd>
        <dt>菜单发布</dt>
        <dd>{{ status.menu_publish_enabled ? '已开启' : '等待正式上线' }}</dd>
      </dl>
    </div>

    <div v-else-if="tab === 'menu'" class="menu">
      <div class="actions">
        <button
          type="button"
          :disabled="busy || !status.enabled"
          @click="importCurrent"
        >
          读取公众号现有菜单
        </button>
        <button type="button" :disabled="groups.length >= 3" @click="addGroup">
          添加一级菜单
        </button>
      </div>
      <p v-if="!groups.length">
        先读取现有菜单，或创建菜单。最多 3 个一级菜单，每组最多 5 个子菜单。
      </p>
      <article v-for="(group, i) in groups" :key="i">
        <div class="group-head">
          <label>
            一级菜单名称
            <input v-model="group.name" maxlength="4" @input="changed" />
          </label>
          <button
            type="button"
            @click="removeGroup(i)"
          >
            删除本组
          </button>
        </div>
        <template v-if="group.sub_button?.length">
          <WechatMenuButton
            v-for="(child, j) in group.sub_button"
            :key="j"
            :button="child"
            :app-id="status.mini_app_id"
            :entries="entries"
            @change="changed"
            @remove="removeChild(group, j)"
          />
        </template>
        <WechatMenuButton
          v-else-if="group.type"
          :button="group"
          :app-id="status.mini_app_id"
          :entries="entries"
          @change="changed"
          @remove="removeGroup(i)"
        />
        <button
          type="button"
          :disabled="group.sub_button?.length >= 5"
          @click="addChild(group)"
        >
          添加子菜单
        </button>
      </article>
      <button
        type="button"
        :disabled="busy || !groups.length"
        @click="saveDraft"
      >
        保存菜单草稿
      </button>
      <button
        type="button"
        :disabled="busy || !groups.length"
        @click="showPreview"
      >
        预览完整菜单与变化
      </button>
      <div v-if="preview" class="compare">
        <section
          v-for="[key, label] in [
            ['current', '公众号当前菜单'],
            ['menu', '准备发布的菜单'],
          ]"
          :key="key"
        >
          <h3>{{ label }}</h3>
          <ul>
            <li v-for="(group, i) in menuToEditor(preview[key])" :key="i">
              <b>{{ group.name }}</b>
              <ul v-if="group.sub_button?.length">
                <li v-for="(b, j) in group.sub_button" :key="j">
                  {{ b.name }} · {{ labels[menuAction(b)] || '现有动作' }}
                  <small>
                    {{ b.pagepath || b.url || b.key || b.media_id }}
                  </small>
                </li>
              </ul>
              <span v-else>
                · {{ labels[menuAction(group)] || '现有动作' }}
                <small>
                  {{
                    group.pagepath || group.url || group.key || group.media_id
                  }}
                </small>
              </span>
            </li>
          </ul>
        </section>
        <p>发布后将使用右侧完整菜单；原菜单会保存在发布记录中。</p>
        <button
          type="button"
          :disabled="busy || !preview.publish_allowed"
          @click="publish"
        >
          发布此菜单
        </button>
        <p v-if="!preview.publish_allowed">
          公众号接入或正式上线条件尚未完成，当前可编辑和预览。
        </p>
      </div>
    </div>
    <div v-else-if="tab === 'bindings'">
      <p v-if="!bindings.length">暂无客户绑定。</p>
      <div v-for="b in bindings" :key="b.openid" class="binding">
        <b>{{ b.customer_name || '尚未选择查询客户' }}</b>
        <span>
          用户 #{{ b.mini_user_id }} · {{ b.active ? '已绑定' : '已解绑' }}
        </span>
        <button type="button" :disabled="busy || !b.active" @click="unbind(b)">
          解除绑定
        </button>
      </div>
    </div>
    <div v-else-if="tab === 'history'">
      <p v-if="!history.length">暂无发布记录。</p>
      <div v-for="row in history" :key="row.id" class="binding">
        <span>{{ new Date(row.created_at).toLocaleString() }}</span>
        <span>
          {{
            {
              draft: '已保存草稿',
              imported: '已导入',
              previous: '发布前菜单',
              publishing: '发布请求',
              published: '微信已接收',
              failed: '发布失败',
            }[row.status] || row.status
          }}
        </span>
        <span>{{ row.actor }}</span>
        <button type="button" @click="restore(row)">载入此版预览</button>
      </div>
    </div>
  </section>
</template>
<style scoped>
.wechat-panel {
  background: #fff;
  border: 1px solid #e1e9e3;
  border-radius: 14px;
  padding: 24px;
  margin-bottom: 20px;
  color: #23372b;
  display: grid;
  gap: 18px;
}
header,
.actions,
.group-head,
.binding {
  display: flex;
  gap: 12px;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
}
h2,
h3,
p {
  margin: 0;
}
p {
  color: #647267;
  font-size: 13px;
  line-height: 1.7;
}
nav {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  border-bottom: 1px solid #e8ede9;
  padding-bottom: 12px;
}
button,
input {
  font: inherit;
  border: 1px solid #d5dfd8;
  border-radius: 7px;
  padding: 9px 13px;
  background: white;
  min-width: 0;
}
button {
  cursor: pointer;
  color: #236440;
}
button.active {
  background: #e5f2e9;
  border-color: #87b79a;
}
button:disabled {
  opacity: 0.45;
  cursor: default;
}
label {
  display: flex;
  align-items: center;
  gap: 10px;
}
.menu {
  display: grid;
  gap: 18px;
}
article {
  padding: 18px;
  border: 1px solid #dfe8e1;
  border-radius: 12px;
  display: grid;
  gap: 12px;
}
dl {
  display: grid;
  grid-template-columns: 150px 1fr;
  gap: 12px;
  font-size: 14px;
}
dd {
  margin: 0;
  overflow-wrap: anywhere;
}
.compare {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 18px;
  border-top: 1px solid #ddd;
  padding-top: 18px;
}
.compare > p,
.compare > button {
  grid-column: 1/-1;
}
li {
  margin: 10px 0;
}
small {
  display: block;
  overflow-wrap: anywhere;
  max-width: 100%;
  color: #647267;
  margin-top: 6px;
}
.compare > section {
  min-width: 0;
}
.binding {
  padding: 14px;
  border-bottom: 1px solid #eee;
}
.error {
  color: #a23535;
}
.success {
  color: #246b40;
}
@media (max-width: 700px) {
  .compare {
    grid-template-columns: 1fr;
  }
  .wechat-panel {
    padding: 14px;
  }
}
</style>
