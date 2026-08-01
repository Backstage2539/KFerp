const environment = String(process.env.VITE_KFERP_ENVIRONMENT || '').trim()
const apiBase = String(process.env.VITE_KFERP_API_BASE || '').trim().replace(/\/+$/, '')
const expectedAPIBase = {
  development: 'https://dev.qacoohee.com/app',
  production: 'https://erp.qacoohee.com/app',
}[environment]

if (!expectedAPIBase) {
  console.error('ERROR: VITE_KFERP_ENVIRONMENT must be development or production')
  process.exit(1)
}
if (apiBase !== expectedAPIBase) {
  console.error(`ERROR: VITE_KFERP_API_BASE does not match ${environment}`)
  process.exit(1)
}

console.log(`miniapp environment verified: ${environment}`)
