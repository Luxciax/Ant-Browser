import { useCallback, useEffect, useMemo, useState } from 'react'
import { ArrowDown, ArrowUp, Copy, Pause, Play, Plus, RefreshCw, Save, Trash2, Workflow } from 'lucide-react'
import { toast } from '../../shared/components'
import type { BrowserWorkflow, BrowserWorkflowRun, BrowserWorkflowStep } from '../../modules/browser/types'
import {
  deleteBrowserWorkflow,
  listBrowserWorkflowRuns,
  listBrowserWorkflows,
  resumeBrowserWorkflow,
  runBrowserWorkflow,
  saveBrowserWorkflow,
} from '../../modules/browser/api'
import './fish-modules.css'

const actionOptions = ['goto','snapshot','click','fill','press','selectOption','waitFor','extract','evaluate','screenshot','tabs','upload']

function createStep(type='action'): BrowserWorkflowStep {
  return {
    id: `step-${Math.random().toString(36).slice(2,8)}`,
    type,
    action: type === 'action' ? 'snapshot' : undefined,
    args: type === 'action' ? {} : undefined,
    timeoutMs: type === 'sleep' ? 1000 : 30000,
    retry: type === 'action' ? { maxAttempts: 1, delayMs: 0, allowSideEffects: false } : undefined,
    steps: type === 'if' || type === 'loop' ? [] : undefined,
    elseSteps: type === 'if' ? [] : undefined,
    maxIterations: type === 'loop' ? 3 : undefined,
    condition: type === 'if' ? { variable: '', operator: 'truthy' } : undefined,
  }
}

function newWorkflow(): BrowserWorkflow {
  return {
    schemaVersion: 1,
    id: '',
    name: '新工作流',
    description: '',
    enabled: true,
    selector: {},
    variables: {},
    steps: [createStep()],
    schedule: { enabled: false, everyMinutes: 60, allowOverlap: false },
    createdAt: '',
    updatedAt: '',
  }
}

function JSONField({ value, onChange, placeholder }: { value: any; onChange:(next:any)=>void; placeholder?:string }) {
  const [text,setText]=useState(()=>JSON.stringify(value ?? {},null,2))
  useEffect(()=>setText(JSON.stringify(value ?? {},null,2)),[value])
  return <textarea className="fish-textarea fish-workflow-json" rows={4} value={text} placeholder={placeholder} onChange={e=>{
    setText(e.target.value)
    try{onChange(e.target.value.trim()?JSON.parse(e.target.value):{})}catch{}
  }}/>
}

