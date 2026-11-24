import { useEffect, useMemo, useState } from 'react'
import { RefreshCw, Save, ShieldCheck } from 'lucide-react'
import type { BrowserEnvironmentViewV2, BrowserNetworkPolicyV2 } from '../../modules/browser/types'
import {
  applyBrowserEnvironmentTemplate,
  fetchBrowserEnvironmentView,
  fetchBrowserNetworkPolicy,
  saveBrowserNetworkPolicy,
} from '../../modules/browser/api'
import { toast } from '../../shared/components'

export function FishEnvironmentPanel({profileId,running}:{profileId:string;running:boolean}){
  const [view,setView]=useState<BrowserEnvironmentViewV2|null>(null)
  const [policy,setPolicy]=useState<BrowserNetworkPolicyV2|null>(null)
  const [busy,setBusy]=useState(false)
  const load=async()=>{try{const [v,p]=await Promise.all([fetchBrowserEnvironmentView(profileId),fetchBrowserNetworkPolicy(profileId)]);setView(v);setPolicy(p)}catch(e){toast.error(e instanceof Error?e.message:'环境契约加载失败')}}
  useEffect(()=>{void load()},[profileId])
  const compatibility=view?.templateCompatibility
  const state=String((view?.launchState as any)?.state||'configured')
  const boundTemplate=useMemo(()=>String((view?.contract as any)?.template?.templateId||''),[view])
  const savePolicy=async()=>{if(!policy)return;setBusy(true);try{setPolicy(await saveBrowserNetworkPolicy(policy));toast.success('NetworkPolicy 已保存');await load()}catch(e){toast.error(e instanceof Error?e.message:'保存失败')}finally{setBusy(false)}}
  const applyTemplate=async(id:string)=>{setBusy(true);try{await applyBrowserEnvironmentTemplate(profileId,id);toast.success('模板已应用');await load()}catch(e){toast.error(e instanceof Error?e.message:'模板应用失败')}finally{setBusy(false)}}
  if(!view||!policy)return <div className="fish-empty compact"><strong>正在读取环境契约</strong><span>加载 LaunchPlan、模板和 NetworkPolicy。</span></div>
  return <div className="fish-detail-stack">
    <div className="fish-detail-summary-grid">
      <section className="fish-panel"><div className="fish-panel-head"><div><strong>EnvironmentView v2</strong><span>配置状态与运行状态分开显示。</span></div><span className={`fish-status-pill${state==='effective'?' good':state==='pending-next-launch'?' accent':''}`}>{state}</span></div><div className="fish-panel-body fish-summary-list">
        <div><span>Environment ID</span><strong className="fish-breakable">{String((view.contract as any)?.identity?.environmentId||(view.contract as any)?.environmentId||'—')}</strong></div>
        <div><span>Revision</span><strong>{String((view.launchState as any)?.environmentRevision||'—')}</strong></div>
        <div><span>Template</span><strong>{boundTemplate||'未绑定'}</strong></div>
        <div><span>Launch Receipt</span><strong>{view.launchReceipt?String((view.launchReceipt as any)?.state||(view.launchReceipt as any)?.status||'存在'):'暂无'}</strong></div>
        <div><span>Runtime Evidence</span><strong>{view.runtimeEvidence?'已有显式证据':'未验证'}</strong></div>
      </div></section>
      <section className="fish-panel"><div className="fish-panel-head"><div><strong>模板兼容性</strong><span>eligible 与 verified 分离。</span></div><ShieldCheck size={17}/></div><div className="fish-panel-body fish-summary-list">
        <div><span>状态</span><strong>{compatibility?.status||'unbound'}</strong></div><div><span>Eligible</span><strong>{compatibility?.eligible?'是':'否'}</strong></div><div><span>Verified</span><strong>{compatibility?.verified?'是':'否'}</strong></div><div><span>Target OS</span><strong>{compatibility?.targetOS||'—'}</strong></div><div><span>Device Class</span><strong>{compatibility?.deviceClass||'—'}</strong></div>
        {compatibility?.reasons?.length?<div className="fish-inline-note">{compatibility.reasons.join('；')}</div>:null}
      </div></section>
    </div>
    <section className="fish-panel"><div className="fish-panel-head"><div><strong>环境模板</strong><span>模板 revision 会冻结到环境；运行中不允许直接修改受管环境。</span></div><div className="fish-panel-actions"><button className="fish-btn" onClick={()=>void load()}><RefreshCw size={15}/>刷新</button></div></div><div className="fish-panel-body fish-template-grid">{view.templates.map(t=><button key={t.id} disabled={busy||running} className={`fish-template-card${boundTemplate===t.id?' active':''}`} onClick={()=>void applyTemplate(t.id)}><strong>{t.name}</strong><span>{t.description}</span><small>{t.targetOS||'通用'} · {t.deviceClass||'通用'} · rev {t.revision}</small></button>)}</div></section>
    <section className="fish-panel"><div className="fish-panel-head"><div><strong>NetworkPolicy</strong><span>代理失败默认 fail-closed；WebRTC、DNS 和 locale linkage 显式配置。</span></div><div className="fish-panel-actions"><button className="fish-btn primary" disabled={busy||running} onClick={()=>void savePolicy()}><Save size={15}/>保存</button></div></div><div className="fish-panel-body">
      {running?<div className="fish-alert warn"><strong>实例运行中</strong><span>停止实例后才能修改 NetworkPolicy。</span></div>:null}
      <div className="fish-form-grid three">
        <label className="fish-setting-row compact"><div><strong>Fail Closed</strong><span>代理配置失败时禁止静默直连</span></div><input type="checkbox" checked={policy.failClosed} onChange={e=>setPolicy({...policy,failClosed:e.target.checked})}/></label>
        <label className="fish-field"><span>DNS</span><select className="fish-select" value={policy.dnsMode} onChange={e=>setPolicy({...policy,dnsMode:e.target.value})}><option value="inherit">inherit</option><option value="proxy">proxy</option></select></label>
        <label className="fish-field"><span>WebRTC</span><select className="fish-select" value={policy.webrtcMode} onChange={e=>setPolicy({...policy,webrtcMode:e.target.value})}><option value="restricted">restricted</option><option value="native">native</option></select></label>
        <label className="fish-field"><span>Locale linkage</span><select className="fish-select" value={policy.localeLinkMode} onChange={e=>setPolicy({...policy,localeLinkMode:e.target.value})}><option value="manual">manual</option><option value="recommend">recommend</option></select></label>
      </div>
    </div></section>
  </div>
}
