import { buildAcceptLanguage, getSystemTimezone, normalizeFingerprintPlatform, type FingerprintConfig } from './fingerprintChromium'
import { CLEARCOTE_CONFIG_ARG, type FingerprintAdapter } from './types'

export const CLEARCOTE_FIELDS = {
  seed: 'fingerprint', platform: 'fingerprint-platform', platformVersion: 'fingerprint-platform-version',
  brand: 'fingerprint-brand', brandVersion: 'fingerprint-brand-version',
  hardwareConcurrency: 'fingerprint-hardware-concurrency', deviceMemory: 'fingerprint-device-memory',
  screenWidth: 'fingerprint-screen-width', screenHeight: 'fingerprint-screen-height',
  availWidth: 'fingerprint-avail-width', availHeight: 'fingerprint-avail-height', colorDepth: 'fingerprint-color-depth',
  devicePixelRatio: 'fingerprint-device-pixel-ratio', touchPoints: 'fingerprint-max-touch-points',
  gpuVendor: 'fingerprint-gpu-vendor', gpuRenderer: 'fingerprint-gpu-renderer',
  timezone: 'timezone', acceptLanguage: 'accept-lang', lang: 'lang', location: 'fingerprint-location', webrtcIp: 'webrtc-ip',
} as const

const manualFields = new Set(['hardwareConcurrency', 'deviceMemory', 'screenWidth', 'screenHeight', 'availWidth', 'availHeight', 'colorDepth', 'devicePixelRatio', 'touchPoints', 'gpuVendor', 'gpuRenderer'])

export function serializeClearcote(config: FingerprintConfig): string[] {
  const mode = config.mode || 'auto'
  const values = { ...config, platform: config.platform ? normalizeFingerprintPlatform(config.platform) : undefined,
    brand: config.brand || 'Chrome', gpuVendor: config.gpuVendor ?? config.webglVendor, gpuRenderer: config.gpuRenderer ?? config.webglRenderer,
    acceptLanguage: config.acceptLanguage || config.acceptLang || (config.lang ? buildAcceptLanguage(config.lang) : undefined),
    timezone: config.timezone === 'system' ? getSystemTimezone() : config.timezone }
  const args: string[] = []
  for (const [field, flag] of Object.entries(CLEARCOTE_FIELDS)) {
    if (manualFields.has(field) && mode !== 'manual') continue
    const value = values[field as keyof typeof values]
    if (value !== undefined && value !== null && value !== '') args.push(`--${flag}=${value}`)
  }
  const resolution = config.resolution === 'custom' ? config.customResolution : config.resolution
  if (resolution) args.push(`--window-size=${resolution}`)
  if (config.webrtcPolicy === 'disable_non_proxied_udp') args.push('--disable-non-proxied-udp')
  else if (config.webrtcPolicy) args.push(`--webrtc-ip-handling-policy=${config.webrtcPolicy}`)
  // Known fingerprint switches are exclusively owned by this adapter.
  return [...args, ...(config.unknownArgs || []).filter(arg => !/^--(?:ant-|fingerprint|disable-spoofing)/.test(arg))]
}

export function deserializeClearcote(args: string[]): FingerprintConfig {
  let config: FingerprintConfig = {}
  const stored = [...args].reverse().find(arg => arg.startsWith(CLEARCOTE_CONFIG_ARG))
  if (stored) {
    try { config = JSON.parse(stored.slice(CLEARCOTE_CONFIG_ARG.length)) } catch { /* Raw arguments remain editable. */ }
  }
  config = { ...config, unknownArgs: [] }
  for (const arg of args) {
    if (arg.startsWith(CLEARCOTE_CONFIG_ARG)) continue
    const eq = arg.indexOf('=')
    const flag = arg.slice(2, eq < 0 ? undefined : eq)
    const field = Object.entries(CLEARCOTE_FIELDS).find(([, value]) => value === flag)?.[0]
    if (field && eq >= 0) (config as Record<string, unknown>)[field] = arg.slice(eq + 1)
    else if (flag === 'window-size' && eq >= 0) config.resolution = arg.slice(eq + 1)
    else if (arg === '--disable-non-proxied-udp') config.webrtcPolicy = 'disable_non_proxied_udp'
    else if (flag === 'webrtc-ip-handling-policy' && eq >= 0) config.webrtcPolicy = arg.slice(eq + 1)
    else config.unknownArgs!.push(arg)
  }
  config.mode ||= 'auto'
  return config
}

export const clearcoteAdapter: FingerprintAdapter = {
  id: 'clearcote', serialize: serializeClearcote, deserialize: deserializeClearcote,
  capabilities: [
    { id: 'persona', name: '自动 Persona', mode: 'seed', coverage: '稳定种子生成关联设备画像；自动模式不覆盖 CPU、RAM、GPU、屏幕。', args: ['--fingerprint'] },
    { id: 'native', name: '原生设备覆盖', mode: 'configurable', coverage: '高级手动模式支持 CPU、RAM、屏幕、可用屏幕、DPR、色深、触控与 GPU。', args: Object.values(CLEARCOTE_FIELDS).map(flag => `--${flag}`) },
    { id: 'profile', name: '真实设备 Profile', mode: 'configurable', coverage: '在启动时编码本地采集 JSON；手动设置优先。', args: ['--fingerprint-profile'] },
  ],
  validate(config) {
    const issues: ReturnType<FingerprintAdapter['validate']>['issues'] = []
    if (config.mode === 'manual') {
      for (const field of manualFields) {
        if (field === 'gpuVendor' || field === 'gpuRenderer') continue
        const value = config[field as keyof FingerprintConfig]
        if (value !== undefined && value !== '' && (!Number.isFinite(Number(value)) || Number(value) < (field === 'touchPoints' ? 0 : 0.01) || (field !== 'devicePixelRatio' && field !== 'deviceMemory' && !Number.isInteger(Number(value))))) issues.push({ level: 'error', message: `${field} 必须是有效的${field === 'touchPoints' ? '非负' : '正'}数值` })
      }
    }
    if (config.mode === 'profile' && !config.fingerprintProfile?.filePath) issues.push({ level: 'error', message: '请选择真实设备 Profile JSON' })
    if (config.mode && !['auto', 'profile', 'manual'].includes(config.mode)) issues.push({ level: 'error', message: '未知指纹生成方式' })
    return { valid: !issues.some(issue => issue.level === 'error'), issues }
  },
}
