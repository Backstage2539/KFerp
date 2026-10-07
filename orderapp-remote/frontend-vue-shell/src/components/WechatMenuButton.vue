<script setup>
import { computed } from 'vue'
import { menuAction, setMenuAction } from '../lib/wechat-official'
const props = defineProps({
  button: { type: Object, required: true },
  appId: String,
  entries: { type: Array, default: () => [] },
})
const emit = defineEmits(['change', 'remove'])
const action = computed(() => menuAction(props.button))
function setAction(value) {
  setMenuAction(props.button, value, props.appId, props.entries[0]?.key || '')
  emit('change')
}
function changeEntry(key) {
  setMenuAction(props.button, 'price', props.appId, key)
  emit('change')
}
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
        <option value="price">查看指定豆单</option>
        <option value="orders">查看全部订单</option>
        <option value="recent1">回复最近一次订单</option>
        <option value="recent3">回复最近三次订单</option>
        <option value="order">进入下单</option>
        <option value="home">打开小程序首页</option>
        <option value="view">打开网页</option>
        <option
          v-if="
            ![
              'price',
              'orders',
              'recent1',
              'recent3',
              'order',
              'home',
              'view',
            ].includes(action)
          "
          :value="action"
        >
          保留已导入的 {{ button.type }} 菜单
        </option>
      </select>
    </label>
    <label v-if="action === 'price'">
      价格表入口
      <select
        :value="(button.pagepath || '').split('entry=')[1]"
        @change="changeEntry($event.target.value)"
      >
        <option value="">请选择入口</option>
        <option v-for="entry in entries" :key="entry.key" :value="entry.key">
          {{ entry.name }} · {{ entry.version }}
        </option>
      </select>
    </label>
    <label v-if="button.type === 'view' || button.type === 'miniprogram'">
      {{ button.type === 'view' ? '网页地址' : '备用网页' }}
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
