import { useEffect, useMemo, useState } from 'react'
import {
  AlertTriangle,
  Check,
  Copy,
  Download,
  Gauge,
  Plus,
  RefreshCw,
  RotateCw,
  Search,
  Trash2,
  Upload,
  X,
} from 'lucide-react'
import { toast } from '../../shared/components'
import type { BrowserExtension, BrowserProfile, BrowserProxy } from '../../modules/browser/types'
import {
  browserProxyTestSpeed,
  exportBrowserProfilePackage,
  exportFullBrowserBackup,
  fetchBrowserProfileTrash,
  importFullBrowserBackup,
  permanentlyDeleteBrowserProfile,
  regenerateBrowserProfileCode,
  restoreBrowserProfile,
  setBrowserProfileCode,
  setProfileKeywords,
} from '../../modules/browser/api'
import {
  fetchBrowserExtensions,
  fetchBrowserProfileExtensionSettings,
  saveBrowserProfileExtensionSettings,
} from '../../modules/browser/api/extensions'
import type { BackupExportProgress } from '../../modules/settings/progress'
import { useBackupStore } from '../../store/backupStore'
import { EventsOn } from '../../wailsjs/runtime/runtime'

type CloseProps = { onClose: () => void }

interface ProfileModalProps extends CloseProps {
  profile: BrowserProfile
  onChanged: () => unknown | Promise<unknown>
}

export function KeywordsV2Modal({ profile, onClose, onChanged }: ProfileModalProps) {
  const [items, setItems] = useState<string[]>(() => profile.keywords?.length ? [...profile.keywords] : [''])
  const [saving, setSaving] = useState(false)

  const save = async () => {
    const keywords = items.map((item) => item.trim()).filter(Boolean)
    setSaving(true)
    try {
      await setProfileKeywords(profile.profileId, keywords)
      toast.success('关键字已保存')
      await onChanged()
      onClose()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '关键字保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="ui-v2-backdrop" onClick={onClose}>
      <div className="ui-v2-modal" onClick={(event) => event.stopPropagation()}>
        <div className="ui-v2-modal-head"><strong>关键字 · {profile.profileName}</strong><span className="ui-v2-spacer" /><button className="ui-v2-mini-icon" type="button" onClick={onClose}><X size={16} strokeWidth={1.8} /></button></div>
        <div className="ui-v2-modal-body">
          <p className="ui-v2-modal-note">关键字用于账号、用途和备注等附加信息，也会参与实例列表搜索。</p>
          <div className="ui-v2-stack">
            {items.map((item, index) => (
              <div className="ui-v2-inline-field" key={index}>
                <input className="ui-v2-input" value={item} onChange={(event) => setItems((current) => current.map((value, itemIndex) => itemIndex === index ? event.target.value : value))} placeholder="输入关键字" />
                <button className="ui-v2-mini-icon" type="button" title="删除关键字" onClick={() => setItems((current) => current.length === 1 ? [''] : current.filter((_, itemIndex) => itemIndex !== index))}><Trash2 size={16} strokeWidth={1.8} /></button>
              </div>
            ))}
          </div>
          <button className="ui-v2-btn" type="button" onClick={() => setItems((current) => [...current, ''])}><Plus size={16} strokeWidth={1.8} />添加关键字</button>
        </div>
        <div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" disabled={saving} onClick={onClose}>取消</button><button className="ui-v2-btn primary" type="button" disabled={saving} onClick={() => void save()}>{saving ? '保存中' : '保存'}</button></div>
      </div>
    </div>
  )
}

export function LaunchCodeV2Modal({ profile, onClose, onChanged }: ProfileModalProps) {
  const [value, setValue] = useState(profile.launchCode || '')
  const [busy, setBusy] = useState(false)

  const save = async () => {
    const code = value.trim()
    if (!/^[A-Za-z0-9_-]{4,32}$/.test(code)) {
      toast.error('Code 需为 4-32 位，仅支持字母、数字、_、-')
      return
    }
    setBusy(true)
    try {
      const applied = await setBrowserProfileCode(profile.profileId, code)
      setValue(applied)
      toast.success(`Code 已更新为 ${applied}`)
      await onChanged()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '快捷码更新失败')
    } finally {
      setBusy(false)
    }
  }

  const regenerate = async () => {
    setBusy(true)
    try {
      const code = await regenerateBrowserProfileCode(profile.profileId)
      if (code) setValue(code)
      toast.success('快捷码已重新生成')
      await onChanged()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '重新生成失败')
    } finally {
      setBusy(false)
    }
  }

  const copy = async () => {
    if (!value) return
    await navigator.clipboard.writeText(value)
    toast.success('快捷码已复制')
  }

  return (
    <div className="ui-v2-backdrop" onClick={onClose}>
      <div className="ui-v2-modal" onClick={(event) => event.stopPropagation()}>
        <div className="ui-v2-modal-head"><strong>快捷打开码 · {profile.profileName}</strong><span className="ui-v2-spacer" /><button className="ui-v2-mini-icon" type="button" onClick={onClose}><X size={16} strokeWidth={1.8} /></button></div>
        <div className="ui-v2-modal-body">
          <p className="ui-v2-modal-note">Quick Launch 和自动化入口会使用该 Code 定位实例。修改后请同步更新外部脚本。</p>
          <div className="ui-v2-field"><label>当前 Code</label><div className="ui-v2-inline-field"><input className="ui-v2-input ui-v2-code" value={value} onChange={(event) => setValue(event.target.value.toUpperCase())} placeholder="例如 WORK_01" /><button className="ui-v2-icon-btn" type="button" title="复制" onClick={() => void copy()}><Copy size={16} strokeWidth={1.8} /></button></div></div>
          <button className="ui-v2-btn" type="button" disabled={busy} onClick={() => void regenerate()}><RefreshCw size={16} strokeWidth={1.8} />重新生成</button>
        </div>
        <div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" disabled={busy} onClick={onClose}>关闭</button><button className="ui-v2-btn primary" type="button" disabled={busy} onClick={() => void save()}>{busy ? '处理中' : '应用 Code'}</button></div>
      </div>
    </div>
  )
}

