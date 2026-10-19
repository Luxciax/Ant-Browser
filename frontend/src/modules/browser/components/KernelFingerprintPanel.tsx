import { useState } from 'react'
import { importFingerprintProfile } from '../api/cores'
import { Button, FormItem, Input, Select } from '../../../shared/components'
import type { FingerprintConfig } from '../utils/fingerprintSerializer'
import { getFingerprintAdapter, storeFingerprintConfig, type FingerprintBackend } from '../utils/fingerprintAdapters'

export interface KernelFingerprintPanelProps {
  value: string[]
  onChange: (args: string[]) => void
  backend: FingerprintBackend
}

export function KernelFingerprintPanel({ value, onChange, backend }: KernelFingerprintPanelProps) {
  const [importing, setImporting] = useState(false)
  const [importError, setImportError] = useState('')
  const adapter = getFingerprintAdapter(backend)
  const config = adapter.deserialize(value)
  const update = (patch: Partial<FingerprintConfig>) => onChange(storeFingerprintConfig(adapter, { ...config, ...patch }))
  const fields: [keyof FingerprintConfig, string][] = backend === 'clearcote'
    ? [['seed', 'Seed（留空按实例稳定生成）'], ['platform', '平台（windows / macos / linux / android）'], ['platformVersion', '系统版本'], ['brand', '品牌（默认 Chrome）'], ['brandVersion', '品牌版本（留空跟随内核）'], ['lang', '界面语言'], ['acceptLanguage', '语言列表'], ['timezone', '时区']]
    : [['lang', '语言'], ['acceptLang', '语言列表'], ['resolution', '窗口大小（宽,高）']]
  if (backend === 'clearcote') {
    fields.push(['location', '位置（纬度,经度）'], ['webrtcIp', 'WebRTC IP'], ['resolution', '窗口大小（宽,高）'])
    if (config.mode === 'manual') fields.push(
      ['hardwareConcurrency', 'CPU 核心数'], ['deviceMemory', 'RAM（GB，Chromium 最高报告 8）'],
      ['screenWidth', '屏幕宽度'], ['screenHeight', '屏幕高度'], ['availWidth', '可用屏幕宽度'], ['availHeight', '可用屏幕高度'],
      ['colorDepth', '色深'], ['devicePixelRatio', 'DPR'], ['touchPoints', '触控点（0 表示无触控）'], ['gpuVendor', 'GPU Vendor'], ['gpuRenderer', 'GPU Renderer'],
    )
  }
  const validation = adapter.validate(config)
  return <div className="space-y-4">
    <p className="text-sm">指纹内核：<strong>{backend === 'clearcote' ? 'Clearcote' : '普通 Chromium'}</strong></p>
    {backend === 'clearcote' && <>
      <FormItem label="指纹生成方式"><Select aria-label="指纹生成方式" value={config.mode || 'auto'} options={[{ value: 'auto', label: '自动 Persona' }, { value: 'profile', label: '导入真实设备 Profile' }, { value: 'manual', label: '高级手动' }]} onChange={event => update({ mode: event.target.value as FingerprintConfig['mode'] })} /></FormItem>
      <p className="text-sm text-[var(--color-text-muted)]">自动 Persona 由 Clearcote 生成一致设备画像。Seed 留空时按实例 ID 稳定生成，重启保持不变。</p>
    </>}
    {backend === 'generic-chromium' && <p className="text-sm text-[var(--color-text-muted)]">当前内核不支持原生指纹伪装。可在内核管理中修改指纹后端。</p>}
    {backend === 'clearcote' && config.mode !== 'auto' && <FormItem label="真实设备 Profile" hint="导入采集 JSON，启动时进行 gzip + base64 编码。高级手动设置优先于 Profile。">
      <div className="flex items-center gap-3"><Button disabled={importing} onClick={async () => {
        setImportError(''); setImporting(true)
        try { const result = await importFingerprintProfile(); if (result) update({ fingerprintProfile: { source: 'file', filePath: result.filePath } }) }
        catch (error) { setImportError(error instanceof Error ? error.message : '导入失败') }
        finally { setImporting(false) }
      }}>{importing ? '导入中…' : '选择 JSON'}</Button><span className="text-sm break-all">{config.fingerprintProfile?.filePath || '未选择'}</span>
      {config.fingerprintProfile && <Button variant="secondary" onClick={() => update({ fingerprintProfile: undefined })}>取消使用</Button>}</div>
      {importError && <p role="alert" className="text-sm text-[var(--color-error)]">{importError}</p>}
    </FormItem>}
    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">{fields.map(([field, label]) => <FormItem key={field} label={label}><Input aria-label={label} value={String(config[field] ?? '')} onChange={event => update({ [field]: event.target.value })} /></FormItem>)}</div>
    {validation.issues.map((issue, index) => <p role={issue.level === 'error' ? 'alert' : undefined} key={index} className="text-sm text-[var(--color-error)]">{issue.message}</p>)}
    <details><summary>当前内核能力</summary>{adapter.capabilities.map(item => <p key={item.id} className="text-sm mt-2"><strong>{item.name}</strong>：{item.coverage}</p>)}</details>
  </div>
}
