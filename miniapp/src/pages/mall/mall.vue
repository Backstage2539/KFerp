<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { buildAPIURL } from '../../api/client'
import { createMallOrder, fetchMallPage, type MallPageResponse } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'
import {
  addMallCartItem,
  buildMallOrderPayload,
  formatMallMoney,
  mallCartCount,
  mallCartTotal,
  normalizeMallProduct,
  updateMallCartQty,
  type MallCartItem,
  type MallProduct,
} from '../../utils/mall'
import { miniappThemeClass, miniappThemeMeta } from '../../utils/themes'

const session = useSessionStore()
const page = ref<MallPageResponse | null>(null)
const cart = ref<MallCartItem[]>([])
const loading = ref(false)
const submitting = ref(false)
const errorMessage = ref('')
const recipient = ref({ name: '', phone: '', address: '', note: '' })

const products = computed(() => (page.value?.products || []).map((item) => normalizeMallProduct(item)))
const activeThemeKey = computed(() => page.value?.theme_key || session.themeKey)
const themeClass = computed(() => miniappThemeClass(activeThemeKey.value))
const themeMeta = computed(() => miniappThemeMeta(activeThemeKey.value))
const customerName = computed(() => page.value?.current_customer_name || session.currentCustomerName || '商城')
const cartCount = computed(() => mallCartCount(cart.value))
const cartTotal = computed(() => mallCartTotal(cart.value))

async function loadMall() {
  if (!session.token) {
    uni.redirectTo({ url: '/pages/login/login' })
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    const data = await fetchMallPage(session.token)
    page.value = data
    session.applyContext({
      mini_user_id: session.miniUserID,
      current_customer_id: data.current_customer_id || session.currentCustomerID,
      current_customer_name: data.current_customer_name || session.currentCustomerName,
      theme_key: data.theme_key || session.themeKey,
      miniapp_entry_mode: data.miniapp_entry_mode || session.entryMode,
      bindings: session.bindings,
      capabilities: session.capabilities,
    })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '商城加载失败'
  } finally {
    loading.value = false
  }
}

