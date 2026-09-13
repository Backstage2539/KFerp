import { fetchMiniappSharePolicy } from '../api/customerPortal'
import { miniappTokenStorageKey } from '../stores/session'

export type MiniappSharePayload = {
  title: string
  path: string
}

export type MiniappTimelineSharePayload = {
  title: string
  query: string
}

export type MiniappShareMenuPlatform = {
  showShareMenu?(options: { menus: string[] }): unknown
  hideShareMenu?(options: { menus: string[] }): unknown
}

const miniappShareMenus = ['shareAppMessage', 'shareTimeline']

function runtimeSharePlatform(): MiniappShareMenuPlatform {
  return uni as unknown as MiniappShareMenuPlatform
}

export function applyMiniappShareMenu(canShare: boolean, platform = runtimeSharePlatform()): void {
  if (canShare) {
    platform.showShareMenu?.({ menus: [...miniappShareMenus] })
    return
  }
  platform.hideShareMenu?.({ menus: [...miniappShareMenus] })
}

export async function refreshMiniappShareMenu(platform = runtimeSharePlatform()): Promise<void> {
  try {
    const token = String(uni.getStorageSync(miniappTokenStorageKey()) || '')
    const policy = await fetchMiniappSharePolicy(token)
    applyMiniappShareMenu(policy.can_share === true, platform)
  } catch {
    applyMiniappShareMenu(false, platform)
  }
}

export function defaultMiniappShare(): MiniappSharePayload {
  return {
    title: 'KFerp 客户中心',
    path: '/pages/index/index',
  }
}

export function defaultMiniappTimelineShare(): MiniappTimelineSharePayload {
  return {
    title: 'KFerp 客户中心',
    query: '',
  }
}
