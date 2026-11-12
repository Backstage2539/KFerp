import test from 'node:test'
import assert from 'node:assert/strict'
import { buildSpecPricingGroupPayload, validateSpecPricingGroup, canDisableUnchangedSpecPricingGroup } from './spec-pricing-groups.js'

const templates = [{id:1, versions:[{id:11, variants:[{spec_key:'s1',inventory_unit:'bag',name:'227g'}, {spec_key:'s2',inventory_unit:'bag',name:'100g'}]}]}]
const rules = [{id:2,active:true},{id:3,active:false}]

test('组必须绑定已发布版本，且每个所选规格都使用有效价格模板', () => {
  const form = {name:' 测试组 ',spec_template_id:1,spec_template_version_id:11,active:true,items:[{spec_key:'s1',inventory_unit:'bag',pricing_rule_id:2}]}
  assert.equal(validateSpecPricingGroup(form,templates,rules), '')
  assert.match(validateSpecPricingGroup({...form,items:[]},templates,rules), /规格/)
  assert.match(validateSpecPricingGroup({...form,items:[{...form.items[0],pricing_rule_id:3}]},templates,rules), /价格模板/)
  assert.match(validateSpecPricingGroup({...form,spec_template_version_id:22},templates,rules), /版本/)
  assert.match(validateSpecPricingGroup({...form,items:[form.items[0],form.items[0]]},templates,rules), /重复/)
  assert.match(validateSpecPricingGroup({...form,items:[{...form.items[0],inventory_unit:'kg'}]},templates,rules), /规格/)
})

test('保存只发送明确选择的规格并保留修订号', () => {
  const payload = buildSpecPricingGroupPayload({id:4,name:' 小包装 ',spec_template_id:'1',spec_template_version_id:'11',revision:3,active:true,items:[{spec_key:'s1',inventory_unit:'bag',pricing_rule_id:'2',name:'227g'}]})
  assert.deepEqual(payload,{id:4,name:'小包装',spec_template_id:1,spec_template_version_id:11,revision:3,active:true,items:[{spec_key:'s1',inventory_unit:'bag',pricing_rule_id:2,sort_order:1}]})
})


test('只允许原有映射不变时停用失效组', () => {
  const saved={id:8,name:'旧组',revision:2,active:true,spec_template_id:1,spec_template_version_id:11,items:[{spec_key:'s1',inventory_unit:'bag',pricing_rule_id:3,sort_order:1}]}
  assert.equal(canDisableUnchangedSpecPricingGroup({...saved,active:false},saved), true)
  assert.equal(canDisableUnchangedSpecPricingGroup({...saved,active:false,items:[{...saved.items[0],pricing_rule_id:2}]},saved), false)
  assert.equal(canDisableUnchangedSpecPricingGroup({...saved,active:true},saved), false)
})
