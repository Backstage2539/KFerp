import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import {
  createMiniappEnvironmentConfig,
  environmentBadgeText,
  miniappRuntimeSafetyMessage,
  miniappStorageKey,
} from './environment'

describe('miniapp environment boundary', () => {
  it('requires an explicit build environment and API base', () => {
    expect(() => createMiniappEnvironmentConfig('', '')).toThrow('小程序构建环境未配置')
    expect(() => createMiniappEnvironmentConfig('development', '')).toThrow('小程序 API 地址未配置')
    expect(() => createMiniappEnvironmentConfig('preview', 'https://dev.qacoohee.com/app')).toThrow('不支持的小程序构建环境')
  })

  it('builds isolated development and production metadata', () => {
    const development = createMiniappEnvironmentConfig('development', ' https://dev.qacoohee.com/app/ ')
    const production = createMiniappEnvironmentConfig('production', 'https://erp.qacoohee.com/app')

    expect(development).toEqual({
      environment: 'development',
      apiBase: 'https://dev.qacoohee.com/app',
      storageNamespace: 'kferp.mini.development',
      isDevelopment: true,
    })
    expect(production).toEqual({
      environment: 'production',
      apiBase: 'https://erp.qacoohee.com/app',
      storageNamespace: 'kferp.mini.production',
      isDevelopment: false,
    })
    expect(miniappStorageKey('token', development)).toBe('kferp.mini.development.token')
    expect(miniappStorageKey('token', production)).toBe('kferp.mini.production.token')
  })

  it('blocks a development build accidentally published as the WeChat release version', () => {
    const development = createMiniappEnvironmentConfig('development', 'https://dev.qacoohee.com/app')
    const production = createMiniappEnvironmentConfig('production', 'https://erp.qacoohee.com/app')

    expect(miniappRuntimeSafetyMessage(development, 'develop')).toBe('')
    expect(miniappRuntimeSafetyMessage(development, 'trial')).toBe('')
    expect(miniappRuntimeSafetyMessage(development, 'release')).toContain('开发包不能作为正式版使用')
    expect(miniappRuntimeSafetyMessage(production, 'release')).toBe('')
  })

  it('shows a persistent badge only for the development build', () => {
    const development = createMiniappEnvironmentConfig('development', 'https://dev.qacoohee.com/app')
    const production = createMiniappEnvironmentConfig('production', 'https://erp.qacoohee.com/app')

    expect(environmentBadgeText(development, 'develop')).toBe('开发环境 · 测试数据')
    expect(environmentBadgeText(development, 'release')).toBe('开发包禁止正式使用')
    expect(environmentBadgeText(production, 'release')).toBe('')
  })

  it('renders the environment badge on every registered page', () => {
    const pages = JSON.parse(readFileSync(resolve('src/pages.json'), 'utf8')) as { pages: { path: string }[] }

    for (const page of pages.pages) {
      const source = readFileSync(resolve('src', `${page.path}.vue`), 'utf8')
      expect(source, page.path).toContain('EnvironmentBadge')
    }
  })
})
