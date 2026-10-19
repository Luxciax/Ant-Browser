export interface CoreDisplayInfo {
  fingerprintBackend?: 'fingerprint-chromium' | 'clearcote' | 'generic-chromium' | ''
  coreId: string
  coreName: string
  corePath: string
  isDefault: boolean
  pathValid: boolean
  pathMessage: string
  chromeVersion: string
  instanceCount: number
}

export interface CoreSettingsForm {
  userDataRoot: string
  defaultFingerprintArgs: string
  defaultLaunchArgs: string
  defaultStartUrls: string
  lightStartEnabled: boolean
  restoreLastSession: boolean
  startReadyTimeoutMs: number
  startStableWindowMs: number
}

export interface CoreEditForm {
  fingerprintBackend?: 'fingerprint-chromium' | 'clearcote' | 'generic-chromium' | ''
  coreName: string
  corePath: string
}

export interface CoreDownloadForm {
  provider?: 'fingerprint-chromium' | 'clearcote'
  coreId?: string
  name: string
  url: string
  proxyMode: string
  proxyId: string
  mode?: 'download' | 'redownload'
}

export interface CoreDownloadProgress {
  phase: string
  progress: number
  message: string
}
