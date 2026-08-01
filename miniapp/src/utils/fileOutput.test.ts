import { describe, expect, it } from 'vitest'
import { downloadFailureMessage } from './fileOutput'

describe('downloadFailureMessage', () => {
  it('explains mini program download domain failures', () => {
    const message = downloadFailureMessage(
      'https://dev.qacoohee.com 不在以下 downloadFile 合法域名列表中',
      'https://dev.qacoohee.com/app',
    )

    expect(message.title).toBe('下载域名未配置')
    expect(message.content).toContain('downloadFile 合法域名')
    expect(message.content).toContain('https://dev.qacoohee.com')
    expect(message.content).not.toContain('关闭合法域名校验')
  })

  it('uses the production origin for production download failures', () => {
    const message = downloadFailureMessage(
      'url not in domain list',
      'https://erp.qacoohee.com/app',
    )

    expect(message.content).toContain('https://erp.qacoohee.com')
  })

  it('keeps generic copy for ordinary download failures', () => {
    expect(downloadFailureMessage('downloadFile:fail timeout')).toEqual({ title: '文件下载失败' })
  })
})