function productImageURL(product: MallProduct): string {
  const url = product.image_url
  if (!url) return ''
  if (/^https?:\/\//.test(url)) return url
  return buildAPIURL(url)
}

function addProduct(product: MallProduct) {
  cart.value = addMallCartItem(cart.value, product, 1)
}

function changeQty(item: MallCartItem, delta: number) {
  cart.value = updateMallCartQty(cart.value, item.mall_product_id, item.qty + delta)
}

function openOrders() {
  uni.navigateTo({ url: '/pages/service/service?key=orders' })
}

async function submitOrder() {
  const payload = buildMallOrderPayload(recipient.value, cart.value)
  if (!payload.recipient_name || !payload.recipient_phone || !payload.recipient_address) {
    errorMessage.value = '请填写收货信息'
    return
  }
  if (!payload.items.length) {
    errorMessage.value = '请先选择商品'
    return
  }
  submitting.value = true
  errorMessage.value = ''
  try {
    const order = await createMallOrder(session.token, payload)
    cart.value = []
    recipient.value.note = ''
    uni.showToast({ title: order.order_no || '已提交', icon: 'success' })
  } catch (error) {
    errorMessage.value = error instanceof Error ? error.message : '提交失败'
  } finally {
    submitting.value = false
  }
}

onShow(() => {
  void loadMall()
})
</script>

<template>
  <view class="page" :class="themeClass">
    <view class="header">
      <text class="eyebrow">{{ themeMeta.eyebrow }}</text>
      <text class="title">{{ customerName }}</text>
      <text class="subtitle">{{ themeMeta.subtitle }}</text>
      <button class="orders-link" size="mini" @tap="openOrders">我的订单</button>
    </view>

    <view v-if="loading" class="state">
      <text>加载中...</text>
    </view>

    <view v-else-if="errorMessage" class="state error">
      <text>{{ errorMessage }}</text>
    </view>

    <view v-else>
      <view class="products">
        <view
          v-for="product in products"
          :key="product.id"
          :class="['product', product.template_key]"
          hover-class="product-pressed">
          <image v-if="productImageURL(product)" class="product-image" mode="aspectFill" :src="productImageURL(product)" />
          <view v-else class="product-image empty-image"></view>
          <view class="product-body">
            <text class="product-title">{{ product.title }}</text>
            <text class="product-subtitle">{{ product.subtitle || `${product.spec_g}g` }}</text>
            <text class="product-description">{{ product.description }}</text>
            <view class="product-foot">
              <text class="price">{{ formatMallMoney(product.unit_price) }}</text>
              <button class="add-btn" size="mini" @tap="addProduct(product)">加入</button>
            </view>
          </view>
        </view>
      </view>

      <view v-if="!products.length" class="state">
        <text>暂无上架商品</text>
      </view>

      <view class="checkout">
        <view class="checkout-head">
          <text>购物车 {{ cartCount }}</text>
          <text>{{ formatMallMoney(cartTotal) }}</text>
        </view>

        <view v-if="cart.length" class="cart-lines">
          <view v-for="item in cart" :key="item.mall_product_id" class="cart-line">
            <text class="cart-title">{{ item.title }}</text>
            <view class="qty">
              <button size="mini" @tap="changeQty(item, -1)">-</button>
              <text>{{ item.qty }}</text>
              <button size="mini" @tap="changeQty(item, 1)">+</button>
            </view>
          </view>
        </view>

        <view class="form">
          <input v-model="recipient.name" class="input" placeholder="收件人" />
          <input v-model="recipient.phone" class="input" placeholder="手机号" />
          <input v-model="recipient.address" class="input" placeholder="收货地址" />
          <textarea v-model="recipient.note" class="textarea" placeholder="备注" />
          <button class="submit" :disabled="submitting" @tap="submitOrder">
            {{ submitting ? '提交中' : '提交订单' }}
          </button>
        </view>
      </view>
    </view>
  </view>
</template>

<style scoped>
.page {
  min-height: 100vh;
  padding: 32rpx;
  background: #f7f2ea;
  box-sizing: border-box;
}

.page.theme-coffee-factory { background: #f7f2ea; }
.page.theme-clean-ops { background: #f5f7f6; }
.page.theme-premium-partner { background: #fbf7ef; }

.header {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  padding: 30rpx 28rpx 34rpx;
  margin-bottom: 24rpx;
  border-radius: 28rpx;
  background: linear-gradient(135deg, #2b2118 0%, #6b4b2b 100%);
}

.theme-clean-ops .header {
  background: #ffffff;
  border: 1rpx solid #dfe7e2;
}

.theme-premium-partner .header {
  background: linear-gradient(135deg, #111111 0%, #513018 55%, #b88a46 100%);
}

.eyebrow {
  color: rgba(255, 248, 235, .78);
  font-size: 24rpx;
  font-weight: 900;
}

.theme-clean-ops .eyebrow { color: #28624a; }

.title {
  color: #fff8eb;
  font-size: 42rpx;
  font-weight: 900;
  line-height: 1.18;
}

.theme-clean-ops .title { color: #14201a; }

.subtitle {
  color: rgba(255, 248, 235, .82);
  font-size: 26rpx;
  line-height: 1.55;
}

.theme-clean-ops .subtitle { color: #66756c; }

.orders-link {
  align-self: flex-start;
  min-height: 58rpx;
  margin: 8rpx 0 0;
  padding: 0 24rpx;
  border: 1rpx solid rgba(255, 248, 235, .58);
  border-radius: 999rpx;
  background: rgba(255, 255, 255, .12);
  color: #fff8eb;
  font-size: 24rpx;
  font-weight: 900;
  line-height: 58rpx;
}

.theme-clean-ops .orders-link {
  border-color: #cddbd4;
  background: #eef6f2;
  color: #28624a;
}

.theme-premium-partner .orders-link {
  border-color: rgba(255, 248, 235, .5);
  background: rgba(255, 248, 235, .14);
}

.products {
  display: grid;
  grid-template-columns: 1fr;
  gap: 22rpx;
}

.product {
  overflow: hidden;
  border: 1rpx solid #ead9bd;
  border-radius: 20rpx;
  background: #fffaf2;
}

.theme-clean-ops .product {
  background: #ffffff;
  border-color: #dde7e1;
}

.product.compact {
  display: grid;
  grid-template-columns: 220rpx minmax(0, 1fr);
}

.product-image {
  display: block;
  width: 100%;
  height: 360rpx;
  background: #e8e1d5;
}

.product.compact .product-image {
  height: 100%;
  min-height: 220rpx;
}

.product.wide .product-image {
  height: 260rpx;
}

.empty-image {
  background: linear-gradient(135deg, #e7efe9, #d7c4a2);
}

.product-body {
  display: flex;
  flex-direction: column;
  gap: 10rpx;
  padding: 24rpx;
}

.product-title {
  color: #171717;
  font-size: 34rpx;
  font-weight: 900;
  line-height: 1.25;
}

.product-subtitle,
.product-description {
  color: #666666;
  font-size: 25rpx;
  line-height: 1.5;
}

.product-foot,
.checkout-head,
.cart-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16rpx;
}

.price {
  color: #8a4b1f;
  font-size: 34rpx;
  font-weight: 900;
}

.add-btn,
.submit {
  background: #1f1f1f;
}

.product-pressed {
  transform: scale(.995);
  opacity: .9;
}

.checkout {
  margin-top: 28rpx;
  padding: 24rpx;
  border: 1rpx solid #ead9bd;
  border-radius: 20rpx;
  background: #ffffff;
}

.theme-clean-ops .checkout {
  border-color: #dde7e1;
}

.checkout-head {
  color: #171717;
  font-size: 30rpx;
  font-weight: 900;
}

.cart-lines {
  margin-top: 16rpx;
  border-top: 1rpx solid #eef1f4;
}

.cart-line {
  min-height: 76rpx;
  border-bottom: 1rpx solid #eef1f4;
}

.cart-title {
  min-width: 0;
  color: #171717;
  font-size: 27rpx;
  font-weight: 800;
}

.qty {
  display: flex;
  align-items: center;
  gap: 12rpx;
  color: #171717;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 14rpx;
  margin-top: 20rpx;
}

.input,
.textarea {
  width: 100%;
  min-height: 78rpx;
  padding: 0 22rpx;
  border: 1rpx solid #d9cec0;
  border-radius: 14rpx;
  background: #fffdf8;
  color: #171717;
  font-size: 28rpx;
  box-sizing: border-box;
}

.textarea {
  min-height: 128rpx;
  padding-top: 18rpx;
  line-height: 1.45;
}

.submit {
  min-height: 82rpx;
  font-weight: 900;
}

.state {
  min-height: 180rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #666666;
  font-size: 28rpx;
}

.error {
  color: #b42318;
}
</style>
