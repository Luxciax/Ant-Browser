import { useCallback, useEffect, useRef, useState } from 'react'
import { ClearAppLogs, ExportBrowserDiagnostics, ExportBrowserDiagnosticIncident, GetBrowserDiagnosticIncident, GetBrowserDiagnosticIncidents, GetBrowserDiagnosticRunLogs, GetAppLogs, GetAppLogStatus } from '../../../wailsjs/go/main/App'

export interface LogEntry {
  time: string
  level: string
  component: string
  message: string
  fields?: Record<string, unknown>
}

export interface DiagnosticIncident { incident_id: string; profile_id: string; run_id: string; event: string; occurred_at: string; status: string }
export function useAppLogs(profile = '', run = '', incidentID = '') {
  const [incidents, setIncidents] = useState<DiagnosticIncident[]>([])
  const [incident, setIncident] = useState<Record<string, any> | null>(null)
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [error, setError] = useState('')
  const [status, setStatus] = useState<Record<string, string>>({ path: '', error: '' })
  const [exportMessage, setExportMessage] = useState('')
  const revision = useRef(0)
  const mounted = useRef(false)
  const load = useCallback(async () => {
    const current = ++revision.current
    setLoading(true)
    try {
      const [entries, persistence, faults, detail] = await Promise.all([
        profile && run && !incidentID ? GetBrowserDiagnosticRunLogs(profile, run) : GetAppLogs(),
        GetAppLogStatus(), GetBrowserDiagnosticIncidents(), incidentID ? GetBrowserDiagnosticIncident(incidentID) : Promise.resolve(null)
      ])
      if (mounted.current && revision.current === current) {
        setLogs(detail?.timeline || entries || [])
        setIncident(detail)
        setIncidents((faults || []) as DiagnosticIncident[])
        setStatus(persistence)
        setError('')
      }
    } catch (err) {
      if (mounted.current && revision.current === current) setError(`读取日志失败：${String(err)}。已保留上次成功读取的日志。`)
    } finally {
      if (mounted.current && revision.current === current) setLoading(false)
    }
  }, [profile, run, incidentID])
  useEffect(() => {
    mounted.current = true
    void load()
    const timer = window.setInterval(() => void load(), 3000)
    return () => { mounted.current = false; revision.current++; window.clearInterval(timer) }
  }, [load])
  const clear = async () => {
    try {
      await ClearAppLogs()
      setLogs([])
      setError('')
    } catch (err) { setError(`清空内存日志失败：${String(err)}`) }
  }
  const exportDiagnostics = async () => {
    setExporting(true)
    setExportMessage('')
    try {
      const result = incidentID ? await ExportBrowserDiagnosticIncident(incidentID) : await ExportBrowserDiagnostics()
      if (!result.cancelled) setExportMessage(`诊断包已保存至：${String(result.path)}（仅本地保存，未上传）`)
    } catch (err) { setExportMessage(`诊断包导出失败：${String(err)}`) }
    finally { setExporting(false) }
  }
  return { logs, incidents, incident, loading, load, clear, error, status, exporting, exportMessage, exportDiagnostics }
}
