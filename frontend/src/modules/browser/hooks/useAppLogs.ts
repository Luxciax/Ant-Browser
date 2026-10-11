import { useCallback, useEffect, useRef, useState } from 'react'
import { ClearAppLogs, ExportBrowserDiagnostics, GetAppLogs, GetAppLogStatus } from '../../../wailsjs/go/main/App'

export interface LogEntry {
  time: string
  level: string
  component: string
  message: string
  fields?: Record<string, unknown>
}

export function useAppLogs() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [exporting, setExporting] = useState(false)
  const [error, setError] = useState('')
  const [status, setStatus] = useState({ path: '', error: '' })
  const [exportMessage, setExportMessage] = useState('')
  const busy = useRef(false)
  const mounted = useRef(false)
  const load = useCallback(async () => {
    if (busy.current) return
    busy.current = true
    setLoading(true)
    try {
      const [entries, persistence] = await Promise.all([GetAppLogs(), GetAppLogStatus()])
      if (mounted.current) {
        setLogs(entries || [])
        setStatus({ path: persistence.path || '', error: persistence.error || '' })
        setError('')
      }
    } catch (err) {
      if (mounted.current) setError(`读取日志失败：${String(err)}。已保留上次成功读取的日志。`)
    } finally {
      busy.current = false
      if (mounted.current) setLoading(false)
    }
  }, [])
  useEffect(() => {
    mounted.current = true
    void load()
    const timer = window.setInterval(() => void load(), 3000)
    return () => { mounted.current = false; window.clearInterval(timer) }
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
      const result = await ExportBrowserDiagnostics()
      if (!result.cancelled) setExportMessage(`诊断包已保存至：${String(result.path)}（仅本地保存，未上传）`)
    } catch (err) { setExportMessage(`诊断包导出失败：${String(err)}`) }
    finally { setExporting(false) }
  }
  return { logs, loading, load, clear, error, status, exporting, exportMessage, exportDiagnostics }
}
