<script setup>
import { computed } from "vue";
import {
  menuAction,
  setMenuAction,
  menuPageKey,
  setMenuPage,
} from "../lib/wechat-official";
const props = defineProps({
  button: { type: Object, required: true },
  appId: String,
  entries: { type: Array, default: () => [] },
});
const emit = defineEmits(["change", "remove"]);
const action = computed(() => menuAction(props.button));
function setAction(value) {
  if (value === "page") {
    const e = props.entries[0];
    if (e)
      setMenuPage(props.button, e, props.appId, "mini", window.location.origin);
    emit("change");
    return;
  }
  setMenuAction(props.button, value, props.appId, props.entries[0]?.key || "");
  emit("change");
}
function changeEntry(
  key,
  mode = props.button.type === "view" ? "web" : "mini",
) {
  const e = props.entries.find((e) => e.key === key);
  if (e)
    setMenuPage(props.button, e, props.appId, mode, window.location.origin);
  emit("change");
}
const currentEntry = computed(() =>
  props.entries.find((e) => e.key === menuPageKey(props.button)),
);
</script>
<template>
  <div class="menu-button">
    <label>
      菜单名称
      <input v-model="button.name" maxlength="8" @input="emit('change')" />
    </label>
    <label>
      点击后
      <select :value="action" @change="setAction($event.target.value)">
        <option value="page" :disabled="!entries.length">选择页面入口</option>
        <option value="beans">小程序豆单中心</option>
 <option value="orders">小程序全部订单</option>
 <option value="order">小程序商品下单</option>
        <option
          v-if="!['page', 'beans', 'orders','order'].includes(action)"
          :value="action"
        >
          保留已导入的 {{ button.type }} 菜单
        </option>
      </select>
    </label>
    <label v-if="action === 'page'">
      页面入口
      <select
        :value="menuPageKey(button)"
        @change="changeEntry($event.target.value)"
      >
        <option v-if="!currentEntry" :value="menuPageKey(button)">
          保留现有页面路径（当前不可选）
        </option>
        <option v-for="entry in entries" :key="entry.key" :value="entry.key">
          {{ entry.published.name }}
        </option>
      </select>
    </label>
    <label
      v-if="action === 'page' && currentEntry?.published.kind !== 'function'"
      >打开方式
      <select
        :value="button.type === 'view' ? 'web' : 'mini'"
        @change="changeEntry(menuPageKey(button), $event.target.value)"
      >
        <option value="mini">小程序</option>
        <option value="web" :disabled="currentEntry?.published.visibility==='registered'">网页</option>
      </select>
    </label>
    <label
      v-if="
        action !== 'page' &&
        (button.type === 'view' || button.type === 'miniprogram')
      "
    >
      {{ button.type === "view" ? "网页地址" : "备用网页" }}
      <input v-model="button.url" @input="emit('change')" />
    </label>
    <button type="button" @click="emit('remove')">移除此项</button>
  </div>
</template>
<style scoped>
.menu-button {
  display: flex;
  gap: 12px;
  align-items: end;
  flex-wrap: wrap;
  padding: 14px;
  background: #f5f8f6;
  border-radius: 9px;
}
label {
  display: grid;
  gap: 7px;
  font-size: 13px;
  flex: 1;
  min-width: 140px;
}
input,
select,
button {
  font: inherit;
  padding: 8px;
  border: 1px solid #ccd7ce;
  border-radius: 6px;
  background: white;
  min-width: 0;
}
button {
  cursor: pointer;
  color: #a13535;
}
</style>
