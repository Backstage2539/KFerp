<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import EnvironmentBadge from '../../components/EnvironmentBadge.vue'
import PullUpBrandFooter from '../../components/PullUpBrandFooter.vue'
import { usePullUpBrandGesture } from '../../composables/usePullUpBrandGesture'
import { defaultMiniappShare, defaultMiniappTimelineShare, refreshMiniappShareMenu } from '../../utils/miniappShare'

const {
  pullUpBrandRevealed,
  handlePullUpBrandTouchStart,
  handlePullUpBrandTouchMove,
  handlePullUpBrandTouchEnd,
  handlePullUpBrandTouchCancel,
} = usePullUpBrandGesture()
const operator = '孟连口加农业科技有限公司'
const contactEmail = '15302787466@163.com'
const kind = ref<'terms' | 'privacy'>('terms')

const agreements = {
  terms: {
    title: '用户服务协议',
    summary: '本协议适用于使用本小程序的员工、客户和访客。登录前请阅读并同意本协议及《隐私政策》。你可以直接浏览公开价格表。员工代客户维护资料或录单时，请先确认已取得收件人的授权。',
    sections: [
      {
        title: '账号和使用权限',
        paragraphs: [
          '你可以直接浏览公开价格表。查看绑定客户的专属价格、提交订单等业务，需要通过身份验证，并在账号获准使用的客户和业务范围内操作。',
          '若代表企业或门店操作，请确认你有权办理相应业务。请妥善保管登录凭证，使用共用设备后及时退出。发现账号被误绑或由他人使用时，请通过文末邮箱联系我们。',
        ],
      },
      {
        title: '下单、寄样和履约',
        paragraphs: [
          '提交前请核对商品、规格、数量、价格、费用、交付方式及收件信息。寄样申请的规格、数量、费用和寄送安排，可在工作人员确认时核对。已有书面供货合同的订单，依具体合同和双方确认的订单履行。',
          '需要变更或取消订单、反映配送或商品质量问题时，请提供相应订单号并联系业务联系人或文末邮箱。我们将按履约进度、双方约定及适用法律处理。消费者依法享有的售后及其他权利受法律保护。',
        ],
      },
      {
        title: '收件人和地址簿',
        paragraphs: [
          '为自己以外的人下单或申请寄样时，请先向收件人说明信息将用于哪些业务，并取得其授权。请只填写本次履约需要的信息，提交前核对姓名、电话和地址。',
          '在支持地址簿的下单页面，可选“仅本单使用”，也可选择保存到当前客户的地址簿。同一客户名下的其他账号可以查看和复用已保存的地址。为他人做一件代发时，请先确认对方同意把地址留作以后复用，再选择保存。',
        ],
      },
      {
        title: '资料使用和服务变更',
        paragraphs: [
          '请在获准的业务范围内使用价格表、订单资料和客户资料。不得利用小程序冒用身份、提交虚假订单或披露他人收件信息。',
          '我们按照《隐私政策》处理个人信息。若调整服务规则，我们将在本小程序公布新版本，并对影响你权益的重要调整作出提示。',
        ],
      },
      {
        title: '意见与争议处理',
        paragraphs: [
          '你可以通过下方邮箱联系运营方处理订单或服务争议。协商未能解决的，可依法投诉、申请调解或向有管辖权的人民法院提起诉讼。',
        ],
      },
    ],
  },
  privacy: {
    title: '隐私政策',
    summary: '本政策说明我们为了哪些业务处理个人信息、信息由谁使用、保存多久，以及你可以怎样管理这些信息。',
    sections: [
      {
        title: '登录和身份验证',
        paragraphs: [
          '选择账号密码登录时，你输入的用户名或手机号和密码会发送至服务器验证。登录成功后，本设备按账号记录同意的协议版本和时间，用于恢复已确认的会话。员工或客户可从个人中心退出登录。',
          '为登录或恢复登录状态，我们使用微信登录凭证识别小程序账号，并使用会话凭证、账号与客户的绑定关系及权限，显示你有权访问的客户资料。当你主动选择“授权手机号登录”时，我们通过微信授权获取手机号，用于识别已绑定的员工、客户或访客账号。',
          '个人中心可能显示你此前填写的昵称或已绑定的客户名称。手机号登记不需要微信头像和微信昵称。你可以返回价格表中心浏览公开内容。',
          '客户账号绑定后，个人中心会按你的权限显示客户档案中已有的联系人、联系电话、联系地址和业务联系人，供你核对和办理业务。若档案资料有误，可联系业务联系人或通过文末邮箱申请更正。',
        ],
      },
      {
        title: '填写、解析及使用收件信息',
        paragraphs: [
          '在商城下单、申请寄样或使用一件代发等配送服务时，我们使用收件人姓名、联系电话、收货地址，以及你选填的公司、门店或订单备注，用于处理订单、联系收件人、配送和处理相关售后。',
          '你可以手动逐项填写。员工代客户维护档案或录单时，收件资料会保存到相应的客户档案或订单。使用地址解析时，你先自行将文字粘贴到输入框，点击“解析收件信息”后，该段原文会发送到我们的服务器，提取姓名、电话和地址供你核对。请只粘贴收件所需的内容。',
          '首次使用收件信息功能时，我们会请你阅读协议并选择是否同意。若选择退出，可以继续浏览公开价格表。使用“仅本单使用”时，我们将收件信息用于当前订单及其履约记录。下单草稿可能暂存你正在填写的信息，供你继续本单操作。',
        ],
      },
      {
        title: '谁可以使用这些信息',
        paragraphs: [
          '选择“保存”或“保存并使用”后，地址将存入当前客户共用的地址簿，该客户名下的其他账号也可以查看和复用。为他人寄送时，请向对方说明用途并取得授权。保存到共用地址簿前，还请向其说明这一可见范围。',
          '我们按账号权限向办理订单、仓储、配送或售后的人员展示其业务所需的信息。需要委托承运方配送时，我们按该笔寄送的需要向承运方提供收件人姓名、电话和地址。若需向独立决定处理目的和方式的其他主体提供个人信息，我们将依法告知其名称、联系方式、处理目的、方式和信息种类，并按法律要求另行取得同意。',
        ],
      },
      {
        title: '保存期限与保护',
        paragraphs: [
          '账号识别和绑定资料在提供账号服务及核查相关权利所需期间保存。常用地址在你选择保留且仍需复用期间保存，你可以删除。订单及寄样记录在履约、售后、争议处理和适用法律要求的留存期内保存。相应用途结束且法定期限届满后，我们将删除相关个人信息或作匿名化处理。',
          '我们按角色和客户范围控制资料访问。删除地址簿中的地址后，已提交的订单仍可按上述期限保存其履约时使用的收件资料。',
        ],
      },
      {
        title: '查询、修改、删除及撤回同意',
        paragraphs: [
          '你可以进入“个人中心 → 管理收件地址”查看、编辑或删除常用地址。在个人中心选择“撤回收件信息授权”，可撤回当前授权。撤回后，若再次使用该功能，需重新选择是否同意。你仍可使用无需收件信息的功能。',
          '无论你是账号使用人还是实际收件人，均可通过下方邮箱申请查阅、复制、更正、删除自己的个人信息，申请撤回同意、注销账号或了解处理规则。请说明请求和可协助定位记录的信息，例如订单号。我们仅在必要范围内核实身份后依法处理并答复。',
        ],
      },
      {
        title: '政策更新',
        paragraphs: [
          '我们会在本页公布更新后的内容，并提示对你权益有重大影响的调整。基于同意处理信息时，如处理目的、方式或信息种类发生变化，我们会重新征求同意。',
        ],
      },
    ],
  },
}

