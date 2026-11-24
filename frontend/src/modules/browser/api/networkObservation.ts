import type { BrowserNetworkObservation } from '../types'
import { getBindings } from './runtime'
async function b():Promise<any>{const v:any=await getBindings();if(!v)throw new Error('Wails runtime binding unavailable');return v}
export async function observeHostNetwork(url:string,profileId='',timeoutMs=15000):Promise<BrowserNetworkObservation>{
  return await (await b()).BrowserNetworkObserve({url,profileId,timeoutMs})
}
export async function getNetworkObservation(id:string):Promise<BrowserNetworkObservation>{
  return await (await b()).BrowserNetworkObservationGet(id)
}
