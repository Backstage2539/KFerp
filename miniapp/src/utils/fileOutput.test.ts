import { describe, expect, it } from 'vitest'
import { isAuthenticationExpiredRequestError } from '../api/client'
import { downloadFailureMessage, miniappDownloadStatusError, shareDownloadedMiniappFile } from './fileOutput'

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

  it('preserves authenticated download status for the page retry and login flow', () => {
    const expired = miniappDownloadStatusError(401)
    expect(isAuthenticationExpiredRequestError(expired)).toBe(true)
    expect(miniappDownloadStatusError(403).message).toBe('当前员工无此权限')
    expect(miniappDownloadStatusError(404).statusCode).toBe(404)
  })
})

describe('shareDownloadedMiniappFile', () => {
  it('shares PDF files through WeChat when the capability is available', async () => {
    const calls: string[] = []
    await shareDownloadedMiniappFile({
      filePath: '/tmp/sales-order.pdf',
      fileName: '销售单.pdf',
      kind: 'pdf',
    }, {
      shareFileMessage: (options) => {
        calls.push(`share:${options.fileName}`)
        options.success?.({ errMsg: 'ok' })
      },
      openDocument: () => calls.push('open'),
    })
    expect(calls).toEqual(['share:销售单.pdf'])
  })

  it('falls back to openDocument with the system share menu for unsupported or failed PDF sharing', async () => {
    const calls: string[] = []
    await shareDownloadedMiniappFile({ filePath: '/tmp/sales-order.pdf', kind: 'pdf' }, {
      shareFileMessage: (options) => options.fail?.({ errMsg: 'not supported' }),
      openDocument: (options) => {
        calls.push(`${options.fileType}:${String(options.showMenu)}`)
        options.success?.({ errMsg: 'ok' })
      },
    })
    expect(calls).toEqual(['pdf:true'])
  })

  it('shares images through showShareImageMenu and falls back to previewImage', async () => {
    const shared: string[] = []
    await shareDownloadedMiniappFile({ filePath: '/tmp/sales-order.png', kind: 'png' }, {
      showShareImageMenu: (options) => {
        shared.push(options.path)
        options.success?.({ errMsg: 'ok' })
      },
      previewImage: () => shared.push('preview'),
    })
    expect(shared).toEqual(['/tmp/sales-order.png'])

    const fallback: string[] = []
    await shareDownloadedMiniappFile({ filePath: '/tmp/sales-order.png', kind: 'png' }, {
      showShareImageMenu: (options) => options.fail?.({ errMsg: 'not supported' }),
      previewImage: (options) => {
        fallback.push(options.current || '')
        options.success?.({ errMsg: 'ok' })
      },
    })
    expect(fallback).toEqual(['/tmp/sales-order.png'])
  })
})
