import { useCallback, useEffect, useMemo, useState } from 'react'
import { Copy, KeyRound, Network, RefreshCw, RotateCcw, ShieldCheck, Trash2 } from 'lucide-react'
import { toast } from '../../shared/components'
import type { BrowserNetworkObservation, BrowserSyncStatusV1, BrowserSystemIntegrationStatus, BrowserTrashItem } from '../../modules/browser/types'
import {
  cleanupBrowserTrash,
  getBrowserControlDiagnostics,
  getBrowserSyncStatus,
  getBrowserSystemIntegrationStatus,
  listBrowserTrash,
  observeHostNetwork,
  permanentlyDeleteBrowserTrash,
  restoreBrowserTrash,
  revokeBrowserLaunchAuth,
  rotateBrowserLaunchAuth,
  setBrowserProtocolRegistration,
} from '../../modules/browser/api'
import { fetchLaunchServerSettings } from '../../modules/settings/api'
import './fish-modules.css'

const allScopes=['environment-read','page-read','page-action','script','environment-admin','backup-credential-export']

export function FishControlCenterPage(){
  const [diagnostics,setDiagnostics]=useState<any>(null)
  const [sync,setSync]=useState<BrowserSyncStatusV1|null>(null)
  const [system,setSystem]=useState<BrowserSystemIntegrationStatus|null>(null)
  const [trash,setTrash]=useState<BrowserTrashItem[]>([])
  const [launch,setLaunch]=useState<any>(null)
  const [scopes,setScopes]=useState<string[]>(['environment-read','page-read','page-action','script','environment-admin'])
  const [newKey,setNewKey]=useState('')
  const [probeUrl,setProbeUrl]=useState('https://example.com')
  const [observation,setObservation]=useState<BrowserNetworkObservation|null>(null)
  const [busy,setBusy]=useState('')
  const load=useCallback(async()=>{try{
    const [d,s,sys,t,l]=await Promise.all([getBrowserControlDiagnostics(),getBrowserSyncStatus(),getBrowserSystemIntegrationStatus(),listBrowserTrash(),fetchLaunchServerSettings()])
    setDiagnostics(d);setSync(s);setSystem(sys);setTrash(t||[]);setLaunch(l)
    const configured=(d as any)?.launchServer?.apiAuth?.scopes
    if(Array.isArray(configured)&&configured.length)setScopes(configured.includes('*')?allScopes:configured)
  }catch(e){toast.error(e instanceof Error?e.message:'控制中心加载失败')}},[])
  useEffect(()=>{void load()},[load])
  const auth=(diagnostics as any)?.launchServer?.apiAuth||{}
  const toggleScope=(scope:string)=>setScopes(current=>current.includes(scope)?current.filter(v=>v!==scope):[...current,scope])
  const rotate=async()=>{if(!scopes.length){toast.warning('至少选择一个权限');return}setBusy('auth');try{const r=await rotateBrowserLaunchAuth(scopes);setNewKey(r.apiKey);toast.success('API Key 已轮换，请立即保存新 Key');await load()}catch(e){toast.error(e instanceof Error?e.message:'轮换失败')}finally{setBusy('')}}
  const revoke=async()=>{setBusy('auth');try{await revokeBrowserLaunchAuth();setNewKey('');toast.success('API 鉴权已撤销');await load()}catch(e){toast.error(e instanceof Error?e.message:'撤销失败')}finally{setBusy('')}}
  const runProbe=async()=>{setBusy('probe');try{const v=await observeHostNetwork(probeUrl);setObservation(v);toast.success('网络观测已完成')}catch(e:any){toast.error(e?.message||'网络观测失败')}finally{setBusy('')}}
  const restore=async(id:string)=>{setBusy(id);try{await restoreBrowserTrash(id);toast.success('实例已恢复');await load()}catch(e){toast.error(e instanceof Error?e.message:'恢复失败')}finally{setBusy('')}}
  const remove=async(id:string)=>{setBusy(id);try{await permanentlyDeleteBrowserTrash(id);toast.success('实例已永久删除');await load()}catch(e){toast.error(e instanceof Error?e.message:'删除失败')}finally{setBusy('')}}
  const toggleProtocol=async()=>{if(!system)return;setBusy('system');try{setSystem(await setBrowserProtocolRegistration(!system.registered));toast.success(system.registered?'协议注册已移除':'antbrowser:// 已注册')}catch(e){toast.error(e instanceof Error?e.message:'系统集成更新失败')}finally{setBusy('')}}
  const tlsSummary=useMemo(()=>observation?[`HTTP ${observation.httpProtocol||'—'}`,String((observation.tls as any)?.version||'TLS —'),String((observation.tls as any)?.negotiatedProtocol||'ALPN —')].join(' · '):'',[observation])
  return <div className="fish-content fish-module-content">
    <section className="fish-module-hero"><div><span className="fish-eyebrow">Control Plane</span><h1>控制中心</h1><p>API 权限、网络证据、回收站、备份通道与可选系统集成。</p></div><button className="fish-btn" onClick={()=>void load()}><RefreshCw size={16}/>刷新</button></section>
    <section className="fish-module-strip"><div><span>Launch API</span><strong>{launch?.ready?`:${launch.port}`:'未就绪'}</strong></div><div><span>SecretStore</span><strong>{diagnostics?.secretStoreReady?'Ready':'Unavailable'}</strong></div><div><span>批量任务记录</span><strong>{diagnostics?.batchOperationCount??0}</strong></div><div><span>回收站</span><strong>{trash.length}</strong></div></section>

    <div className="fish-control-grid">
      <section className="fish-panel"><div className="fish-panel-head"><div><strong>API / MCP 鉴权</strong><span>HTTP 与 MCP 共用能力范围。</span></div><ShieldCheck size={18}/></div><div className="fish-panel-body">
        <div className="fish-summary-list"><div><span>状态</span><strong>{auth.enabled?'已启用':'未启用'}</strong></div><div><span>Header</span><strong>{auth.header||'X-Ant-API-Key'}</strong></div></div>
        <div className="fish-scope-grid">{allScopes.map(scope=><label key={scope} className="fish-scope-item"><input type="checkbox" checked={scopes.includes(scope)} onChange={()=>toggleScope(scope)}/><span>{scope}</span></label>)}</div>
        <div className="fish-inline-actions"><button className="fish-btn primary" disabled={busy==='auth'} onClick={()=>void rotate()}><RotateCcw size={15}/>轮换 Key</button>{auth.enabled?<button className="fish-btn danger" disabled={busy==='auth'} onClick={()=>void revoke()}><KeyRound size={15}/>撤销鉴权</button>:null}</div>
        {newKey?<div className="fish-secret-once"><strong>新 API Key（仅本次显示）</strong><code>{newKey}</code><button className="fish-btn" onClick={()=>void navigator.clipboard.writeText(newKey)}><Copy size={14}/>复制</button></div>:null}
      </div></section>

      <section className="fish-panel"><div className="fish-panel-head"><div><strong>系统与同步</strong><span>系统集成默认关闭；Sync 明确使用备份通道。</span></div></div><div className="fish-panel-body fish-summary-list">
        <div><span>同步模式</span><strong>{sync?.mode||'—'}</strong></div><div><span>实时双向</span><strong>{sync?.bidirectional?'是':'否'}</strong></div><div><span>本地目录</span><strong className="fish-breakable">{sync?.localDirectory||'未配置'}</strong></div>
        <div><span>{system?.protocol||'antbrowser'}://</span><strong>{system?.registered?'已注册':'未注册'}</strong></div>
        <div><span>系统支持</span><strong>{system?.supported?'支持':'不支持'}</strong></div>
        <button className="fish-btn" disabled={!system?.supported||busy==='system'} onClick={()=>void toggleProtocol()}>{system?.registered?'注销 URL 协议':'注册 URL 协议'}</button>
      </div></section>
    </div>

    <section className="fish-panel fish-control-section"><div className="fish-panel-head"><div><strong>网络证据</strong><span>仅观测公开目标；私网/回环/重定向到私网会被拒绝。</span></div><Network size={18}/></div><div className="fish-panel-body">
      <div className="fish-inline-control"><input className="fish-input" value={probeUrl} onChange={e=>setProbeUrl(e.target.value)} placeholder="https://example.com"/><button className="fish-btn primary" disabled={busy==='probe'} onClick={()=>void runProbe()}>{busy==='probe'?'观测中':'开始观测'}</button></div>
      {observation?<div className="fish-network-result"><div className="fish-module-strip compact"><div><span>结果</span><strong>{observation.ok?'OK':'Failed'}</strong></div><div><span>耗时</span><strong>{observation.totalMs} ms</strong></div><div><span>协议</span><strong>{observation.httpProtocol||'—'}</strong></div><div><span>证据</span><strong>{observation.id.slice(0,8)}</strong></div></div><div className="fish-inline-note">{tlsSummary}</div><pre className="fish-code-surface">{JSON.stringify({dns:observation.dns,connection:observation.connection,tls:observation.tls,ja3:observation.ja3,ja4:observation.ja4,quic:observation.quic,http3:observation.http3},null,2)}</pre></div>:null}
    </div></section>

    <section className="fish-panel fish-control-section"><div className="fish-panel-head"><div><strong>回收站</strong><span>软删除保留 72 小时，可恢复或永久删除。</span></div><div className="fish-panel-actions"><button className="fish-btn" onClick={async()=>{try{await cleanupBrowserTrash();toast.success('已清理过期项');await load()}catch(e){toast.error(e instanceof Error?e.message:'清理失败')}}}>清理过期</button></div></div><div className="fish-panel-body no-pad">
      {trash.length?trash.map(item=><div className="fish-flat-row" key={item.profileId}><span className="fish-row-mark"><Trash2 size={15}/></span><div className="fish-flat-row-main"><strong>{item.profileName}</strong><small>{item.profileId} · 删除于 {item.deletedAt||'—'}</small></div><div className="fish-inline-actions"><button className="fish-btn" disabled={busy===item.profileId} onClick={()=>void restore(item.profileId)}>恢复</button><button className="fish-btn danger" disabled={busy===item.profileId} onClick={()=>void remove(item.profileId)}>永久删除</button></div></div>):<div className="fish-empty compact"><strong>回收站为空</strong><span>软删除的浏览器环境会出现在这里。</span></div>}
    </div></section>
  </div>
}
