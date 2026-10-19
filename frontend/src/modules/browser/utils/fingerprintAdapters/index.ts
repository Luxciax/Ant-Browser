import * as legacy from './fingerprintChromium'
import { FINGERPRINT_CAPABILITIES } from '../fingerprintCapabilities'
import { clearcoteAdapter } from './clearcote'
import { CLEARCOTE_CONFIG_ARG, type FingerprintAdapter, type FingerprintBackend } from './types'

export * from './types'
export const fingerprintChromiumAdapter: FingerprintAdapter = {
  id: 'fingerprint-chromium', serialize: legacy.serialize, deserialize: legacy.deserialize,
  capabilities: FINGERPRINT_CAPABILITIES,
  validate: config => legacy.validateFingerprintArgs(legacy.serialize(config).filter(arg => !arg.startsWith('--ant-'))),
}
const genericFlags = new Set(['--lang', '--accept-lang', '--window-size', '--disable-non-proxied-udp', '--webrtc-ip-handling-policy'])
export const genericChromiumAdapter: FingerprintAdapter = {
  id: 'generic-chromium', deserialize: legacy.deserialize,
  serialize: config => legacy.serialize(config).filter(arg => genericFlags.has(arg.split('=')[0])),
  capabilities: [], validate: () => ({ valid: true, issues: [] }),
}
export function getFingerprintAdapter(backend: FingerprintBackend = 'fingerprint-chromium'): FingerprintAdapter {
  if (backend === 'clearcote') return clearcoteAdapter
  return backend === 'generic-chromium' ? genericChromiumAdapter : fingerprintChromiumAdapter
}
export function resolveFingerprintBackend(cores: { coreId: string; isDefault: boolean; fingerprintBackend?: string }[], coreId: string): FingerprintBackend {
  const core = cores.find(item => item.coreId === coreId) || cores.find(item => item.isDefault) || cores[0]
  return core?.fingerprintBackend === 'clearcote' || core?.fingerprintBackend === 'generic-chromium' ? core.fingerprintBackend : 'fingerprint-chromium'
}
export function storeFingerprintConfig(adapter: FingerprintAdapter, config: legacy.FingerprintConfig): string[] {
  const args = adapter.serialize(config)
  if (adapter.id !== 'clearcote') return args
  const { unknownArgs: _unknownArgs, ...stored } = config
  return [...args, CLEARCOTE_CONFIG_ARG + JSON.stringify(stored)]
}
