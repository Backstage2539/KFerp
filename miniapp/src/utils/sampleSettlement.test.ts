import {describe,it,expect} from 'vitest'
import {isFreeSampleOrder} from './sampleSettlement'
describe('free sample payment',()=>{it('requires trusted sample and zero goods, freight and other fees',()=>{expect(isFreeSampleOrder(true,[0],0,0,0)).toBe(true);for(const values of [[false,[0],0,0,0],[true,[1],0,0,0],[true,[0],8,0,8],[true,[0],0,2,2],[true,[],0,0,0]] as [boolean,number[],number,number,number][]) expect(isFreeSampleOrder(...values)).toBe(false)})})
