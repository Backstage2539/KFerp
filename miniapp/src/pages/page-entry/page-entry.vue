<script setup lang="ts">
import { ref } from 'vue'
import {
  onLoad,
  onShow,
  onHide,
  onShareAppMessage,
  onShareTimeline,
} from '@dcloudio/uni-app'
import { fetchPageEntry, type PageDocument } from '../../api/pageEntry'
import { buildAPIURL } from '../../api/client'
import { fetchMe, type BeanListSummary } from '../../api/customerPortal'
import {
  MiniRequestError,
  isAuthenticationExpiredRequestError,
} from '../../api/client'
import { useSessionStore } from '../../stores/session'
import { loginRouteFor } from '../../utils/loginReturn'
import { beanListDisplayStyle } from '../../utils/beanListDisplay'
import {
  defaultMiniappShare as baseShare,
  defaultMiniappTimelineShare as baseTimeline,
  refreshMiniappShareMenu,
} from '../../utils/miniappShare'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
const document = ref<PageDocument | null>(null), images = ref<Record<string,string>>({})
const session = useSessionStore(),
  entry = ref(''),
  publication = ref<BeanListSummary | null>(null),
  loading = ref(false),
  error = ref(''),
  needsLogin = ref(false)
const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
let sequence = 0
const destination = () => `/pages/page-entry/page-entry?entry=${entry.value}`
const defaultMiniappShare = () => ({
  ...baseShare(),
  title: document.value?.name || '棵凡咖啡',
  path: destination(),
})
const defaultMiniappTimelineShare = () => ({
  ...baseTimeline(),
  title: document.value?.name || '棵凡咖啡',
  query: `entry=${encodeURIComponent(entry.value)}`,
})
function login() {
  uni.navigateTo({ url: loginRouteFor(destination()) })
}
async function load() {
  const request = ++sequence
  publication.value = null
  document.value = null
  images.value = {}
  error.value = ''
  needsLogin.value = false
  if (!/^[a-f0-9]{32}$/.test(entry.value)) {
    error.value = '页面入口无效'
    return
  }
  loading.value = true
  try {
    if (session.token) {
      try {
        session.applyContext(await fetchMe(session.token))
      } catch (e) {
        if (isAuthenticationExpiredRequestError(e)) session.clearSession()
        else throw e
      }
    }
    const data = await fetchPageEntry(entry.value, session.token)
    const local: Record<string,string> = {}
    for (const block of data.document.blocks || []) {
      if (block.kind !== 'image' || !block.asset_id) continue
      const asset = block.asset_id
      local[asset] = await new Promise<string>((resolve,reject)=>uni.downloadFile({
        url: buildAPIURL(`/api/pages/${entry.value}/images/${asset}`), header: session.token ? {Authorization:`Bearer ${session.token}`} : {},
        success:r=>r.statusCode===200?resolve(r.tempFilePath):reject(new MiniRequestError('图片不可用，请重新加载',r.statusCode)), fail:()=>reject(new Error('图片加载失败'))
      }))
    }
    if (request === sequence) {
      publication.value = data.publication || null; document.value=data.document; images.value=local
      if(data.document.kind==='function' && data.target_path) uni.redirectTo({url:'/'+data.target_path})
    }
  } catch (e) {
    if (request !== sequence) return
    error.value = e instanceof Error ? e.message : '暂时无法加载页面'
    needsLogin.value = e instanceof MiniRequestError && (e.statusCode === 401 || e.statusCode === 403)
  } finally {
    if (request === sequence) loading.value = false
  }
}
onLoad((query) => {
  entry.value = String(query?.entry || '')
})
onShow(() => {
  void load()
  void refreshMiniappShareMenu()
})
onHide(() => {
  sequence++
  publication.value = null
  document.value = null
  images.value = {}
})
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>
<template>
  <view
    class="page pull-up-brand-page"
    @touchstart="handlePullUpBrandTouchStart"
    @touchmove="handlePullUpBrandTouchMove"
    @touchend="handlePullUpBrandTouchEnd"
    @touchcancel="handlePullUpBrandTouchCancel"
  >
    <EnvironmentBadge />
    <text v-if="loading" class="state">正在加载页面…</text>
    <view v-if="error" class="state">
      <text>{{ error }}</text>
      <button v-if="needsLogin" @tap="login">登录后查看</button>
      <button v-else @tap="load">重新加载</button>
    </view>
    <view v-if="document?.kind === 'article'" class="sheet">
      <text class="title">{{ document.name }}</text>
      <view v-for="(block,index) in document.blocks || []" :key="index" class="article-block">
        <text v-if="block.kind === 'heading'" class="category">{{ block.text }}</text>
        <text v-else-if="block.kind === 'text'" class="paragraph">{{ block.text }}</text>
        <view v-else-if="block.kind === 'image'"><image v-if="images[block.asset_id || '']" :src="images[block.asset_id || '']" mode="widthFix" class="article-image"/><text class="muted">{{ block.caption }}</text></view>
      </view>
    </view>
    <view
      v-if="publication"
      class="sheet"
      :style="beanListDisplayStyle(publication)"
    >
      <image
        v-if="publication.logo_image"
        :src="publication.logo_image"
        mode="aspectFit"
        class="logo"
      />
      <text class="title">{{ publication.title || '商品价格表' }}</text>
      <text v-if="publication.subtitle" class="subtitle">
        {{ publication.subtitle }}
      </text>
      <text class="version">
        {{ publication.version_no }} · {{ publication.published_at }}
      </text>
      <text v-if="publication.brand_intro" class="intro">
        {{ publication.brand_intro }}
      </text>
      <view
        v-for="(group, gi) in publication.groups || []"
        :key="gi"
        class="group"
      >
        <text class="category">{{ group.category }}</text>
        <view v-for="(item, i) in group.items" :key="i" class="product">
          <view class="product-head">
            <text class="name">{{ item.name }}</text>
            <text v-if="item.badge_label" class="badge">
              {{ item.badge_label }}
            </text>
          </view>
          <text v-if="item.code" class="muted">{{ item.code }}</text>
          <text v-if="item.flavor" class="description">
            风味：{{ item.flavor }}
          </text>
          <text v-if="item.recommended_use" class="description">
            出品建议：{{ item.recommended_use }}
          </text>
          <text v-if="item.description" class="description">
            {{ item.description }}
          </text>
          <view class="prices">
            <view
              v-for="(price, j) in item.prices || []"
              :key="j"
              class="price"
            >
              <text>{{ price.label }}</text>
              <text :class="{ red: price.red }">{{ price.value }}</text>
            </view>
          </view>
        </view>
      </view>
      <text class="footer">棵凡咖啡 · 价格以当前展示版本为准</text>
    </view>
    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
  </view>
