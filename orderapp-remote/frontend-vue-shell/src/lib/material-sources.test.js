import test from 'node:test'
import assert from 'node:assert/strict'
import {materialSourcesPayload} from './material-sources.js'
test('warehouse-only saves never write historical operation associations',()=>{
 assert.deepEqual(materialSourcesPayload({version:'v1',rows:[{key:'a',binding:{node_key:'legacy',source_warehouse:'raw'}}]},9),{expected_version:'v1',variant_id:9,bindings:{a:{source_warehouse:'raw'}}})
})