function StepList({ steps, onChange, depth=0 }: { steps:BrowserWorkflowStep[]; onChange:(steps:BrowserWorkflowStep[])=>void; depth?:number }) {
  const patch=(index:number,next:BrowserWorkflowStep)=>onChange(steps.map((s,i)=>i===index?next:s))
  const move=(index:number,dir:-1|1)=>{const target=index+dir;if(target<0||target>=steps.length)return;const copy=[...steps];[copy[index],copy[target]]=[copy[target],copy[index]];onChange(copy)}
  return <div className={`fish-workflow-steps depth-${Math.min(depth,2)}`}>
    {steps.map((step,index)=><div className="fish-workflow-step" key={step.id}>
      <div className="fish-workflow-step-head">
        <span className="fish-row-mark"><Workflow size={15}/></span>
        <input className="fish-input compact" value={step.id} onChange={e=>patch(index,{...step,id:e.target.value})}/>
        <select className="fish-select compact" value={step.type} onChange={e=>patch(index,{...createStep(e.target.value),id:step.id})}>
          <option value="action">动作</option><option value="set">设置变量</option><option value="if">条件</option><option value="loop">循环</option><option value="sleep">等待</option>
        </select>
        <button className="fish-mini-icon" disabled={index===0} onClick={()=>move(index,-1)}><ArrowUp size={14}/></button>
        <button className="fish-mini-icon" disabled={index===steps.length-1} onClick={()=>move(index,1)}><ArrowDown size={14}/></button>
        <button className="fish-mini-icon danger" onClick={()=>onChange(steps.filter((_,i)=>i!==index))}><Trash2 size={14}/></button>
      </div>
      <div className="fish-workflow-step-body">
        {step.type==='action'?<>
          <div className="fish-form-grid three">
            <label className="fish-field"><span>动作</span><select className="fish-select" value={step.action||'snapshot'} onChange={e=>patch(index,{...step,action:e.target.value})}>{actionOptions.map(v=><option key={v}>{v}</option>)}</select></label>
            <label className="fish-field"><span>保存结果到变量</span><input className="fish-input" value={step.saveAs||''} onChange={e=>patch(index,{...step,saveAs:e.target.value})} placeholder="result"/></label>
            <label className="fish-field"><span>超时 ms</span><input className="fish-input" type="number" min={100} max={300000} value={step.timeoutMs||30000} onChange={e=>patch(index,{...step,timeoutMs:Number(e.target.value)||30000})}/></label>
          </div>
          <label className="fish-field"><span>动作参数 JSON</span><JSONField value={step.args||{}} onChange={args=>patch(index,{...step,args})}/></label>
          <div className="fish-form-grid three">
            <label className="fish-field"><span>最大尝试</span><input className="fish-input" type="number" min={1} max={5} value={step.retry?.maxAttempts||1} onChange={e=>patch(index,{...step,retry:{...step.retry,maxAttempts:Number(e.target.value)||1}})}/></label>
            <label className="fish-field"><span>重试间隔 ms</span><input className="fish-input" type="number" min={0} value={step.retry?.delayMs||0} onChange={e=>patch(index,{...step,retry:{...step.retry,delayMs:Number(e.target.value)||0}})}/></label>
            <label className="fish-setting-row compact"><div><strong>允许副作用重试</strong><span>点击/输入等默认禁止自动重试</span></div><input type="checkbox" checked={!!step.retry?.allowSideEffects} onChange={e=>patch(index,{...step,retry:{...step.retry,allowSideEffects:e.target.checked}})}/></label>
          </div>
        </>:null}
        {step.type==='set'?<div className="fish-form-grid two"><label className="fish-field"><span>变量名</span><input className="fish-input" value={step.saveAs||''} onChange={e=>patch(index,{...step,saveAs:e.target.value})}/></label><label className="fish-field"><span>值</span><input className="fish-input" value={String(step.value??'')} onChange={e=>patch(index,{...step,value:e.target.value})}/></label></div>:null}
        {step.type==='sleep'?<label className="fish-field"><span>等待毫秒</span><input className="fish-input" type="number" min={0} max={300000} value={step.timeoutMs||1000} onChange={e=>patch(index,{...step,timeoutMs:Number(e.target.value)||0})}/></label>:null}
        {step.type==='if'?<>
          <div className="fish-form-grid three">
            <label className="fish-field"><span>变量</span><input className="fish-input" value={step.condition?.variable||''} onChange={e=>patch(index,{...step,condition:{...(step.condition||{operator:'truthy'}),variable:e.target.value}})}/></label>
            <label className="fish-field"><span>运算符</span><select className="fish-select" value={step.condition?.operator||'truthy'} onChange={e=>patch(index,{...step,condition:{...(step.condition||{variable:''}),operator:e.target.value}})}><option value="truthy">truthy</option><option value="eq">eq</option><option value="ne">ne</option><option value="contains">contains</option><option value="exists">exists</option><option value="not-exists">not-exists</option></select></label>
            <label className="fish-field"><span>比较值</span><input className="fish-input" value={String(step.condition?.value??'')} onChange={e=>patch(index,{...step,condition:{...(step.condition||{variable:'',operator:'eq'}),value:e.target.value}})}/></label>
          </div>
          <div className="fish-workflow-branch"><strong>条件成立</strong><StepList depth={depth+1} steps={step.steps||[]} onChange={children=>patch(index,{...step,steps:children})}/></div>
          <div className="fish-workflow-branch"><strong>否则</strong><StepList depth={depth+1} steps={step.elseSteps||[]} onChange={children=>patch(index,{...step,elseSteps:children})}/></div>
        </>:null}
        {step.type==='loop'?<>
          <div className="fish-form-grid two"><label className="fish-field"><span>最大循环次数</span><input className="fish-input" type="number" min={1} max={100} value={step.maxIterations||3} onChange={e=>patch(index,{...step,maxIterations:Number(e.target.value)||1})}/></label><label className="fish-field"><span>可选条件变量</span><input className="fish-input" value={step.condition?.variable||''} onChange={e=>patch(index,{...step,condition:e.target.value?{variable:e.target.value,operator:'truthy'}:undefined})}/></label></div>
          <div className="fish-workflow-branch"><strong>循环步骤</strong><StepList depth={depth+1} steps={step.steps||[]} onChange={children=>patch(index,{...step,steps:children})}/></div>
        </>:null}
        <label className="fish-workflow-breakpoint"><input type="checkbox" checked={!!step.breakpoint} onChange={e=>patch(index,{...step,breakpoint:e.target.checked})}/> 在此步骤前暂停</label>
      </div>
    </div>)}
    <button className="fish-btn" onClick={()=>onChange([...steps,createStep()])}><Plus size={15}/>添加步骤</button>
  </div>
}

