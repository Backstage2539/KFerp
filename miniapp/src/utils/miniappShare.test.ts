import { beforeEach, describe, expect, it, vi } from 'vitest'
import { fetchMiniappSharePolicy } from '../api/customerPortal'
import { applyMiniappShareMenu, refreshMiniappShareMenu } from './miniappShare'

vi.mock('../api/customerPortal', () => ({ fetchMiniappSharePolicy: vi.fn() }))
vi.mock('../stores/session', () => ({ miniappTokenStorageKey: () => 'kferp.mini.production.token' }))

beforeEach(() => {
  vi.mocked(fetchMiniappSharePolicy).mockReset()
  vi.stubGlobal('uni', {
    getStorageSync: vi.fn(() => 'employee-token'),
  })
})

describe('miniapp share menu policy', () => {
  it('shows page and timeline forwarding when the current user may share', () => {
    const platform = {
      showShareMenu: vi.fn(),
      hideShareMenu: vi.fn(),
    }

    applyMiniappShareMenu(true, platform)

    expect(platform.showShareMenu).toHaveBeenCalledWith({
      menus: ['shareAppMessage', 'shareTimeline'],
    })
    expect(platform.hideShareMenu).not.toHaveBeenCalled()
  })

  it('hides forwarding when the current user is outside the configured scope', () => {
    const platform = {
      showShareMenu: vi.fn(),
      hideShareMenu: vi.fn(),
    }

    applyMiniappShareMenu(false, platform)

    expect(platform.hideShareMenu).toHaveBeenCalledWith({
      menus: ['shareAppMessage', 'shareTimeline'],
    })
    expect(platform.showShareMenu).not.toHaveBeenCalled()
  })

  it('does not crash on a client without share-menu APIs', () => {
    expect(() => applyMiniappShareMenu(false, {})).not.toThrow()
  })

  it('refreshes from the server with the persisted session token', async () => {
    const platform = {
      showShareMenu: vi.fn(),
      hideShareMenu: vi.fn(),
    }
    vi.mocked(fetchMiniappSharePolicy).mockResolvedValue({
      settings: { image_need_show_entrance: true, share_scope: 'employee' },
      can_share: true,
    })

    await refreshMiniappShareMenu(platform)

    expect(fetchMiniappSharePolicy).toHaveBeenCalledWith('employee-token')
    expect(platform.showShareMenu).toHaveBeenCalledOnce()
  })

  it('fails closed when the share policy cannot be read', async () => {
    const platform = {
      showShareMenu: vi.fn(),
      hideShareMenu: vi.fn(),
    }
    vi.mocked(fetchMiniappSharePolicy).mockRejectedValue(new Error('network error'))

    await refreshMiniappShareMenu(platform)

    expect(platform.hideShareMenu).toHaveBeenCalledOnce()
    expect(platform.showShareMenu).not.toHaveBeenCalled()
  })
})