</template>
<style scoped>
.article-block{margin-top:30rpx}.paragraph{white-space:pre-wrap;line-height:1.8}.article-image{width:100%;display:block}
.page {
  min-height: 100vh;
  background: #f5f2ec;
  padding: 24rpx;
  box-sizing: border-box;
}
.state {
  display: flex;
  flex-direction: column;
  gap: 24rpx;
  text-align: center;
  padding: 80rpx 30rpx;
}
.sheet {
  padding: 40rpx 28rpx;
  border-radius: 24rpx;
  background-size: cover;
}
.logo {
  height: 120rpx;
  width: 180rpx;
  display: block;
  margin: 0 auto 24rpx;
}
.title,
.subtitle,
.version,
.intro,
.category,
.description,
.muted,
.footer {
  display: block;
}
.title {
  font-size: 42rpx;
  font-weight: 800;
  text-align: center;
}
.subtitle,
.version {
  font-size: 24rpx;
  text-align: center;
  margin-top: 14rpx;
}
.intro {
  font-size: 25rpx;
  line-height: 1.7;
  margin-top: 28rpx;
}
.group {
  margin-top: 42rpx;
}
.category {
  font-size: 32rpx;
  font-weight: 700;
  margin-bottom: 20rpx;
}
.product {
  background: #ffffffd9;
  border-radius: 16rpx;
  padding: 26rpx;
  margin-bottom: 20rpx;
  color: #242e28;
}
.product-head {
  display: flex;
  justify-content: space-between;
  gap: 16rpx;
}
.name {
  font-size: 32rpx;
  font-weight: 700;
}
.badge {
  font-size: 22rpx;
  color: #a3392b;
}
.description,
.muted {
  font-size: 24rpx;
  line-height: 1.7;
  margin-top: 12rpx;
}
.prices {
  display: grid;
  gap: 12rpx;
  margin-top: 20rpx;
  border-top: 1rpx solid #ddd;
  padding-top: 18rpx;
}
.price {
  display: flex;
  justify-content: space-between;
  gap: 20rpx;
  font-size: 25rpx;
}
.red {
  color: #ad3027;
}
.footer {
  text-align: center;
  margin-top: 38rpx;
  font-size: 22rpx;
}
</style>
