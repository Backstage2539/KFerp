<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { BeanListSummary } from '../api/customerPortal'
import { beanListCardRows, beanListDisplayStyle, beanListQualityLines, splitBeanListHighlight } from '../utils/beanListDisplay'
import { emptyPriceListSearch, filterPriceList, priceListFacets, priceListSearchMemory, reconcilePriceListSearch, type PriceListSearch } from '../utils/priceListSearch'
const props=defineProps<{publication:BeanListSummary;searchScope:string}>()
const state=ref<PriceListSearch>(emptyPriceListSearch()),draft=ref<PriceListSearch>(emptyPriceListSearch()),filterOpen=ref(false)
watch(()=>props.searchScope,scope=>{filterOpen.value=false;state.value=reconcilePriceListSearch(props.publication,priceListSearchMemory.load(scope))},{immediate:true})
watch(()=>props.publication,publication=>{state.value=reconcilePriceListSearch(publication,state.value);if(filterOpen.value)closeFilters()})
watch(state,value=>priceListSearchMemory.save(props.searchScope,value),{deep:true})
const facets=computed(()=>priceListFacets(props.publication))
const result=computed(()=>filterPriceList(props.publication,state.value))
const selectedCount=computed(()=>state.value.badges.length+Object.values(state.value.attributes).reduce((n,values)=>n+values.length,0))
const hasConditions=computed(()=>!!state.value.query.trim()||selectedCount.value>0)
const badgeOptions=[{key:'new',label:'上新'},{key:'recommended',label:'推荐'},{key:'none',label:'无标签'}]
function finishSearch(){uni.hideKeyboard()}
function openFilters(){uni.hideKeyboard();draft.value=reconcilePriceListSearch(props.publication,state.value);filterOpen.value=true}
function closeFilters(){filterOpen.value=false}
function clear(){state.value=emptyPriceListSearch();uni.hideKeyboard()}
function toggleField(key:string,value:string){const selected=draft.value.attributes[key]||[];draft.value.attributes[key]=selected.includes(value)?selected.filter(v=>v!==value):[...selected,value]}
function toggleBadge(value:string){const selected=draft.value.badges;draft.value.badges=selected.includes(value)?selected.filter(v=>v!==value):[...selected,value]}
function applyFilters(){state.value=reconcilePriceListSearch(props.publication,draft.value);closeFilters()}
function resetDraft(){draft.value={query:state.value.query,attributes:{},badges:[]}}
</script>
<template>
 <view class="price-list-surface" :style="beanListDisplayStyle(publication)">
  <view class="cover">
   <image v-if="publication.logo_image" class="logo" :src="publication.logo_image" mode="aspectFit"/>
   <text class="title">{{publication.title||'商品价格表'}}</text>
   <text v-if="publication.subtitle" class="subtitle">{{publication.subtitle}}</text>
   <text v-if="publication.show_version!==false" class="version">{{publication.version_no}} · {{publication.published_at}}</text>
   <text v-if="publication.brand_intro" class="intro">{{publication.brand_intro}}</text>
   <text v-if="publication.show_changelog!==false&&publication.changelog" class="intro">{{publication.changelog}}</text>
  </view>
  <slot name="notice"/>
  <view class="search-tools">
   <view class="search-row"><input v-model="state.query" class="search-input" placeholder="搜索商品、行业字段、规格或价格" confirm-type="search" :maxlength="200" @confirm="finishSearch"/><button class="filter-button" @tap="openFilters">筛选{{selectedCount?' ('+selectedCount+')':''}}</button></view>
   <view class="search-meta"><text>匹配 {{result.count}} / {{result.total}} 项商品</text><button v-if="hasConditions" class="clear-button" @tap="clear">清空条件</button></view>
  </view>
  <view v-if="!result.count" class="empty"><text>当前条件未找到商品</text><button v-if="hasConditions" class="empty-clear" @tap="clear">清空条件，查看全部</button></view>
  <view v-for="(group,gi) in result.groups" :key="gi" class="group">
   <text v-if="publication.show_category_numbers!==false&&group.show_category!==false&&group.category" class="category">{{group.category}}</text>
   <view v-for="(row,ri) in beanListCardRows(group.items,publication.layout_style==='table'?1:(publication.cards_per_row||1))" :key="ri" class="product-row">
    <view v-for="(bean,bi) in row" :key="bi" :class="['product',publication.layout_style==='table'?'table-product':'card-product']">
     <view class="product-details">
      <text class="product-title"><text v-for="(part,pi) in splitBeanListHighlight(bean.name,bean.highlight_terms||[])" :key="pi" :class="['product-name',{red:part.red}]">{{part.text}}</text><text v-if="bean.badge_label" :class="['badge',bean.badge?'badge-'+bean.badge:'']">{{bean.badge_label}}</text></text>
      <text v-if="bean.code" class="code">{{bean.code}}</text>
      <text v-for="(line,li) in bean.attribute_lines||[]" :key="'attr-'+li" class="detail industry-line">{{line}}</text>
      <text v-if="bean.recommended_use" class="detail">出品建议：<text v-for="(part,pi) in splitBeanListHighlight(bean.recommended_use,bean.highlight_terms||[])" :key="pi" :class="{red:part.red}">{{part.text}}</text></text>
      <text v-if="bean.flavor" class="detail">风味：<text v-for="(part,pi) in splitBeanListHighlight(bean.flavor,bean.highlight_terms||[])" :key="pi" :class="{red:part.red}">{{part.text}}</text></text>
      <text v-if="bean.description" class="detail"><text v-for="(part,pi) in splitBeanListHighlight(bean.description,bean.highlight_terms||[])" :key="pi" :class="{red:part.red}">{{part.text}}</text></text>
      <text v-for="quality in beanListQualityLines(bean)" :key="quality.label" class="detail">{{quality.label}}：{{quality.value}}</text>
     </view>
     <view class="prices"><view v-for="(price,pi) in bean.prices||[]" :key="pi" class="price"><text>{{price.label}}</text><text :class="['price-value',{red:price.red}]">{{price.value}}</text></view></view>
    </view>
   </view>
  </view>
  <text class="footer">{{publication.brand_name||'棵凡咖啡'}} · 价格以当前展示版本为准</text>
 </view>
 <view v-if="filterOpen" class="filter-overlay" @tap="closeFilters" @touchmove.stop.prevent>
  <view class="filter-sheet" @tap.stop>
   <view class="filter-heading"><text class="filter-title">筛选商品</text><button class="close-button" aria-label="关闭筛选" @tap="closeFilters">×</button></view>
   <text class="filter-hint">同一项可多选，不同项同时满足。</text>
   <scroll-view scroll-y class="filter-scroll" @touchmove.stop>
    <view class="facet"><text class="facet-title">商品标签</text><view class="facet-options"><button v-for="option in badgeOptions" :key="option.key" :class="['option',{selected:draft.badges.includes(option.key)}]" @tap="toggleBadge(option.key)">{{option.label}}</button></view></view>
    <view v-for="facet in facets" :key="facet.key" class="facet"><text class="facet-title">{{facet.label}}</text><view class="facet-options"><button v-for="value in facet.values" :key="value" :class="['option',{selected:(draft.attributes[facet.key]||[]).includes(value)}]" @tap="toggleField(facet.key,value)">{{value}}</button></view></view>
    <text v-if="!facets.length" class="filter-hint">此版本暂无可筛选的行业字段，可使用全文搜索。</text>
   </scroll-view>
   <view class="filter-actions"><button class="reset-button" @tap="resetDraft">重置筛选</button><button class="apply-button" @tap="applyFilters">显示结果</button></view>
  </view>
 </view>
