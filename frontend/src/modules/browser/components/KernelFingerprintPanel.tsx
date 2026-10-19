import { FormItem, Input } from '../../../shared/components'
import type { FingerprintConfig } from '../utils/fingerprintSerializer'
import { getFingerprintAdapter, storeFingerprintConfig, type FingerprintBackend } from '../utils/fingerprintAdapters'

export interface KernelFingerprintPanelProps {
  value: string[]
  onChange: (args: string[]) => void
  backend: FingerprintBackend
}

export function KernelFingerprintPanel({ value, onChange, backend }: KernelFingerprintPanelProps) {
  const adapter = getFingerprintAdapter(backend)
  const config = adapter.deserialize(value)
  const update = (patch: Partial<FingerprintConfig>) => onChange(storeFingerprintConfig(adapter, { ...config, ...patch }))
  const fields: [keyof FingerprintConfig, string][] = backend === 'clearcote'
    ? [['seed', 'Seed（留空按实例稳定生成）'], ['platform', '平台（windows / macos / linux / android）'], ['platformVersion', '系统版本'], ['brand', '品牌（默认 Chrome）'], ['brandVersion', '品牌版本（留空跟随内核）'], ['lang', '界面语言'], ['acceptLanguage', '语言列表'], ['timezone', '时区']]
    : [['lang', '语言'], ['acceptLang', '语言列表'], ['resolution', '窗口大小（宽,高）']]
  const validation = adapter.validate(config)
  return <div className="space-y-4">
    <p className="text-sm">指纹内核：<strong>{backend === 'clearcote' ? 'Clearcote' : '普通 Chromium'}</strong></p>
    {backend === 'generic-chromium' && <p className="text-sm text-[var(--color-text-muted)]">当前内核不支持原生指纹伪装。可在内核管理中修改指纹后端。</p>}
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">{fields.map(([field, label]) => <FormItem key={field} label={label}><Input aria-label={label} value={String(config[field] ?? '')} onChange={event => update({ [field]: event.target.value })} /></FormItem>)}</div>
    {validation.issues.map((issue, index) => <p role={issue.level === 'error' ? 'alert' : undefined} key={index} className="text-sm text-[var(--color-error)]">{issue.message}</p>)}
    <details><summary>当前内核能力</summary>{adapter.capabilities.map(item => <p key={item.id} className="text-sm mt-2"><strong>{item.name}</strong>：{item.coverage}</p>)}</details>
  </div>
}
