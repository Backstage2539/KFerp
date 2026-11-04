export function materialSourcesPayload(config,variantID){
 return {expected_version:config.version,variant_id:variantID,bindings:Object.fromEntries(config.rows.map(row=>[row.key,{source_warehouse:row.binding?.source_warehouse||''}]))}
}
