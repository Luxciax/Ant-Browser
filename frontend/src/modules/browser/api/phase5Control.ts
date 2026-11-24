import type { BrowserSyncStatusV1, BrowserSystemIntegrationStatus, BrowserTrashItem } from '../types'
import { getBindings } from './runtime'
async function b():Promise<any>{const v:any=await getBindings();if(!v)throw new Error('Wails runtime binding unavailable');return v}
export async function listBrowserTrash():Promise<BrowserTrashItem[]>{return await (await b()).BrowserProfileTrashList()}
export async function restoreBrowserTrash(profileId:string):Promise<any>{return await (await b()).BrowserProfileRestore(profileId)}
export async function permanentlyDeleteBrowserTrash(profileId:string):Promise<void>{await (await b()).BrowserProfilePermanentlyDelete(profileId)}
export async function cleanupBrowserTrash():Promise<void>{await (await b()).BrowserProfileTrashCleanup()}
export async function getBrowserSyncStatus():Promise<BrowserSyncStatusV1>{return await (await b()).BrowserSyncStatus()}
export async function getBrowserSystemIntegrationStatus():Promise<BrowserSystemIntegrationStatus>{return await (await b()).BrowserSystemIntegrationStatusGet()}
export async function setBrowserProtocolRegistration(enabled:boolean):Promise<BrowserSystemIntegrationStatus>{return await (await b()).BrowserSystemIntegrationSetProtocol(enabled)}
export async function getBrowserStartupProtocolURLs():Promise<string[]>{return await (await b()).BrowserSystemIntegrationStartupURLs()}

export interface BrowserControlDiagnostics {
  ok: boolean
  schemaVersion: number
  launchServer?: Record<string, any>
  secretStoreReady?: boolean
  batchOperationCount?: number
  runtimeEvidenceCount?: number
}
export async function getBrowserControlDiagnostics():Promise<BrowserControlDiagnostics>{return await (await b()).BrowserControlDiagnostics()}
export async function rotateBrowserLaunchAuth(scopes:string[]):Promise<{apiKey:string;header:string;scopes:string[]}>{return await (await b()).BrowserLaunchAuthRotate(scopes)}
export async function revokeBrowserLaunchAuth():Promise<void>{await (await b()).BrowserLaunchAuthRevoke()}
