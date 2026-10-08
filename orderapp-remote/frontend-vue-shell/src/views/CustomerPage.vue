<script setup>
import { onMounted, onBeforeUnmount, ref } from "vue";
import { pageRequest, apiFetch, appURL } from "../api/client";
import PageContent from "../components/PageContent.vue";
const key = window.location.pathname.split("/").filter(Boolean).pop();
const data = ref(null),
  images = ref({}),
  error = ref(""),
  loading = ref(false),
  auth = ref(null),
  needsLogin = ref(false),
  loginName = ref(""),
  password = ref(""),
  customerId = ref(0),
  busy = ref(false);
let sequence = 0;
function clear() {
  data.value = null;
  Object.values(images.value).forEach(URL.revokeObjectURL);
  images.value = {};
}
async function load() {
  const seq = ++sequence;
  clear();
  error.value = "";
  loading.value = true;
  needsLogin.value = false;
  try {
    auth.value = await pageRequest("/api/page-auth/status");
    customerId.value = auth.value.customer?.current_customer_id || 0;
    const result = await pageRequest("/api/pages/" + key);
    if (result.document.kind === "function")
      throw new Error("请通过小程序打开此功能页面。");
    const local = {};
    try {
      for (const b of result.document.blocks || []) {
        if (b.kind !== "image") continue;
        const res = await apiFetch(
          "/api/pages/" + key + "/images/" + b.asset_id,
          {
            customerSession: true,
            cache: "no-store",
            credentials: "same-origin",
          },
        );
        if (!res.ok) throw new Error("图片不可用，请重新打开页面");
        local[b.asset_id] = URL.createObjectURL(await res.blob());
      }
      if (seq === sequence) {
        images.value = local;
        data.value = result;
        document.title = result.document.name + " · 棵凡咖啡";
      } else Object.values(local).forEach(URL.revokeObjectURL);
    } catch (e) {
      Object.values(local).forEach(URL.revokeObjectURL);
      throw e;
    }
  } catch (e) {
    if (seq !== sequence) return;
    error.value = e.message;
    needsLogin.value = [401, 403].includes(e.status);
    if (
      e.status === 401 &&
      auth.value?.oauth_ready &&
      /MicroMessenger/i.test(navigator.userAgent) &&
      !new URLSearchParams(location.search).has("auth")
    ) {
      location.replace(
        appURL("/api/page-auth/wechat?entry=" + encodeURIComponent(key)),
      );
    }
  } finally {
    if (seq === sequence) loading.value = false;
  }
}
async function login() {
  busy.value = true;
  error.value = "";
  try {
    await pageRequest("/api/page-auth/login", {
      method: "POST",
      body: { login: loginName.value, password: password.value },
    });
    password.value = "";
    await load();
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function selectCustomer() {
  busy.value = true;
  try {
    await pageRequest("/api/page-auth/customer", {
      method: "POST",
      body: { customer_id: customerId.value },
    });
    await load();
  } catch (e) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function logout() {
  await pageRequest("/api/page-auth/logout", { method: "POST", body: {} });
  await load();
}
function visibility() {
  if (document.hidden) {
    sequence++;
    clear();
  } else void load();
}
function pageshow(event) {
  if (event.persisted) void load();
}
onMounted(() => {
  void load();
  document.addEventListener("visibilitychange", visibility);
  window.addEventListener("pageshow", pageshow);
});
onBeforeUnmount(() => {
  sequence++;
  clear();
  document.removeEventListener("visibilitychange", visibility);
  window.removeEventListener("pageshow", pageshow);
});
</script>
<template>
  <main>
    <header>
      <strong>棵凡咖啡</strong
      ><button v-if="auth?.authenticated" @click="logout">退出登录</button>
    </header>
    <div
      v-if="auth?.authenticated && auth.customer.bindings?.length > 1"
      class="account"
    >
      <label
        >当前客户<select
          v-model.number="customerId"
          :disabled="busy"
          @change="selectCustomer"
        >
          <option :value="0" disabled>请选择客户</option>
          <option
            v-for="b in auth.customer.bindings.filter(
              (b) => b.status === 'approved',
            )"
            :key="b.customer_id"
            :value="b.customer_id"
          >
            {{ b.customer_name }}
          </option>
        </select></label
      >
    </div>
    <p v-if="loading" class="state">正在加载…</p>
    <p v-if="error" role="alert" class="state error">{{ error }}</p>
    <form v-if="needsLogin && !auth?.authenticated" @submit.prevent="login">
      <h1>客户登录</h1>
      <p>使用已有客户账号。登录后返回此页面。</p>
      <label
        >账号<input
          v-model="loginName"
          autocomplete="username"
          required /></label
      ><label
        >密码<input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required /></label
      ><button :disabled="busy">{{ busy ? "登录中…" : "登录并继续" }}</button>
    </form>
    <PageContent v-if="data" :data="data" :images="images" /><button
      v-if="!loading && error"
      class="retry"
      @click="load"
    >
      重新加载
    </button>
  </main>
</template>
<style>
body {
  margin: 0;
  background: #f1f5f1;
  font-family: system-ui, sans-serif;
  color: #243b2d;
}
* {
  box-sizing: border-box;
}
main {
  max-width: 720px;
  margin: auto;
  min-height: 100vh;
}
header {
  padding: 18px 22px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: #244c38;
  color: white;
}
button,
select,
input {
  font: inherit;
  border: 1px solid #c7d5cb;
  padding: 11px;
  border-radius: 8px;
  background: white;
  color: #284a35;
  max-width: 100%;
}
button {
  cursor: pointer;
}
button:disabled {
  opacity: 0.5;
}
form {
  padding: 24px;
  background: white;
  margin: 20px;
  border-radius: 12px;
}
form h1 {
  font-size: 22px;
}
form p {
  font-size: 14px;
  color: #64786b;
}
label {
  display: grid;
  gap: 8px;
  margin-bottom: 18px;
}
form button {
  background: #28543d;
  color: white;
  width: 100%;
}
.state {
  padding: 20px;
  text-align: center;
}
.error {
  color: #a4362d;
}
.account {
  padding: 20px;
  background: white;
}
.account label {
  margin: 0;
}
.retry {
  display: block;
  margin: 20px auto;
}
</style>