</template>
<style scoped>
.price-list-surface{padding:32rpx 24rpx;border-radius:22rpx;background-size:cover;box-sizing:border-box;overflow-wrap:anywhere}.cover{text-align:center;margin-bottom:28rpx}.logo{width:180rpx;height:110rpx;display:block;margin:0 auto 18rpx}.title,.subtitle,.version,.intro,.category,.code,.detail,.footer{display:block}.title{font-size:40rpx;font-weight:800;line-height:1.4}.subtitle,.intro{font-size:25rpx;line-height:1.7;margin-top:12rpx}.version{font-size:22rpx;opacity:.7;margin-top:12rpx}.search-tools{padding:18rpx;background:#ffffffed;border:1rpx solid #d7e1d7;border-radius:14rpx;color:#253d31;margin-bottom:26rpx}.search-row{display:flex;gap:12rpx;align-items:center}.search-input{flex:1;min-width:0;height:72rpx;padding:0 16rpx;box-sizing:border-box;background:#f6f8f4;border-radius:10rpx;font-size:25rpx}.filter-button{margin:0;flex-shrink:0;padding:0 18rpx;height:72rpx;line-height:72rpx;border-radius:10rpx;background:#285d3e;color:#fff;font-size:25rpx}.search-meta{display:flex;align-items:center;justify-content:space-between;min-height:50rpx;margin-top:10rpx;font-size:23rpx;color:#69766c}.clear-button{margin:0;padding:0 8rpx;line-height:44rpx;background:transparent;color:#285d3e;font-size:23rpx}.clear-button::after,.close-button::after{border:0}.group{margin-top:26rpx}.category{font-size:31rpx;font-weight:700;margin-bottom:18rpx}.product-row{display:flex;gap:16rpx;align-items:stretch;margin-bottom:16rpx}.product{box-sizing:border-box;background:#fffffff0;color:#242e28;border-radius:14rpx;padding:22rpx;min-width:0;overflow-wrap:anywhere}.card-product{flex:1;display:flex;flex-direction:column}.table-product{width:100%;display:flex;gap:24rpx;align-items:flex-start}.product-details{min-width:0;flex:1}.product-title{display:block;line-height:1.7;overflow-wrap:anywhere}.product-name{font-size:29rpx;font-weight:700}.badge{display:inline;margin-left:12rpx;font-size:22rpx;white-space:nowrap;color:#a3392b;font-weight:600}.badge-thumb,.badge-medal{color:#8a641e}.code{font-size:22rpx;color:#7c877e;margin-top:4rpx}.detail{font-size:24rpx;line-height:1.8;margin-top:8rpx}.industry-line{color:#4b6554}.prices{margin-top:18rpx;font-size:25rpx}.card-product .prices{margin-top:auto;padding-top:18rpx}.price{display:flex;justify-content:space-between;gap:10rpx;line-height:1.7;margin-top:6rpx}.price-value{font-weight:700}.table-product .prices{width:180rpx;flex-shrink:0;margin-top:0}.table-product .price{display:block;text-align:right}.table-product .price-value{display:block}.red{color:#be332d}.footer{text-align:center;font-size:22rpx;opacity:.6;margin-top:28rpx;line-height:1.7}.empty{padding:44rpx 16rpx;text-align:center;color:#718277;font-size:26rpx}.empty-clear{margin-top:22rpx;line-height:68rpx;background:#eaf2e7;color:#285d3e;font-size:25rpx}.filter-overlay{position:fixed;inset:0;z-index:1300;background:rgba(20,35,26,.42)}.filter-sheet{position:absolute;bottom:0;left:0;right:0;background:#fffdf8;color:#253d31;border-radius:28rpx 28rpx 0 0;padding:28rpx 28rpx calc(24rpx + env(safe-area-inset-bottom));box-sizing:border-box}.filter-heading{display:flex;align-items:center;justify-content:space-between;gap:16rpx}.filter-title{font-size:34rpx;font-weight:700}.close-button{margin:0;padding:0;width:60rpx;height:60rpx;line-height:56rpx;background:transparent;color:#6c786d;font-size:44rpx}.filter-hint{display:block;font-size:23rpx;color:#758076;line-height:1.7;margin:12rpx 0}.filter-scroll{height:52vh}.facet{margin:24rpx 0}.facet-title{display:block;font-size:27rpx;font-weight:600;margin-bottom:14rpx}.facet-options{display:flex;flex-wrap:wrap;gap:14rpx}.option{margin:0;padding:10rpx 22rpx;line-height:1.6;font-size:25rpx;background:#f3f5ef;color:#415947;border-radius:12rpx;text-align:left;max-width:100%;overflow-wrap:anywhere}.option.selected{background:#deebdb;color:#285d3e;font-weight:700}.option.selected::after{border:2rpx solid #74a080}.filter-actions{display:flex;gap:16rpx;padding-top:16rpx}.reset-button,.apply-button{flex:1;margin:0;line-height:80rpx;font-size:27rpx;border-radius:12rpx}.reset-button{background:#eef2e9;color:#415947}.apply-button{background:#285d3e;color:#fff}
</style>
