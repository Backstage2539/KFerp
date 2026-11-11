import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { expect, it } from 'vitest'
it('uses a server controlled masked password entry while retaining phone login', () => {
  const source = readFileSync(resolve('src/pages/login/login.vue'), 'utf8')
  expect(source).toContain('fetchMiniLoginOptions')
  expect(source).toContain('v-if="showPasswordLogin"')
  expect(source).toContain('loginWithPassword')
  expect(source).toContain('password placeholder="密码"')
  expect(source).toContain('loginWithPhoneIdentify')
  expect(source).toContain('loginAttempt')
})
