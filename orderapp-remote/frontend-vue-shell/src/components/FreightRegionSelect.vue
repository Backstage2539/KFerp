<template>
 <div class="region-select">
  <div class="filters"><input v-model="query" placeholder="搜索地区、简称、全拼、首字母或代码" aria-label="搜索地区" /><select v-model="province"><option value="">全部省份</option><option v-for="r in provinces" :key="r.code" :value="r.code.slice(0,2)">{{r.name}}</option></select><select v-model.number="level"><option :value="0">全部层级</option><option :value="1">整省</option><option :value="2">城市</option><option :value="3">区县</option></select></div>
  <div class="selection"><span v-if="!modelValue.length">尚未选择地区</span><button v-for="r in selected" :key="r.code" type="button" @click="toggle(r.code)">{{r.path||r.name}} ×</button></div>
  <div class="options"><label v-for="r in matches.slice(0,150)" :key="r.code"><input type="checkbox" :checked="modelValue.includes(r.code)" @change="toggle(r.code)" /><span>{{r.path||r.name}} <small>{{r.level===1?'整省':r.level===2?'全市':'区县'}} · {{r.code}}</small></span></label></div>
  <p v-if="matches.length>150">共{{matches.length}}项，请继续搜索或选择省份缩小范围。</p><p v-if="!matches.length">没有符合条件的地区</p>
 </div>
</template>
<script setup>
import {computed,ref} from 'vue'
import {searchFreightRegions} from '../lib/freight.js'
const props=defineProps({modelValue:{type:Array,default:()=>[]},regions:{type:Array,default:()=>[]}}),emit=defineEmits(['update:modelValue'])
const query=ref(''),province=ref(''),level=ref(1)
const provinces=computed(()=>props.regions.filter(r=>r.level===1)),selected=computed(()=>props.regions.filter(r=>props.modelValue.includes(r.code)))
const matches=computed(()=>searchFreightRegions(props.regions,query.value).filter(r=>(!province.value||r.code.startsWith(province.value))&&(!level.value||r.level===level.value||query.value)))
function toggle(c){emit('update:modelValue',props.modelValue.includes(c)?props.modelValue.filter(x=>x!==c):[...props.modelValue,c])}
</script>
<style scoped>
.filters{display:flex;gap:8px;flex-wrap:wrap}.filters input{flex:1;min-width:200px}.filters select,.filters input{padding:8px;border:1px solid #cbd9cf;border-radius:6px}.selection{display:flex;gap:6px;flex-wrap:wrap;margin:10px 0;color:#64756b}.selection button{background:#edf6f0;border:1px solid #d2e3d8;padding:5px 8px;border-radius:5px;color:#286248;font-size:12px}.options{max-height:240px;overflow:auto;border:1px solid #dce5df;padding:8px;display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:6px}.options label{display:flex;align-items:center;gap:8px;font-size:13px;padding:5px}.options input{width:16px;height:16px}.options small{display:block;color:#78857d}p{font-size:12px;color:#78857d}
</style>
