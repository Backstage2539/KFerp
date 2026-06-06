import { describe, expect, it } from 'vitest'
import { downloadFailureMessage } from './fileOutput'

describe('downloadFailureMessage', () => {
  it('explains mini program download domain failures', () => {
    const message = downloadFailureMessage('https://erp.qacoohee.com 不在以下 downloadFile 合法域名列表中')

    expect(message.title).toBe('下载域名未配置')
    expect(message.content).toContain('downloadFile 合法域名')
    expect(message.content).toContain('https://erp.qacoohee.com')
  })

  it('keeps generic copy for ordinary download failures', () => {
    expect(downloadFailureMessage('downloadFile:fail timeout')).toEqual({ title: '文件下载失败' })
  })
})
