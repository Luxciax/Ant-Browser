import type { BrowserCore, BrowserProfile, BrowserProxy, BrowserSettings } from '../types'

export async function getBindings() {
  const devMode = Boolean((import.meta as any).env?.DEV)
  const raw = (globalThis as any).go?.main?.App ?? null
  if (!raw) {
    if (devMode) return null
    throw new Error('Wails runtime binding unavailable')
  }
  try {
    const generated: any = await import('../../../wailsjs/go/main/App')
    return new Proxy(generated, {
      get(target, property, receiver) {
        const generatedValue = Reflect.get(target, property, receiver)
        if (generatedValue !== undefined) return generatedValue
        const rawValue = raw?.[String(property)]
        if (typeof rawValue === 'function') return (...args: any[]) => rawValue(...args)
        return rawValue
      },
    })
  } catch (error) {
    if (raw) {
      return new Proxy({}, {
        get(_target, property) {
          const rawValue = raw?.[String(property)]
          if (typeof rawValue === 'function') return (...args: any[]) => rawValue(...args)
          return rawValue
        },
      })
    }
    if (devMode) return null
    const reason = error instanceof Error ? error.message : String(error)
    throw new Error(`Failed to load Wails bindings: ${reason}`)
  }
}

export function getGoApp(): any {
  return (globalThis as any).go?.main?.App ?? null
}

export function nowISOString(): string {
  return new Date().toISOString()
}

export function createDefaultBrowserSettings(): BrowserSettings {
  return {
    userDataRoot: 'data',
    defaultFingerprintArgs: [],
    defaultLaunchArgs: [],
    defaultStartUrls: [],
    lightStartEnabled: true,
    restoreLastSession: true,
    startReadyTimeoutMs: 3000,
    startStableWindowMs: 1200,
    defaultConnectorType: 'xray',
  }
}

let mockProfiles: BrowserProfile[] = [
  {
    profileId: 'mock-1',
    profileName: '默认指纹配置',
    userDataDir: 'data/default',
    coreId: 'default',
    fingerprintArgs: ['--fingerprint-brand=Chrome', '--fingerprint-platform=windows'],
    proxyId: '',
    proxyConfig: '',
    memoryLimitMb: 0,
    launchArgs: ['--disable-features=Translate'],
    tags: ['默认'],
    keywords: [],
    runtimeState: 'stopped',
    running: false,
    debugPort: 0,
    debugReady: false,
    pid: 0,
    runtimeWarning: '',
    lastError: '',
    createdAt: nowISOString(),
    updatedAt: nowISOString(),
  },
]

let mockCores: BrowserCore[] = []
let mockProxies: BrowserProxy[] = []

export function getMockProfiles(): BrowserProfile[] {
  return mockProfiles
}

export function setMockProfiles(next: BrowserProfile[]): void {
  mockProfiles = next
}

export function getMockCores(): BrowserCore[] {
  return mockCores
}

export function setMockCores(next: BrowserCore[]): void {
  mockCores = next
}

export function getMockProxies(): BrowserProxy[] {
  return mockProxies
}

export function setMockProxies(next: BrowserProxy[]): void {
  mockProxies = next
}
