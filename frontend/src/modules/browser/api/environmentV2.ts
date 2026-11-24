import type {
  BrowserBatchOperation,
  BrowserEnvironmentTemplateV2,
  BrowserEnvironmentViewV2,
  BrowserNetworkPolicyV2,
  BrowserProfileInput,
} from '../types'
import { getBindings } from './runtime'

export async function fetchBrowserEnvironmentView(profileId: string): Promise<BrowserEnvironmentViewV2> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserProfileEnvironmentView) throw new Error('EnvironmentView v2 unavailable')
  return await bindings.BrowserProfileEnvironmentView(profileId)
}

export async function fetchBrowserEnvironmentTemplates(): Promise<BrowserEnvironmentTemplateV2[]> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserEnvironmentTemplateList) return []
  return (await bindings.BrowserEnvironmentTemplateList()) || []
}

export async function applyBrowserEnvironmentTemplate(profileId: string, templateId: string): Promise<void> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserProfileApplyEnvironmentTemplate) throw new Error('environment template API unavailable')
  await bindings.BrowserProfileApplyEnvironmentTemplate(profileId, templateId)
}

export async function fetchBrowserNetworkPolicy(profileId: string): Promise<BrowserNetworkPolicyV2> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserProfileNetworkPolicy) throw new Error('NetworkPolicy unavailable')
  return await bindings.BrowserProfileNetworkPolicy(profileId)
}

export async function saveBrowserNetworkPolicy(policy: BrowserNetworkPolicyV2): Promise<BrowserNetworkPolicyV2> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserProfileSetNetworkPolicy) throw new Error('NetworkPolicy update unavailable')
  return await bindings.BrowserProfileSetNetworkPolicy(policy)
}

export async function batchStartBrowserProfiles(profileIds: string[], concurrency = 3): Promise<BrowserBatchOperation> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserBatchStart) throw new Error('batch start unavailable')
  return await bindings.BrowserBatchStart({ profileIds, concurrency })
}

export async function batchStopBrowserProfiles(profileIds: string[], concurrency = 3): Promise<BrowserBatchOperation> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserBatchStop) throw new Error('batch stop unavailable')
  return await bindings.BrowserBatchStop({ profileIds, concurrency })
}

export async function batchCreateBrowserProfiles(items: Array<{ clientKey?: string; config: BrowserProfileInput }>, concurrency = 3): Promise<BrowserBatchOperation> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserBatchCreate) throw new Error('batch create unavailable')
  return await bindings.BrowserBatchCreate({ items, concurrency })
}

export async function fetchBrowserBatchOperation(operationId: string): Promise<BrowserBatchOperation> {
  const bindings: any = await getBindings()
  if (!bindings?.BrowserBatchOperationGet) throw new Error('batch operation unavailable')
  return await bindings.BrowserBatchOperationGet(operationId)
}
