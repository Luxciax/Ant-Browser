import type { BrowserWorkflow, BrowserWorkflowRun } from '../types'
import { getBindings } from './runtime'

async function bindings(): Promise<any> {
  const value: any = await getBindings()
  if (!value) throw new Error('Wails runtime binding unavailable')
  return value
}
export async function listBrowserWorkflows(): Promise<BrowserWorkflow[]> { const b=await bindings(); return await b.BrowserAutomationWorkflowList() }
export async function getBrowserWorkflow(id:string): Promise<BrowserWorkflow> { const b=await bindings(); return await b.BrowserAutomationWorkflowGet(id) }
export async function saveBrowserWorkflow(input:BrowserWorkflow): Promise<BrowserWorkflow> { const b=await bindings(); return await b.BrowserAutomationWorkflowSave(input) }
export async function deleteBrowserWorkflow(id:string): Promise<void> { const b=await bindings(); await b.BrowserAutomationWorkflowDelete(id) }
export async function runBrowserWorkflow(workflowId:string,variables:Record<string,any>={},breakpoints:string[]=[]): Promise<BrowserWorkflowRun> {
  const b=await bindings(); return await b.BrowserAutomationWorkflowRun({workflowId,variables,breakpoints})
}
export async function resumeBrowserWorkflow(runId:string,force=false): Promise<BrowserWorkflowRun> { const b=await bindings(); return await b.BrowserAutomationWorkflowResume(runId,force) }
export async function listBrowserWorkflowRuns(limit=50): Promise<BrowserWorkflowRun[]> { const b=await bindings(); return await b.BrowserAutomationWorkflowRunList(limit) }
export async function stageBrowserWorkflowUpload(sourcePath:string,filename=''): Promise<{name:string;size:number}> { const b=await bindings(); return await b.BrowserAutomationWorkflowStageUpload(sourcePath,filename) }