export function ProfileExtensionsV2Modal({ profile, onClose }: Omit<ProfileModalProps, 'onChanged'>) {
  const [extensions, setExtensions] = useState<BrowserExtension[]>([])
  const [configured, setConfigured] = useState(false)
  const [selectedIds, setSelectedIds] = useState<string[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const selectedSet = useMemo(() => new Set(selectedIds), [selectedIds])

  useEffect(() => {
    let active = true
    Promise.all([fetchBrowserExtensions(), fetchBrowserProfileExtensionSettings(profile.profileId)])
      .then(([items, settings]) => {
        if (!active) return
        setExtensions(items)
        setConfigured(settings.configured)
        setSelectedIds(settings.extensionIds)
      })
      .catch((error) => toast.error(error instanceof Error ? error.message : '加载实例插件配置失败'))
      .finally(() => active && setLoading(false))
    return () => { active = false }
  }, [profile.profileId])

  const save = async () => {
    setSaving(true)
    try {
      await saveBrowserProfileExtensionSettings(profile.profileId, selectedIds, configured)
      toast.success('实例插件配置已保存')
      onClose()
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '保存实例插件配置失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="ui-v2-backdrop" onClick={onClose}>
      <div className="ui-v2-modal wide" onClick={(event) => event.stopPropagation()}>
        <div className="ui-v2-modal-head"><strong>实例插件 · {profile.profileName}</strong><span className="ui-v2-spacer" /><button className="ui-v2-mini-icon" type="button" onClick={onClose}><X size={16} strokeWidth={1.8} /></button></div>
        <div className="ui-v2-modal-body">
          <label className="ui-v2-setting-row"><div><strong>单独配置此实例</strong><span>关闭时继承全局已启用插件；打开后只加载下方勾选项。</span></div><input type="checkbox" checked={configured} onChange={(event) => setConfigured(event.target.checked)} /></label>
          {loading ? <div className="ui-v2-empty compact"><strong>正在加载插件</strong></div> : (
            <div className="ui-v2-choice-list">
              {extensions.map((extension) => (
                <label className={`ui-v2-choice-row${!configured ? ' disabled' : ''}`} key={extension.extensionId}>
                  <input type="checkbox" disabled={!configured} checked={selectedSet.has(extension.extensionId)} onChange={(event) => setSelectedIds((current) => event.target.checked ? (current.includes(extension.extensionId) ? current : [...current, extension.extensionId]) : current.filter((id) => id !== extension.extensionId))} />
                  <div><strong>{extension.name || extension.extensionId}</strong><span>{extension.version ? `v${extension.version} · ` : ''}{extension.enabled ? extension.extensionId : `全局停用 · ${extension.extensionId}`}</span></div>
                </label>
              ))}
              {extensions.length === 0 ? <div className="ui-v2-empty compact"><strong>还没有安装插件</strong><span>请先到插件包管理页面安装。</span></div> : null}
            </div>
          )}
        </div>
        <div className="ui-v2-modal-foot"><button className="ui-v2-btn" type="button" disabled={saving} onClick={onClose}>取消</button><button className="ui-v2-btn primary" type="button" disabled={loading || saving} onClick={() => void save()}>{saving ? '保存中' : '保存'}</button></div>
      </div>
    </div>
  )
}

interface ProxyPickerV2Props extends CloseProps {
  profile: BrowserProfile
  proxies: BrowserProxy[]
  onSelect: (proxy: BrowserProxy) => Promise<void>
}

export function ProxyPickerV2Modal({ profile, proxies, onSelect, onClose }: ProxyPickerV2Props) {
  const [search, setSearch] = useState('')
  const [group, setGroup] = useState('')
  const [testingId, setTestingId] = useState('')
  const [speedMap, setSpeedMap] = useState<Record<string, { ok: boolean; latencyMs: number; error: string }>>({})
  const groups = useMemo(() => Array.from(new Set(proxies.map((proxy) => proxy.groupName || '').filter(Boolean))).sort(), [proxies])
  const filtered = useMemo(() => proxies.filter((proxy) => {
    if (group && proxy.groupName !== group) return false
    if (!search.trim()) return true
    const query = search.trim().toLowerCase()
    return `${proxy.proxyName} ${proxy.proxyConfig}`.toLowerCase().includes(query)
  }), [group, proxies, search])

  const test = async (proxy: BrowserProxy) => {
    setTestingId(proxy.proxyId)
    try {
      const result = await browserProxyTestSpeed(proxy.proxyId)
      setSpeedMap((current) => ({ ...current, [proxy.proxyId]: { ok: result.ok, latencyMs: result.latencyMs, error: result.error } }))
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '代理测速失败')
    } finally {
      setTestingId('')
    }
  }

  const select = async (proxy: BrowserProxy) => {
    try {
      await onSelect(proxy)
      onClose()
    } catch {
      // onSelect 已反馈具体错误，保留弹窗供用户重新选择。
    }
  }

  return (
    <div className="ui-v2-backdrop" onClick={onClose}>
      <div className="ui-v2-modal xwide" onClick={(event) => event.stopPropagation()}>
        <div className="ui-v2-modal-head"><strong>切换代理 · {profile.profileName}</strong><span className="ui-v2-spacer" /><button className="ui-v2-mini-icon" type="button" onClick={onClose}><X size={16} strokeWidth={1.8} /></button></div>
        <div className="ui-v2-modal-body">
          <div className="ui-v2-toolbar modal-toolbar"><label className="ui-v2-search"><Search size={16} strokeWidth={1.8} /><input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="搜索代理名称或配置" /></label><select className="ui-v2-select" value={group} onChange={(event) => setGroup(event.target.value)}><option value="">全部分组</option>{groups.map((item) => <option value={item} key={item}>{item}</option>)}</select></div>
          <div className="ui-v2-choice-list proxy-list">
            {filtered.map((proxy) => {
              const speed = speedMap[proxy.proxyId]
              const selected = proxy.proxyId === profile.proxyId
              return <div className={`ui-v2-proxy-row${selected ? ' selected' : ''}`} key={proxy.proxyId}><button className="ui-v2-proxy-main" type="button" onClick={() => void select(proxy)}><span className="ui-v2-proxy-state">{selected ? <Check size={16} strokeWidth={1.8} /> : null}</span><div><strong>{proxy.proxyName || proxy.proxyId}</strong><span>{proxy.groupName || '未分组'} · {proxy.proxyConfig || '—'}</span></div></button><div className="ui-v2-proxy-speed">{speed ? (speed.ok ? `${speed.latencyMs} ms` : speed.error || '失败') : proxy.lastTestedAt ? (proxy.lastTestOk ? `${proxy.lastLatencyMs ?? '-'} ms` : '上次失败') : '未测速'}</div><button className="ui-v2-mini-icon" type="button" disabled={testingId === proxy.proxyId} title="测速" onClick={() => void test(proxy)}><Gauge size={16} strokeWidth={1.8} /></button></div>
            })}
            {filtered.length === 0 ? <div className="ui-v2-empty compact"><strong>没有匹配的代理</strong></div> : null}
          </div>
          <p className="ui-v2-modal-note">这里仅负责为实例选择现有代理。新增、编辑、订阅和删除代理统一放在“代理池配置”。</p>
        </div>
      </div>
    </div>
  )
}

interface TrashV2Props extends CloseProps {
  onChanged: () => unknown | Promise<unknown>
}

export function TrashV2Modal({ onClose, onChanged }: TrashV2Props) {
  const [profiles, setProfiles] = useState<BrowserProfile[]>([])
  const [loading, setLoading] = useState(true)
  const [busyId, setBusyId] = useState('')
  const [permanentCandidate, setPermanentCandidate] = useState<BrowserProfile | null>(null)

  const load = async () => {
    setLoading(true)
    try { setProfiles(await fetchBrowserProfileTrash()) } catch (error) { toast.error(error instanceof Error ? error.message : '加载回收站失败') } finally { setLoading(false) }
  }
  useEffect(() => { void load() }, [])

  const restore = async (profile: BrowserProfile) => {
    setBusyId(profile.profileId)
    try { await restoreBrowserProfile(profile.profileId); toast.success('实例已恢复'); await load(); await onChanged() } catch (error) { toast.error(error instanceof Error ? error.message : '恢复失败') } finally { setBusyId('') }
  }
  const permanentDelete = async () => {
    if (!permanentCandidate) return
    setBusyId(permanentCandidate.profileId)
    try { await permanentlyDeleteBrowserProfile(permanentCandidate.profileId); toast.success('实例已彻底删除'); setPermanentCandidate(null); await load(); await onChanged() } catch (error) { toast.error(error instanceof Error ? error.message : '彻底删除失败') } finally { setBusyId('') }
  }
  const formatTime = (value?: string) => value ? new Date(value).toLocaleString('zh-CN') : '—'
  const formatExpire = (value?: string) => { if (!value) return '—'; const date = new Date(value); date.setDate(date.getDate() + 3); return date.toLocaleString('zh-CN') }

  return (
    <div className="ui-v2-backdrop" onClick={onClose}>
      <div className="ui-v2-modal xwide" onClick={(event) => event.stopPropagation()}>
        <div className="ui-v2-modal-head"><strong>实例回收站</strong><span className="ui-v2-spacer" /><button className="ui-v2-mini-icon" type="button" onClick={onClose}><X size={16} strokeWidth={1.8} /></button></div>
        <div className="ui-v2-modal-body">
          {loading ? <div className="ui-v2-empty compact"><strong>正在加载回收站</strong></div> : profiles.length === 0 ? <div className="ui-v2-empty compact"><strong>回收站为空</strong><span>被删除的实例会在这里保留 3 天。</span></div> : <div className="ui-v2-trash-list">{profiles.map((profile) => <div className="ui-v2-trash-row" key={profile.profileId}><div><strong>{profile.profileName || '未命名实例'}</strong><span>删除：{formatTime(profile.deletedAt)} · 自动清理：{formatExpire(profile.deletedAt)}</span></div><button className="ui-v2-btn" type="button" disabled={!!busyId} onClick={() => void restore(profile)}><RotateCw size={16} strokeWidth={1.8} />恢复</button><button className="ui-v2-btn danger" type="button" disabled={!!busyId} onClick={() => setPermanentCandidate(profile)}><Trash2 size={16} strokeWidth={1.8} />彻底删除</button></div>)}</div>}
        </div>
        {permanentCandidate ? <div className="ui-v2-danger-panel"><AlertTriangle size={18} strokeWidth={1.8} /><div><strong>彻底删除「{permanentCandidate.profileName}」？</strong><span>配置、用户数据、快照、快捷码和插件绑定都会删除，无法恢复。</span></div><button className="ui-v2-btn" type="button" disabled={!!busyId} onClick={() => setPermanentCandidate(null)}>取消</button><button className="ui-v2-btn danger" type="button" disabled={!!busyId} onClick={() => void permanentDelete()}>确认彻底删除</button></div> : null}
      </div>
    </div>
  )
}

type BackupMode = 'none' | 'export' | 'selected' | 'import-merge' | 'import-reset'
interface BackupV2Props extends CloseProps {
  selectedProfiles: BrowserProfile[]
  runningCount: number
  onChanged: () => unknown | Promise<unknown>
}

export function BackupV2Modal({ selectedProfiles, runningCount, onClose, onChanged }: BackupV2Props) {
  const [mode, setMode] = useState<BackupMode>('none')
  const [progress, setProgress] = useState<BackupExportProgress | null>(null)
  const [resetArmed, setResetArmed] = useState(false)
  const setImportState = useBackupStore((state) => state.setImportState)
  const clearImportState = useBackupStore((state) => state.clearImportState)
  const busy = mode !== 'none'

  useEffect(() => {
    const off = EventsOn('backup:import:progress', (payload: BackupExportProgress) => {
      if (!payload || typeof payload !== 'object') return
      const next = { ...payload, progress: Math.max(0, Math.min(100, Number(payload.progress) || 0)), message: String(payload.message || '正在加载配置...') }
      if (next.phase === 'cancelled') { setProgress(null); return }
      setProgress(next)
    })
    return () => { off?.(); clearImportState() }
  }, [clearImportState])

  useEffect(() => {
    const importing = mode === 'import-merge' || mode === 'import-reset'
    if (importing) setImportState({ inProgress: true, progress: progress?.progress || 0, message: progress?.message || '正在加载配置...' })
    else clearImportState()
  }, [clearImportState, mode, progress?.message, progress?.progress, setImportState])

  const exportSelected = async () => {
    if (selectedProfiles.length === 0) return
    const running = selectedProfiles.filter((profile) => profile.running)
    if (running.length > 0) { toast.error(`请先停止实例再导出：${running.slice(0, 3).map((profile) => profile.profileName).join('、')}${running.length > 3 ? ' 等' : ''}`); return }
    setMode('selected')
    try { const result = await exportBrowserProfilePackage(selectedProfiles.map((profile) => profile.profileId)); if (!result.cancelled) toast.success(`已导出 ${result.profileCount} 个实例`) } catch (error) { toast.error(error instanceof Error ? error.message : '备份选中实例失败') } finally { setMode('none') }
  }
  const exportFull = async () => {
    if (runningCount > 0) toast.warning(`建议先停止 ${runningCount} 个运行中实例后再备份`)
    setMode('export')
    try { const result = await exportFullBrowserBackup(); if (!result.cancelled) toast.success(result.zipPath ? `备份已导出：${result.zipPath}` : result.message || '备份已导出') } catch (error) { toast.error(error instanceof Error ? error.message : '全量备份失败') } finally { setMode('none') }
  }
  const importBackup = async (resetFirst: boolean) => {
    setMode(resetFirst ? 'import-reset' : 'import-merge')
    setProgress({ phase: 'starting', progress: 0, message: resetFirst ? '正在准备清空恢复...' : '正在准备合并导入...' })
    try { const result = await importFullBrowserBackup(resetFirst); if (!result.cancelled) { toast.success(result.message || (resetFirst ? '备份已恢复' : '备份已合并')); await onChanged(); onClose() } } catch (error) { toast.error(error instanceof Error ? error.message : '导入备份失败') } finally { setMode('none'); setProgress(null); setResetArmed(false) }
  }

  return (
    <div className="ui-v2-backdrop" onClick={() => { if (!busy) onClose() }}>
      <div className="ui-v2-modal wide" onClick={(event) => event.stopPropagation()}>
        <div className="ui-v2-modal-head"><strong>备份与恢复</strong><span className="ui-v2-spacer" /><button className="ui-v2-mini-icon" type="button" disabled={busy} onClick={onClose}><X size={16} strokeWidth={1.8} /></button></div>
        <div className="ui-v2-modal-body">
          <div className="ui-v2-backup-grid"><div><strong>备份选中</strong><span>当前选择 {selectedProfiles.length} 个实例及其浏览器用户数据。</span><button className="ui-v2-btn" type="button" disabled={busy || selectedProfiles.length === 0} onClick={() => void exportSelected()}><Download size={16} strokeWidth={1.8} />{mode === 'selected' ? '导出中' : '导出选中'}</button></div><div><strong>全量备份</strong><span>包含实例、代理、内核、数据库、插件和应用级配置。</span><button className="ui-v2-btn primary" type="button" disabled={busy} onClick={() => void exportFull()}><Download size={16} strokeWidth={1.8} />{mode === 'export' ? '备份中' : '创建全量备份'}</button></div></div>
          {runningCount > 0 ? <div className="ui-v2-warning-panel"><AlertTriangle size={18} strokeWidth={1.8} /><span>当前有 {runningCount} 个实例运行中。建议停止后再备份，避免 Cookie、数据库和缓存尚未落盘。</span></div> : null}
          <div className="ui-v2-backup-import"><div><strong>合并导入</strong><span>保留当前数据，根据 ID、路径和 URL 规则跳过重复项。</span></div><button className="ui-v2-btn" type="button" disabled={busy} onClick={() => void importBackup(false)}><Upload size={16} strokeWidth={1.8} />{mode === 'import-merge' ? '导入中' : '合并导入'}</button></div>
          <div className="ui-v2-backup-import danger"><div><strong>清空恢复</strong><span>先初始化当前数据，再从备份包完整恢复。此操作会替换当前数据。</span></div><button className="ui-v2-btn danger" type="button" disabled={busy} onClick={() => resetArmed ? void importBackup(true) : setResetArmed(true)}>{resetArmed ? '再次点击确认清空恢复' : '清空恢复'}</button></div>
          {progress ? <div className="ui-v2-progress-card"><div><span>{progress.message}</span><strong>{Math.round(progress.progress)}%</strong></div><div className="ui-v2-progress-track"><span style={{ width: `${Math.max(0, Math.min(100, progress.progress))}%` }} /></div></div> : null}
        </div>
      </div>
    </div>
  )
}
