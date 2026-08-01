import { buildAPIURL } from '../api/client'

export type MiniappFileOutputKind = 'pdf' | 'png'

export type DownloadFailureMessage = {
  title: string
  content?: string
}

function apiOrigin(apiBase: string): string {
  const match = String(apiBase || '').trim().match(/^(https?:\/\/[^/]+)/i)
  return match?.[1] || '当前小程序接口域名'
}

export function downloadFailureMessage(errMsg = '', apiBase = ''): DownloadFailureMessage {
  if (/downloadFile.*合法域名|不在以下 downloadFile 合法域名列表|url not in domain list|domain list/i.test(errMsg)) {
    return {
      title: '下载域名未配置',
      content: `请在微信公众平台把 ${apiOrigin(apiBase)} 加入 downloadFile 合法域名后重试。`,
    }
  }
  return { title: '文件下载失败' }
}

export function showDownloadFailure(err?: UniNamespace.GeneralCallbackResult) {
  let apiBase = ''
  try {
    apiBase = buildAPIURL('/').replace(/\/$/, '')
  } catch {
    // Environment errors are reported by the request guard; keep this modal concise.
  }
  const message = downloadFailureMessage(err?.errMsg || '', apiBase)
  if (message.content) {
    uni.showModal({
      title: message.title,
      content: message.content,
      showCancel: false,
    })
    return
  }
  uni.showToast({ title: message.title, icon: 'none' })
}

export function openMiniappFileOutput(options: {
  path: string
  token: string
  kind: MiniappFileOutputKind
  loadingTitle?: string
}): Promise<void> {
  return new Promise((resolve) => {
    let url = ''
    try {
      url = buildAPIURL(options.path)
    } catch (cause) {
      uni.showModal({
        title: '小程序环境错误',
        content: cause instanceof Error ? cause.message : '小程序环境配置错误',
        showCancel: false,
      })
      resolve()
      return
    }
    uni.showLoading({ title: options.loadingTitle || '生成中' })
    uni.downloadFile({
      url,
      header: { Authorization: `Bearer ${options.token}` },
      success: (res) => {
        if (res.statusCode !== 200 || !res.tempFilePath) {
          uni.showToast({ title: '文件暂不可用', icon: 'none' })
          return
        }
        if (options.kind === 'pdf') {
          uni.openDocument({
            filePath: res.tempFilePath,
            fileType: 'pdf',
            showMenu: true,
            fail: () => uni.showToast({ title: 'PDF 打开失败', icon: 'none' }),
          })
          return
        }
        uni.previewImage({
          urls: [res.tempFilePath],
          current: res.tempFilePath,
          fail: () => uni.showToast({ title: '图片预览失败', icon: 'none' }),
        })
      },
      fail: showDownloadFailure,
      complete: () => {
        uni.hideLoading()
        resolve()
      },
    })
  })
}
