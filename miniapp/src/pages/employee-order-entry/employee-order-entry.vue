<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { createEmployeeOrder, fetchEmployeeOrderForm, type EmployeeOrderForm } from '../../api/customerPortal'
import { useSessionStore } from '../../stores/session'

const session = useSessionStore()
const formData = ref<EmployeeOrderForm>()
const saving = ref(false)
const form = ref({ order_date:'', customer_id:0, source_id:0, order_type_id:0, pay_status_id:0, ship_status_id:0, receiver_name:'', receiver_phone:'', receiver_address:'', notes:'', product_id:0, product_name:'', product_kind:'roasted', qty:1, spec_g:454, unit_price:0 })
const customerLabels = computed(() => formData.value?.customers.map(v => v.name) || [])
const productLabels = computed(() => formData.value?.products.map(v => v.name) || [])
function chooseCustomer(e:any){const row=formData.value?.customers[Number(e.detail.value)];if(row)form.value.customer_id=row.id}
function chooseProduct(e:any){const row=formData.value?.products[Number(e.detail.value)];if(row){form.value.product_id=row.id;form.value.product_name=row.name;form.value.product_kind=row.product_kind||'roasted'}}
async function submit(){
  if(!form.value.customer_id||!form.value.product_id){uni.showToast({title:'请选择客户和商品',icon:'none'});return}
  saving.value=true
  try{
    const result=await createEmployeeOrder(session.token,{order_date:form.value.order_date,customer_id:form.value.customer_id,source_id:form.value.source_id,order_type_id:form.value.order_type_id,pay_status_id:form.value.pay_status_id,ship_status_id:form.value.ship_status_id,receiver_name:form.value.receiver_name,receiver_phone:form.value.receiver_phone,receiver_address:form.value.receiver_address,notes:form.value.notes,items:[{product_id:form.value.product_id,name:form.value.product_name,product_kind:form.value.product_kind,qty:Number(form.value.qty),spec_g:Number(form.value.spec_g),unit:'袋',sales_unit:'bag',unit_price:Number(form.value.unit_price)}]})
    uni.showModal({title:'录单成功',content:result.order_no,showCancel:false,success:()=>uni.navigateTo({url:'/pages/employee-orders/employee-orders'})})
  }catch(cause){uni.showToast({title:cause instanceof Error?cause.message:'录单失败',icon:'none'})}finally{saving.value=false}
}
onLoad(async()=>{formData.value=await fetchEmployeeOrderForm(session.token);form.value.order_date=formData.value.today;form.value.source_id=formData.value.sources[0]?.id||0;form.value.order_type_id=formData.value.order_types[0]?.id||0;form.value.pay_status_id=formData.value.pay_statuses[0]?.id||0;form.value.ship_status_id=formData.value.ship_statuses[0]?.id||0})
</script>

<template>
  <view class="page"><view class="panel">
    <text class="title">新建销售订单</text>
    <picker mode="date" :value="form.order_date" @change="form.order_date=($event.detail as any).value"><view class="field">{{ form.order_date || '订单日期' }}</view></picker>
    <picker :range="customerLabels" @change="chooseCustomer"><view class="field">{{ formData?.customers.find(v=>v.id===form.customer_id)?.name || '选择客户 *' }}</view></picker>
    <picker :range="productLabels" @change="chooseProduct"><view class="field">{{ form.product_name || '选择商品 *' }}</view></picker>
    <view class="row"><input v-model="form.spec_g" type="number" class="field" placeholder="规格克重"/><input v-model="form.qty" type="number" class="field" placeholder="数量"/></view>
    <input v-model="form.unit_price" type="digit" class="field" placeholder="销售单价"/>
    <input v-model="form.receiver_name" class="field" placeholder="收货人"/>
    <input v-model="form.receiver_phone" type="number" class="field" placeholder="联系电话"/>
    <textarea v-model="form.receiver_address" class="field area" placeholder="收货地址"/>
    <textarea v-model="form.notes" class="field area" placeholder="备注"/>
    <button class="submit" :loading="saving" :disabled="saving" @tap="submit">提交订单</button>
  </view></view>
</template>

<style scoped>
.page{min-height:100vh;padding:28rpx;background:#f5f7f6;box-sizing:border-box}.panel{padding:28rpx;background:#fff;border-radius:18rpx}.title{display:block;margin-bottom:24rpx;font-size:36rpx;font-weight:800}.field{min-height:82rpx;margin-bottom:18rpx;padding:20rpx;border:1rpx solid #dfe7e2;border-radius:12rpx;box-sizing:border-box;background:#fff}.row{display:flex;gap:16rpx}.row .field{flex:1}.area{width:100%;height:130rpx}.submit{background:#28624a;color:#fff}
</style>
