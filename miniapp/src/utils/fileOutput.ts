import { buildAPIURL } from '../api/client'

export type MiniappFileOutputKind = 'pdf' | 'png'

export type DownloadFailureMessage = {
  title: string
  content?: string
}

export function downloadFailureMessage(errMsg = ''): DownloadFailureMessage {
  if (/downloadFile.*合法域名|不在以下 downloadFile 合法域名列表|url not in domain list|domain list/i.test(errMsg)) {
    return {
      title: '下载域名未配置',
      content: '请在微信后台把 https://erp.qacoohee.com 加入 downloadFile 合法域名；开发者工具可在详情中关闭合法域名校验后重试。',
    }
  }
  return { title: '文件下载失败' }
}

export function showDownloadFailure(err?: UniNamespace.GeneralCallbackResult) {
  const message = downloadFailureMessage(err?.errMsg || '')
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
    uni.showLoading({ title: options.loadingTitle || '生成中' })
    uni.downloadFile({
      url: buildAPIURL(options.path),
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
