import test from 'node:test'
import assert from 'node:assert/strict'
import {manualFreightCents,freightAmount,freightStateFromOrder,regionDestination,searchOrderRegions} from './order-freight.js'
test('blank auto, explicit zero, manual and free are distinct',()=>{
 assert.equal(manualFreightCents(''),null);assert.equal(manualFreightCents('0'),0);assert.equal(manualFreightCents('20.01'),2001)
 for(const value of ['-1','1.001','abc'])assert.throws(()=>manualFreightCents(value))
 assert.equal(freightAmount({manual:'20',free:false}),20);assert.equal(freightAmount({manual:'20',free:true}),0)
 assert.equal(freightStateFromOrder({shipping_amount:0}).manual,'0.00')
 assert.equal(freightStateFromOrder({freight:{protocol:'deferred_freight_v1',policy:{free_shipping:false},destination:{province:'云南省'}}}).manual,'')
})
test('selecting district returns whole code path and searches server aliases',()=>{
 const rows=[{code:'530000',name:'云南省',level:1},{code:'530800',parent_code:'530000',name:'普洱市',level:2},{code:'530827',parent_code:'530800',name:'孟连傣族拉祜族佤族自治县',alias:'孟连县',level:3,path:'云南省 / 普洱市 / 孟连傣族拉祜族佤族自治县'}]
 assert.equal(searchOrderRegions(rows,'孟连县')[0].code,'530827');assert.deepEqual(regionDestination(rows,'530827'),{province:'云南省',city:'普洱市',district:'孟连傣族拉祜族佤族自治县',province_code:'530000',city_code:'530800',district_code:'530827'})
})
