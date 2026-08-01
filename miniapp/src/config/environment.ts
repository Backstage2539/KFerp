export type MiniappBuildEnvironment = 'development' | 'production'
export type MiniProgramEnvVersion = 'develop' | 'trial' | 'release' | 'unknown'

export type MiniappEnvironmentConfig = {
  environment: MiniappBuildEnvironment
  apiBase: string
  storageNamespace: string
  isDevelopment: boolean
}

export class MiniappEnvironmentError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'MiniappEnvironmentError'
  }
}

function normalizeAPIBase(base: string): string {
  return String(base || '').trim().replace(/\/+$/, '')
}

export function createMiniappEnvironmentConfig(
  buildEnvironment: string,
  apiBase: string,
): MiniappEnvironmentConfig {
  const environment = String(buildEnvironment || '').trim()
  if (!environment) {
    throw new MiniappEnvironmentError('小程序构建环境未配置，请使用受控构建命令')
  }
  if (environment !== 'development' && environment !== 'production') {
    throw new MiniappEnvironmentError(`不支持的小程序构建环境：${environment}`)
  }

  const normalizedAPIBase = normalizeAPIBase(apiBase)
  if (!normalizedAPIBase) {
    throw new MiniappEnvironmentError('小程序 API 地址未配置，请使用受控构建命令')
  }

  return {
    environment,
    apiBase: normalizedAPIBase,
    storageNamespace: `kferp.mini.${environment}`,
    isDevelopment: environment === 'development',
  }
}

export function configuredMiniappEnvironment(): MiniappEnvironmentConfig {
  return createMiniappEnvironmentConfig(
    import.meta.env.VITE_KFERP_ENVIRONMENT || '',
    import.meta.env.VITE_KFERP_API_BASE || '',
  )
}

export function currentMiniProgramEnvVersion(): MiniProgramEnvVersion {
  try {
    const accountAPI = uni as typeof uni & {
      getAccountInfoSync?: () => { miniProgram?: { envVersion?: string } }
    }
    const version = String(accountAPI.getAccountInfoSync?.().miniProgram?.envVersion || '').trim()
    if (version === 'develop' || version === 'trial' || version === 'release') return version
  } catch {
    // Non-WeChat unit-test and preview runtimes do not always expose account info.
  }
  return 'unknown'
}

export function miniappRuntimeSafetyMessage(
  config: MiniappEnvironmentConfig,
  envVersion: MiniProgramEnvVersion,
): string {
  if (config.isDevelopment && envVersion === 'release') {
    return '开发包不能作为正式版使用，请重新上传 production 小程序包'
  }
  return ''
}

export function assertMiniappRuntimeSafe(
  config = configuredMiniappEnvironment(),
  envVersion = currentMiniProgramEnvVersion(),
): MiniappEnvironmentConfig {
  const message = miniappRuntimeSafetyMessage(config, envVersion)
  if (message) throw new MiniappEnvironmentError(message)
  return config
}

export function environmentBadgeText(
  config = configuredMiniappEnvironment(),
  envVersion = currentMiniProgramEnvVersion(),
): string {
  if (!config.isDevelopment) return ''
  return envVersion === 'release' ? '开发包禁止正式使用' : '开发环境 · 测试数据'
}

export function miniappStorageKey(
  key: string,
  config = configuredMiniappEnvironment(),
): string {
  return `${config.storageNamespace}.${String(key || '').trim()}`
}
