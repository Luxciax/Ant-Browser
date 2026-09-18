import type { BrowserProfile } from '../types'

export type BrowserProfileRuntimeState = 'stopped' | 'starting' | 'running' | 'stopping' | 'failed'

export function normalizeBrowserRuntimeState(profile: BrowserProfile): BrowserProfileRuntimeState {
  switch (profile.runtimeState) {
    case 'starting':
    case 'running':
    case 'stopping':
    case 'failed':
    case 'stopped':
      return profile.runtimeState
    default:
      return profile.running || profile.debugReady || profile.pid > 0 || profile.debugPort > 0 ? 'running' : 'stopped'
  }
}

export function isBrowserRuntimeRunning(profile: BrowserProfile): boolean {
  return normalizeBrowserRuntimeState(profile) === 'running'
}

export function isBrowserRuntimeTransitioning(profile: BrowserProfile): boolean {
  const state = normalizeBrowserRuntimeState(profile)
  return state === 'starting' || state === 'stopping'
}

export function hasBrowserRuntimeActivity(profile: BrowserProfile): boolean {
  return profile.running || profile.debugReady || profile.pid > 0 || profile.debugPort > 0
}

export function isBrowserRuntimeConfigLocked(profile: BrowserProfile): boolean {
  const state = normalizeBrowserRuntimeState(profile)
  if (state === 'starting' || state === 'running' || state === 'stopping') return true
  return state === 'failed' && hasBrowserRuntimeActivity(profile)
}

export function isBrowserRuntimeStoppable(profile: BrowserProfile): boolean {
  const state = normalizeBrowserRuntimeState(profile)
  return state === 'running' || state === 'stopping' || (state === 'failed' && hasBrowserRuntimeActivity(profile))
}

export function isBrowserRuntimeStopped(profile: BrowserProfile): boolean {
  return normalizeBrowserRuntimeState(profile) === 'stopped'
}

export function browserRuntimeLabel(profile: BrowserProfile): string {
  switch (normalizeBrowserRuntimeState(profile)) {
    case 'starting': return '启动中'
    case 'stopping': return '停止中'
    case 'failed': return '异常'
    case 'running': return profile.debugReady ? '运行中' : '运行中 · 待就绪'
    default: return '已停止'
  }
}
