import { getGoApp } from './runtime'

export interface IdentityTemplate { id: string; name: string; platform: string; platformVersion: string; cpu: number; windowSize: string }
export interface IdentityGeo { latitude: number; longitude: number; accuracy: number }
export interface IdentityReport {
  identity: { seed: string; platform: string; platformVersion: string; brand: string; brandVersion: string; userAgent: string; hardwareConcurrency: number; windowSize: string; locale: string; languages: string[]; timezone: string; templateId?: string; fontPolicy: string; geo?: IdentityGeo }
  hash: string
  kernelVersion: string
  issues: Array<{ field: string; level: 'warning' | 'error'; message: string }>
  duplicateProfiles: string[]
}
export interface IdentityInput { profileId: string; coreId: string; args: string[]; templateId?: string; alignKernel?: boolean; geo?: IdentityGeo; clearGeo?: boolean }

export async function fetchIdentityTemplates(): Promise<IdentityTemplate[]> {
  const app = getGoApp()
  if (!app?.BrowserIdentityTemplates) throw new Error('身份画像需连接更新后的桌面后端')
  return (await app.BrowserIdentityTemplates()) || []
}
export async function generateIdentity(input: IdentityInput): Promise<{ args: string[]; report: IdentityReport }> {
  const app = getGoApp()
  if (!app?.BrowserIdentityGenerate) throw new Error('身份画像需连接更新后的桌面后端')
  return await app.BrowserIdentityGenerate(input)
}
export function identityArg(args: string[], key: string): string {
  const arg = [...args].reverse().find(item => item.split('=')[0] === key)
  return arg?.slice(key.length + 1) || ''
}
export function setIdentityArg(args: string[], key: string, value: string): string[] {
  const kept = args.filter(item => item.split('=')[0] !== key)
  return value ? [...kept, `${key}=${value}`] : kept
}
