import type { FingerprintConfig, FingerprintValidationResult } from './fingerprintChromium'
import type { FingerprintCapabilityItem } from '../fingerprintCapabilities'

export type FingerprintBackend = 'fingerprint-chromium' | 'clearcote' | 'generic-chromium'
export type FingerprintMode = 'auto' | 'profile' | 'manual'
export interface FingerprintProfileConfig { source: 'file'; filePath: string }

export interface FingerprintAdapter {
  id: FingerprintBackend
  serialize(config: FingerprintConfig): string[]
  deserialize(args: string[]): FingerprintConfig
  capabilities: FingerprintCapabilityItem[]
  validate(config: FingerprintConfig): FingerprintValidationResult
}

// Application metadata lives beside the existing argument array for backward-compatible
// persistence/backup. It is consumed by AntBrowser and never passed to Chromium.
export const CLEARCOTE_CONFIG_ARG = '--ant-clearcote-config='
