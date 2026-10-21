import { describe,it,expect } from 'vitest'
import { employeeOrderProductCategoriesForFamilies, filterEmployeeOrderProductFamilies } from './employeeOrder'
import type { EmployeeOrderProductFamily } from '../api/customerPortal'

describe('published-table categories',()=>{
  it('shows arbitrary published categories and keeps classification independent from product kind',()=>{
    const families = [
      {parent_product_id:1,name:'新品',product_kind:'generic',price_table_categories:[{key:'classification:464',label:'茶饮与新品',publication_id:113}],specs:[]},
      {parent_product_id:2,name:'咖啡',product_kind:'roasted',price_table_categories:[{key:'classification:56',label:'烘焙咖啡豆',publication_id:158}],specs:[]},
    ] as EmployeeOrderProductFamily[]
    expect(employeeOrderProductCategoriesForFamilies(families,487)).toEqual([{key:'all',label:'全部'},{key:'classification:464',label:'茶饮与新品'},{key:'classification:56',label:'烘焙咖啡豆'}])
    expect(filterEmployeeOrderProductFamilies(families,487,'','classification:464').map(row=>row.name)).toEqual(['新品'])
  })
})
