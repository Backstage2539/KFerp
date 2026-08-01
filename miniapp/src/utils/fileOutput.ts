import { buildAPIURL, MiniRequestError } from '../api/client'

export type MiniappFileOutputKind = 'pdf' | 'png'

type MiniappCallbackResult = { errMsg?: string }

type MiniappCallbackOptions = {
  success?: (result: MiniappCallbackResult) => void
  fail?: (result: MiniappCallbackResult) => void
}

export type MiniappSharePlatform = {
  shareFileMessage?: (options: MiniappCallbackOptions & { filePath: string; fileName?: string }) => void
  showShareImageMenu?: (options: MiniappCallbackOptions & { path: string }) => void
  openDocument?: (options: MiniappCallbackOptions & { filePath: string; fileType?: string; showMenu?: boolean }) => void
  previewImage?: (options: MiniappCallbackOptions & { urls: string[]; current?: string }) => void
}

declare const wx: MiniappSharePlatform | undefined

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

export function miniappDownloadStatusError(statusCode: number): MiniRequestError {
  if (statusCode === 401) return new MiniRequestError('登录已失效，请重新登录', statusCode)
  if (statusCode === 403) return new MiniRequestError('当前员工无此权限', statusCode)
  return new MiniRequestError('文件暂不可用', statusCode || 500)
}

function invokeMiniappCallback(
  run: (success: (result: MiniappCallbackResult) => void, fail: (result: MiniappCallbackResult) => void) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    let settled = false
    const succeed = () => {
      if (settled) return
      settled = true
      resolve()
    }
    const fail = (result: MiniappCallbackResult = {}) => {
      if (settled) return
      settled = true
      reject(new Error(result.errMsg || '分享失败'))
    }
    try {
      run(succeed, fail)
    } catch (cause) {
      fail({ errMsg: cause instanceof Error ? cause.message : '分享失败' })
    }
  })
}

async function openDownloadedPDF(filePath: string, platform: MiniappSharePlatform): Promise<void> {
  if (!platform.openDocument) throw new Error('当前微信版本无法打开 PDF')
  await invokeMiniappCallback((success, fail) => platform.openDocument?.({
    filePath,
    fileType: 'pdf',
    showMenu: true,
    success,
    fail,
  }))
}

async function previewDownloadedImage(filePath: string, platform: MiniappSharePlatform): Promise<void> {
  if (!platform.previewImage) throw new Error('当前微信版本无法预览图片')
  await invokeMiniappCallback((success, fail) => platform.previewImage?.({
    urls: [filePath],
    current: filePath,
    success,
    fail,
  }))
}

export async function shareDownloadedMiniappFile(
  options: { filePath: string; fileName?: string; kind: MiniappFileOutputKind },
  platform: MiniappSharePlatform,
): Promise<void> {
  if (options.kind === 'pdf') {
    if (!platform.shareFileMessage) {
      await openDownloadedPDF(options.filePath, platform)
      return
    }
    try {
      await invokeMiniappCallback((success, fail) => platform.shareFileMessage?.({
        filePath: options.filePath,
        fileName: options.fileName,
        success,
        fail,
      }))
    } catch {
      await openDownloadedPDF(options.filePath, platform)
    }
    return
  }

  if (!platform.showShareImageMenu) {
    await previewDownloadedImage(options.filePath, platform)
    return
  }
  try {
    await invokeMiniappCallback((success, fail) => platform.showShareImageMenu?.({
      path: options.filePath,
      success,
      fail,
    }))
  } catch {
    await previewDownloadedImage(options.filePath, platform)
  }
}

function currentSharePlatform(): MiniappSharePlatform {
  let wechat: MiniappSharePlatform | undefined
  try {
    wechat = typeof wx === 'undefined' ? undefined : wx
  } catch {
    wechat = undefined
  }
  return {
    shareFileMessage: wechat?.shareFileMessage?.bind(wechat),
    showShareImageMenu: wechat?.showShareImageMenu?.bind(wechat),
    openDocument: uni.openDocument.bind(uni) as MiniappSharePlatform['openDocument'],
    previewImage: uni.previewImage.bind(uni) as MiniappSharePlatform['previewImage'],
  }
}

export function shareMiniappFileOutput(options: {
  path: string
  token: string
  kind: MiniappFileOutputKind
  fileName?: string
  loadingTitle?: string
}): Promise<void> {
  return new Promise((resolve, reject) => {
    let finished = false
    let loadingVisible = false
    const stopLoading = () => {
      if (!loadingVisible) return
      loadingVisible = false
      uni.hideLoading()
    }
    const finish = () => {
      if (finished) return
      finished = true
      stopLoading()
      resolve()
    }
    const fail = (cause: Error) => {
      if (finished) return
      finished = true
      stopLoading()
      reject(cause)
    }
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
    uni.showLoading({ title: options.loadingTitle || '准备分享' })
    loadingVisible = true
    uni.downloadFile({
      url,
      header: { Authorization: `Bearer ${options.token}` },
      success: async (res) => {
        if (res.statusCode !== 200 || !res.tempFilePath) {
          fail(miniappDownloadStatusError(res.statusCode))
          return
        }
        stopLoading()
        try {
          await shareDownloadedMiniappFile({
            filePath: res.tempFilePath,
            fileName: options.fileName,
            kind: options.kind,
          }, currentSharePlatform())
        } catch {
          uni.showToast({
            title: options.kind === 'pdf' ? '分享失败，请从 PDF 菜单转发' : '分享失败，请从图片预览转发',
            icon: 'none',
          })
        } finally {
          finish()
        }
      },
      fail: (err) => {
        showDownloadFailure(err)
        finish()
      },
    })
  })
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
