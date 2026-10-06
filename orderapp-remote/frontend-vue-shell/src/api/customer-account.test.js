import test from 'node:test'
import assert from 'node:assert/strict'

import { customerFileDownloadErrorMessage } from './customer-account.js'

test('customer file download explains an expired session', () => {
  assert.equal(
    customerFileDownloadErrorMessage({ status: 401 }, {}),
    '登录已过期，请先打开 /app/login 重新登录后再下载',
  )
})

test('customer file download explains missing permission', () => {
  assert.equal(
    customerFileDownloadErrorMessage({ status: 403 }, {}),
    '当前账号无权下载此文件',
  )
})

test('customer file download preserves server errors for other failures', () => {
  assert.equal(
    customerFileDownloadErrorMessage({ status: 500 }, { error: '文件服务暂不可用' }),
    '文件服务暂不可用',
  )
})
