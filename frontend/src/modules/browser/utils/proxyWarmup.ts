import { browserProxyWarmupBridgeWithConfig } from '../api'
import type { BrowserProfile } from '../types'
import { isBrowserRuntimeConfigLocked } from './runtimeState'

export async function warmupProfileProxyBeforeStart(profile: BrowserProfile | null | undefined): Promise<void> {
  if (!profile || isBrowserRuntimeConfigLocked(profile) || (!profile.proxyId && !profile.proxyConfig)) {
    return
  }
  try {
    await browserProxyWarmupBridgeWithConfig(profile.proxyId || '', profile.proxyConfig || '')
  } catch {
  }
}
