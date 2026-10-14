import { useEffect, useRef, useState } from 'react'
import { fetchIdentityTemplates, generateIdentity, identityArg, setIdentityArg, type IdentityInput, type IdentityReport, type IdentityTemplate } from '../../modules/browser/api/identity'
import './fish-identity.css'
import { getGoApp } from '../../modules/browser/api/runtime'

interface Props { args: string[]; profileId: string; coreId: string; report?: IdentityReport; reportLoading?: boolean; reportError?: string; onRetry?: () => void; onChange: (args: string[]) => void }

export function FishIdentityPanel({ args, profileId, coreId, report, reportLoading = false, reportError = '', onRetry, onChange }: Props) {
  const [templates, setTemplates] = useState<IdentityTemplate[]>([])
  const [template, setTemplate] = useState('')
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [latitude, setLatitude] = useState('')
  const [longitude, setLongitude] = useState('')
  const [accuracy, setAccuracy] = useState('1000')
  const key = JSON.stringify([profileId, coreId, args])
  const latestKey = useRef(key)
  latestKey.current = key
  const canGenerate = typeof getGoApp()?.BrowserIdentityGenerate === 'function'
  useEffect(() => {
    let active = true
    fetchIdentityTemplates().then(items => { if (active) setTemplates(items) }).catch(reason => { if (active) setError(String(reason instanceof Error ? reason.message : reason)) }).finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [])
  useEffect(() => {
    const geo = identityArg(args, '--ant-geolocation').split(',')
    setLatitude(geo.length === 3 ? geo[0] : '')
    setLongitude(geo.length === 3 ? geo[1] : '')
    setAccuracy(geo.length === 3 ? geo[2] : '1000')
  }, [identityArg(args, '--ant-geolocation')])

  const apply = async (change: Partial<IdentityInput>) => {
    const before = latestKey.current
    setBusy(true); setError('')
    try {
      const result = await generateIdentity({ profileId, coreId, args, ...change })
      if (latestKey.current !== before) { setError('配置已变化，请重新应用；未覆盖新修改。'); return }
      onChange(result.args)
    } catch (reason) { setError(String(reason instanceof Error ? reason.message : reason)) } finally { setBusy(false) }
  }
  const applyGeo = () => {
    const lat = Number(latitude), lon = Number(longitude), acc = Number(accuracy)
    if (!latitude.trim() || !longitude.trim() || !accuracy.trim() || !Number.isFinite(lat) || !Number.isFinite(lon) || !Number.isFinite(acc) || Math.abs(lat) > 90 || Math.abs(lon) > 180 || acc <= 0) { setError('请填写有效纬度（−90–90）、经度（−180–180）和大于 0 的精度。'); return }
    void apply({ geo: { latitude: lat, longitude: lon, accuracy: acc } })
  }

  return <section className="fish-subpanel fish-identity-panel" aria-labelledby="fish-identity-title">
    <div className="fish-subpanel-head"><strong id="fish-identity-title">身份画像一致性</strong><span>{busy ? '正在生成…' : reportLoading ? '正在读取内核能力…' : report?.kernelVersion ? `内核 ${report.kernelVersion}` : '内核版本未识别'}</span></div>
    {reportError ? <div className="fish-validation error" role="alert">{reportError}{onRetry ? <button className="fish-link-btn" type="button" onClick={onRetry}>重新读取</button> : null}</div> : !reportLoading && !report?.kernelVersion ? <p>无法识别内核版本，请选择可识别的内核后再应用画像或对齐版本。</p> : null}
    <p>应用模板会统一平台、CPU、窗口和 UA，保留已有种子、语言与时区。修改在保存后生效，不会随机重采样旧环境。</p>
    <div className="fish-form-grid two">
      <div className="fish-field"><label htmlFor="fish-identity-template">离线设备模板</label><select id="fish-identity-template" className="fish-select" value={template} disabled={loading || busy || !templates.length} onChange={event => setTemplate(event.target.value)}><option value="">{loading ? '正在读取模板…' : '选择模板'}</option>{templates.map(item => <option key={item.id} value={item.id}>{item.name}</option>)}</select></div>
      <div className="fish-inline-actions"><button className="fish-btn" type="button" disabled={!template || busy || reportLoading || !report?.kernelVersion} onClick={() => void apply({ templateId: template })}>应用画像</button><button className="fish-btn" type="button" disabled={busy || reportLoading || !report?.kernelVersion} onClick={() => void apply({ alignKernel: true })}>UA / 版本对齐</button></div>
      <div className="fish-field"><label htmlFor="fish-font-policy">跨平台字体策略</label><select id="fish-font-policy" className="fish-select" value={identityArg(args, '--ant-font-policy')} disabled={busy} onChange={event => onChange(setIdentityArg(args, '--ant-font-policy', event.target.value))}><option value="">保留旧版兼容行为</option><option value="spoof">保留内核字体伪装（推荐）</option><option value="host">使用宿主字体（缺字兼容）</option></select><small>宿主字体可能暴露真实系统；保留伪装时需实际检查中文、日文和韩文渲染。</small></div>
      <div className="fish-field"><strong>当前画像</strong><span>{report?.identity.platform || '未指定平台'} · {report?.identity.hardwareConcurrency || '默认'} 核 · 种子 {report?.identity.seed || '保存后生成'}</span><span>窗口：{report?.identity.windowSize || '内核默认'} · 品牌版本：{report?.identity.brandVersion || '内核默认'}</span><span>语言：{report?.identity.locale || '内核默认'} · 时区：{report?.identity.timezone || '内核默认'}</span><small className="fish-identity-ua">UA：{report?.identity.userAgent || '内核默认；可点版本对齐生成'}</small><small>屏幕、GPU 和设备内存仍取决于内核，模板不宣称控制这些参数。</small></div>
    </div>
    {report?.issues.length ? <div className="fish-warning-list" role="status">{report.issues.map((issue, index) => <span key={`${issue.field}-${index}`}>{issue.message}</span>)}</div> : report ? <p className="fish-good">配置字段未发现矛盾；这不是网页实测通过的证明。</p> : null}
    <details>
      <summary>网站地理定位覆盖（可选）</summary>
      <p>请使用代理出口附近的坐标。覆盖应用于页面、弹窗及跨站框架，不自动授予网站定位权限；切换代理后需重新核对。</p>
      <div className="fish-form-grid three">
        <div className="fish-field"><label htmlFor="fish-geo-lat">纬度</label><input id="fish-geo-lat" className="fish-input" type="number" min="-90" max="90" step="any" value={latitude} onChange={e => setLatitude(e.target.value)} /></div>
        <div className="fish-field"><label htmlFor="fish-geo-lon">经度</label><input id="fish-geo-lon" className="fish-input" type="number" min="-180" max="180" step="any" value={longitude} onChange={e => setLongitude(e.target.value)} /></div>
        <div className="fish-field"><label htmlFor="fish-geo-accuracy">精度（米）</label><input id="fish-geo-accuracy" className="fish-input" type="number" min="1" value={accuracy} onChange={e => setAccuracy(e.target.value)} /></div>
      </div>
      <div className="fish-inline-actions"><button className="fish-btn" type="button" disabled={busy || !canGenerate} onClick={applyGeo}>应用定位</button><button className="fish-btn" type="button" disabled={busy || !canGenerate || !identityArg(args, '--ant-geolocation')} onClick={() => void apply({ clearGeo: true })}>清除定位覆盖</button></div>
    </details>
    {error ? <div className="fish-validation error" role="alert">{error}</div> : null}
  </section>
}