export function FishWorkflowsPage(){
  const [items,setItems]=useState<BrowserWorkflow[]>([])
  const [runs,setRuns]=useState<BrowserWorkflowRun[]>([])
  const [selectedId,setSelectedId]=useState('')
  const [draft,setDraft]=useState<BrowserWorkflow|null>(null)
  const [busy,setBusy]=useState(false)
  const load=useCallback(async()=>{try{const [w,r]=await Promise.all([listBrowserWorkflows(),listBrowserWorkflowRuns(100)]);setItems(w||[]);setRuns(r||[]);if(!selectedId&&w?.length)setSelectedId(w[0].id)}catch(e){toast.error(e instanceof Error?e.message:'工作流加载失败')}},[selectedId])
  useEffect(()=>{void load()},[])
  useEffect(()=>{const item=items.find(v=>v.id===selectedId);if(item)setDraft(JSON.parse(JSON.stringify(item)))},[items,selectedId])
  const relatedRuns=useMemo(()=>runs.filter(r=>!draft||r.workflowId===draft.id).slice(0,8),[runs,draft])
  const save=async()=>{if(!draft)return;setBusy(true);try{const saved=await saveBrowserWorkflow(draft);setDraft(saved);setSelectedId(saved.id);toast.success('工作流已保存');await load()}catch(e){toast.error(e instanceof Error?e.message:'保存失败')}finally{setBusy(false)}}
  const run=async()=>{if(!draft?.id)return;setBusy(true);try{const result=await runBrowserWorkflow(draft.id);if(result.status==='success')toast.success('工作流执行完成');else toast.warning(`工作流状态：${result.status}`);await load()}catch(e){toast.error(e instanceof Error?e.message:'执行失败');await load()}finally{setBusy(false)}}
  const resume=async(r:BrowserWorkflowRun)=>{setBusy(true);try{await resumeBrowserWorkflow(r.id,r.resumeRequiresForce===true);toast.success('已提交恢复执行');await load()}catch(e){toast.error(e instanceof Error?e.message:'恢复失败')}finally{setBusy(false)}}
  const remove=async()=>{if(!draft?.id)return;setBusy(true);try{await deleteBrowserWorkflow(draft.id);setSelectedId('');setDraft(null);toast.success('工作流已删除');await load()}catch(e){toast.error(e instanceof Error?e.message:'删除失败')}finally{setBusy(false)}}
  const create=()=>{const w=newWorkflow();setDraft(w);setSelectedId('')}
  return <div className="fish-content fish-module-content">
    <section className="fish-module-hero"><div><span className="fish-eyebrow">RPA / Workflow</span><h1>可恢复工作流</h1><p>条件、循环、变量、断点、重试与调度统一复用现有 PageSession。</p></div><button className="fish-btn primary" onClick={create}><Plus size={16}/>新建工作流</button></section>
    <section className="fish-module-strip"><div><span>工作流</span><strong>{items.length}</strong></div><div><span>已启用</span><strong>{items.filter(v=>v.enabled).length}</strong></div><div><span>最近运行</span><strong>{runs.length}</strong></div><div><span>需恢复</span><strong>{runs.filter(v=>['paused','interrupted','failed'].includes(v.status)).length}</strong></div></section>
    <div className="fish-workflow-layout">
      <aside className="fish-workflow-list">
        <button className="fish-btn" onClick={()=>void load()}><RefreshCw size={15}/>刷新</button>
        {items.map(item=><button key={item.id} className={`fish-workflow-list-item${selectedId===item.id?' active':''}`} onClick={()=>setSelectedId(item.id)}><span><strong>{item.name}</strong><small>{item.schedule?.enabled?'已启用调度':'手动运行'}</small></span><span className={`fish-status-pill${item.enabled?' good':''}`}>{item.enabled?'启用':'停用'}</span></button>)}
        {!items.length?<div className="fish-empty compact"><strong>暂无工作流</strong><span>创建第一个可视化流程。</span></div>:null}
      </aside>
      <main className="fish-workflow-editor">
        {!draft?<div className="fish-empty"><strong>选择或新建工作流</strong><span>在左侧选择定义后编辑。</span></div>:<>
          <div className="fish-module-section-head"><div><strong>{draft.id?draft.name:'新工作流'}</strong><span>{draft.id||'保存后生成 ID'}</span></div><div className="fish-inline-actions">{draft.id?<button className="fish-btn" disabled={busy} onClick={()=>void run()}><Play size={15}/>运行</button>:null}<button className="fish-btn primary" disabled={busy} onClick={()=>void save()}><Save size={15}/>保存</button>{draft.id?<button className="fish-btn danger" disabled={busy} onClick={()=>void remove()}><Trash2 size={15}/>删除</button>:null}</div></div>
          <div className="fish-form-grid two">
            <label className="fish-field"><span>名称</span><input className="fish-input" value={draft.name} onChange={e=>setDraft({...draft,name:e.target.value})}/></label>
            <label className="fish-field"><span>Profile ID（可选）</span><input className="fish-input" value={String((draft.selector as any)?.profileId||'')} onChange={e=>setDraft({...draft,selector:{...draft.selector,profileId:e.target.value}})}/></label>
          </div>
          <label className="fish-field"><span>说明</span><textarea className="fish-textarea" rows={2} value={draft.description||''} onChange={e=>setDraft({...draft,description:e.target.value})}/></label>
          <div className="fish-form-grid three">
            <label className="fish-setting-row compact"><div><strong>启用</strong><span>允许手动与计划执行</span></div><input type="checkbox" checked={draft.enabled} onChange={e=>setDraft({...draft,enabled:e.target.checked})}/></label>
            <label className="fish-setting-row compact"><div><strong>调度</strong><span>按间隔或每日时间</span></div><input type="checkbox" checked={!!draft.schedule?.enabled} onChange={e=>setDraft({...draft,schedule:{...draft.schedule,enabled:e.target.checked}})}/></label>
            <label className="fish-setting-row compact"><div><strong>允许重叠</strong><span>默认避免同一流程并发</span></div><input type="checkbox" checked={!!draft.schedule?.allowOverlap} onChange={e=>setDraft({...draft,schedule:{...draft.schedule,allowOverlap:e.target.checked}})}/></label>
          </div>
          {draft.schedule?.enabled?<div className="fish-form-grid two"><label className="fish-field"><span>每 N 分钟</span><input className="fish-input" type="number" min={1} value={draft.schedule.everyMinutes||60} onChange={e=>setDraft({...draft,schedule:{...draft.schedule,everyMinutes:Number(e.target.value)||60,dailyTime:''}})}/></label><label className="fish-field"><span>或每日时间</span><input className="fish-input" type="time" value={draft.schedule.dailyTime||''} onChange={e=>setDraft({...draft,schedule:{...draft.schedule,dailyTime:e.target.value,everyMinutes:e.target.value?0:draft.schedule.everyMinutes}})}/></label></div>:null}
          <div className="fish-workflow-section-title"><strong>流程步骤</strong><span>{draft.steps.length} 个顶层步骤</span></div>
          <StepList steps={draft.steps} onChange={steps=>setDraft({...draft,steps})}/>
          {relatedRuns.length?<section className="fish-workflow-history"><div className="fish-workflow-section-title"><strong>最近运行</strong><span>状态与恢复点</span></div>{relatedRuns.map(r=><div className="fish-flat-row" key={r.id}><span className="fish-row-mark">{r.status==='paused'?<Pause size={15}/>:<Play size={15}/>}</span><div className="fish-flat-row-main"><strong>{r.status}</strong><small>{r.id} · nextStep {r.nextStep}{r.error?` · ${r.error}`:''}</small></div>{['paused','interrupted','failed'].includes(r.status)?<button className="fish-btn" disabled={busy} onClick={()=>void resume(r)}>恢复{r.resumeRequiresForce?'（确认副作用）':''}</button>:null}<button className="fish-mini-icon" title="复制运行 ID" onClick={()=>void navigator.clipboard.writeText(r.id)}><Copy size={14}/></button></div>)}</section>:null}
        </>}
      </main>
    </div>
  </div>
}
