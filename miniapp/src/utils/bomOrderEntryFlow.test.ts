import { it, expect } from 'vitest'
import { readFileSync } from 'node:fs'
const page=readFileSync(new URL('../pages/employee-order-entry/employee-order-entry.vue',import.meta.url),'utf8')
it('employee order entry only offers price-table specs and no temporary conversion entry',()=>{
 expect(page).not.toMatch(/loadBOMSpecs|chooseBOMTier|EmployeeSpecConversionSheet|临时换规格/)
 expect(page).toContain('familyForItem(item)?.specs.map(productSpecLabel)')
})