const agreement = computed(() => agreements[kind.value])

onLoad((query) => {
  kind.value = query?.type === 'privacy' ? 'privacy' : 'terms'
  uni.setNavigationBarTitle({ title: agreement.value.title })
})
onShow(() => { void refreshMiniappShareMenu() })
onShareAppMessage(defaultMiniappShare)
onShareTimeline(defaultMiniappTimelineShare)
</script>

<template>
  <view
    class="agreement-page pull-up-brand-page"
    @touchstart="handlePullUpBrandTouchStart"
    @touchmove="handlePullUpBrandTouchMove"
    @touchend="handlePullUpBrandTouchEnd"
    @touchcancel="handlePullUpBrandTouchCancel"
  >
    <EnvironmentBadge />
    <text class="title">{{ agreement.title }}</text>
    <text class="meta">棵凡咖啡供应链 · 运营方　{{ operator }}</text>
    <text class="meta">版本日期　2026年11月24日</text>
    <text class="summary" selectable>{{ agreement.summary }}</text>
    <view v-for="section in agreement.sections" :key="section.title" class="section">
      <text class="heading">{{ section.title }}</text>
      <text v-for="paragraph in section.paragraphs" :key="paragraph" class="paragraph" selectable>{{ paragraph }}</text>
    </view>
    <view class="section">
      <text class="heading">联系我们</text>
      <text class="paragraph" selectable>{{ operator }} · 服务与隐私联系邮箱</text>
      <text class="email" selectable>{{ contactEmail }}</text>
    </view>
    <view class="pull-up-brand-footer-anchor">
      <PullUpBrandFooter :revealed="pullUpBrandRevealed" />
    </view>
  </view>
</template>

<style scoped>
.agreement-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: 44rpx 36rpx 72rpx;
  color: #26372f;
  background: #fff;
}
.title, .meta, .summary, .heading, .paragraph, .email { display: block; }
.title { margin-bottom: 16rpx; font-size: 46rpx; font-weight: 700; line-height: 1.3; }
.meta { color: #53645b; font-size: 26rpx; line-height: 1.7; }
.summary { margin-top: 30rpx; }
.section { margin-top: 36rpx; }
.heading { color: #173b2e; font-size: 34rpx; font-weight: 700; }
.summary, .paragraph, .email { font-size: 30rpx; line-height: 1.75; }
.paragraph { margin-top: 12rpx; }
.email { color: #235640; word-break: break-all; }
</style>
