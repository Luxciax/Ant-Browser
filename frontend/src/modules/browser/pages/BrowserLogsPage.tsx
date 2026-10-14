import { useEffect, useRef, useState } from 'react'
import { Download, RefreshCw, Trash2 } from 'lucide-react'
import { Badge, Button, Card } from '../../../shared/components'

import { useAppLogs } from '../hooks/useAppLogs'

const LEVELS = ['ALL', 'DEBUG', 'INFO', 'WARN', 'ERROR']

const levelVariant = (level: string) => {
  switch (level) {
    case 'ERROR': return 'error'
    case 'WARN': return 'warning'
    case 'DEBUG': return 'default'
    default: return 'info'
  }
}

const levelColor = (level: string) => {
  switch (level) {
    case 'ERROR': return 'text-red-500'
    case 'WARN': return 'text-yellow-500'
    case 'DEBUG': return 'text-[var(--color-text-muted)]'
    default: return 'text-[var(--color-text-secondary)]'
  }
}

export function BrowserLogsPage() {
  const { logs, loading, load, clear: handleClear, error, status, exporting, exportMessage, exportDiagnostics } = useAppLogs()
  const [levelFilter, setLevelFilter] = useState('ALL')
  const [componentFilter, setComponentFilter] = useState('ALL')
  const [methodFilter, setMethodFilter] = useState('ALL')
  const [keyword, setKeyword] = useState('')
  const [fieldKeyword, setFieldKeyword] = useState('')
  const [quickFilter, setQuickFilter] = useState('ALL')
  const [durationMin, setDurationMin] = useState('')
  const [timeFrom, setTimeFrom] = useState('')
  const [timeTo, setTimeTo] = useState('')
  const [autoScroll, setAutoScroll] = useState(false)
  const logContainerRef = useRef<HTMLDivElement>(null)
  const initialScrollDoneRef = useRef(false)

  const scrollLogsToBottom = (behavior: ScrollBehavior = 'auto') => {
    requestAnimationFrame(() => {
      const container = logContainerRef.current
      if (!container) return
      container.scrollTo({ top: container.scrollHeight, behavior })
    })
  }

  useEffect(() => {
    if (autoScroll) {
      scrollLogsToBottom('smooth')
    }
  }, [logs, autoScroll])

  const filtered = logs.filter(entry => {
    if (levelFilter !== 'ALL' && entry.level !== levelFilter) return false
    if (componentFilter !== 'ALL' && entry.component !== componentFilter) return false
    const method = String(entry.fields?.method || '')
    const duration = Number(entry.fields?.duration_ms || entry.fields?.durationMs || 0)
    if (methodFilter !== 'ALL' && method !== methodFilter) return false
    if (quickFilter === 'ERRORS' && entry.level !== 'ERROR') return false
    if (quickFilter === 'SLOW' && duration < 1000) return false
    if (quickFilter === 'FRONTEND' && entry.component !== 'Frontend') return false
    if (quickFilter === 'BACKEND' && entry.component === 'Frontend') return false
    if (durationMin && duration < Number(durationMin)) return false
    if (timeFrom && entry.time < timeFrom.replace('T', ' ')) return false
    if (timeTo && entry.time > timeTo.replace('T', ' ')) return false
    const fieldText = entry.fields ? JSON.stringify(entry.fields).toLowerCase() : ''
    const q = keyword.trim().toLowerCase()
    if (q && !entry.message.toLowerCase().includes(q) &&
        !entry.component.toLowerCase().includes(q) &&
        !method.toLowerCase().includes(q) &&
        !fieldText.includes(q)) return false
    const fq = fieldKeyword.trim().toLowerCase()
    if (fq && !fieldText.includes(fq)) return false
    return true
  })
  useEffect(() => {
    if (initialScrollDoneRef.current || loading || logs.length === 0) return
    initialScrollDoneRef.current = true
    scrollLogsToBottom('auto')
  }, [filtered.length, loading, logs.length])

  const components = Array.from(new Set(logs.map(entry => entry.component).filter(Boolean))).sort()
  const methods = Array.from(new Set(logs.map(entry => String(entry.fields?.method || '')).filter(Boolean))).sort()
  const resetFilters = () => {
    setLevelFilter('ALL')
    setComponentFilter('ALL')
    setMethodFilter('ALL')
    setQuickFilter('ALL')
    setKeyword('')
    setFieldKeyword('')
    setDurationMin('')
    setTimeFrom('')
    setTimeTo('')
  }

  return (
    <div className="space-y-4 animate-fade-in">
      <Card padding="none" className="shadow-[var(--shadow-sm)]">
        <div className="flex flex-col gap-3 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <h1 className="text-xl font-semibold text-[var(--color-text-primary)]">日志查看</h1>
          <div className="flex flex-wrap gap-2 sm:justify-end">
            <Button variant="secondary" size="sm" onClick={load} loading={loading}>
              <RefreshCw className="w-4 h-4" />刷新
            </Button>
            <Button variant="secondary" size="sm" onClick={exportDiagnostics} loading={exporting}><Download className="w-4 h-4" />导出诊断包</Button>
            <Button variant="secondary" size="sm" onClick={handleClear}>
              <Trash2 className="w-4 h-4" />清空
            </Button>
          </div>
        </div>
      </Card>

      {error && <p role="alert">{error}</p>}
      {status.error && <p role="alert">日志持久化不可用：{status.error}。当前内存日志仍可查看和导出。</p>}
      <p className="text-sm break-all">日志文件：{status.path || '未启用'}；清空只清除内存列表，不删除磁盘日志。{exportMessage}</p>
      <div className="rounded-xl border border-[var(--color-border-default)] bg-[var(--color-bg-surface)] p-3">
        <div className="flex flex-wrap items-center gap-2">
          {LEVELS.map(l => (
            <button
              key={l}
              onClick={() => setLevelFilter(l)}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors ${
                levelFilter === l
                  ? 'bg-[var(--color-accent)] text-white'
                  : 'bg-[var(--color-bg-muted)] text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
              }`}
            >
              {l}
            </button>
          ))}

          {[
            ['ALL', '全部'],
            ['ERRORS', '只看异常'],
            ['SLOW', '慢调用'],
            ['FRONTEND', '前端操作'],
            ['BACKEND', '后端组件'],
          ].map(([value, label]) => (
            <button
              key={value}
              onClick={() => { setQuickFilter(value); if (value === 'ERRORS') setLevelFilter('ALL') }}
              className={`px-2.5 py-1 text-xs rounded-md transition-colors ${
                quickFilter === value
                  ? 'bg-[var(--color-text-primary)] text-white'
                  : 'bg-[var(--color-bg-muted)] text-[var(--color-text-muted)] hover:text-[var(--color-text-primary)]'
              }`}
            >
              {label}
            </button>
          ))}

          <span className="ml-auto text-xs text-[var(--color-text-muted)]">
            {filtered.length} / {logs.length} 条
          </span>
        </div>

        <div className="mt-3 grid grid-cols-1 gap-2 lg:grid-cols-6">
          <input
            value={keyword}
            onChange={e => setKeyword(e.target.value)}
            placeholder="搜索消息 / 组件 / 方法"
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)] lg:col-span-2"
          />
          <input
            value={fieldKeyword}
            onChange={e => setFieldKeyword(e.target.value)}
            placeholder="搜索字段"
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
          />
          <select
            value={componentFilter}
            onChange={e => setComponentFilter(e.target.value)}
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
          >
            <option value="ALL">全部组件</option>
            {components.map(component => (
              <option key={component} value={component}>{component}</option>
            ))}
          </select>
          <select
            value={methodFilter}
            onChange={e => setMethodFilter(e.target.value)}
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
          >
            <option value="ALL">全部方法</option>
            {methods.map(method => (
              <option key={method} value={method}>{method}</option>
            ))}
          </select>
          <input
            type="number"
            min="0"
            value={durationMin}
            onChange={e => setDurationMin(e.target.value)}
            placeholder="最小耗时 ms"
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
          />
        </div>

        <div className="mt-2 flex flex-wrap items-center gap-2">
          <input
            type="datetime-local"
            value={timeFrom}
            onChange={e => setTimeFrom(e.target.value)}
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
          />
          <span className="text-xs text-[var(--color-text-muted)]">到</span>
          <input
            type="datetime-local"
            value={timeTo}
            onChange={e => setTimeTo(e.target.value)}
            className="px-3 py-1.5 text-sm rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-secondary)] text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]"
          />
          <label className="ml-auto flex items-center gap-1.5 text-xs text-[var(--color-text-muted)] cursor-pointer select-none">
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={e => setAutoScroll(e.target.checked)}
              className="w-3.5 h-3.5"
            />
            自动滚动
          </label>
          <Button variant="secondary" size="sm" onClick={resetFilters}>重置筛选</Button>
        </div>
      </div>

      {/* 日志列表 */}
      <Card padding="none">
        <div
          ref={logContainerRef}
          className="overflow-auto font-mono text-xs"
          style={{ maxHeight: 'calc(100vh - 280px)' }}
        >
          {filtered.length === 0 ? (
            <div className="py-10 text-center text-sm text-[var(--color-text-muted)]">暂无日志</div>
          ) : (
            <table className="min-w-full">
              <thead className="sticky top-0 z-10 bg-[var(--color-bg-muted)]">
                <tr>
                  <th className="px-3 py-2 text-left text-[var(--color-text-muted)] font-semibold w-40">时间</th>
                  <th className="px-3 py-2 text-left text-[var(--color-text-muted)] font-semibold w-16">级别</th>
                  <th className="px-3 py-2 text-left text-[var(--color-text-muted)] font-semibold w-28">组件</th>
                  <th className="px-3 py-2 text-left text-[var(--color-text-muted)] font-semibold">消息</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border-muted)]">
                {filtered.map((entry, i) => (
                  <tr key={i} className="hover:bg-[var(--color-bg-muted)]/40">
                    <td className="px-3 py-1.5 text-[var(--color-text-muted)] whitespace-nowrap">{entry.time}</td>
                    <td className="px-3 py-1.5">
                      <Badge variant={levelVariant(entry.level)} className="text-[10px]">{entry.level}</Badge>
                    </td>
                    <td className="px-3 py-1.5 text-[var(--color-text-muted)] truncate max-w-[112px]" title={entry.component}>
                      {entry.component}
                    </td>
                    <td className={`px-3 py-1.5 ${levelColor(entry.level)}`}>
                      <span>{entry.message}</span>
                      {entry.fields && Object.keys(entry.fields).length > 0 && (
                        <span className="ml-2 text-[var(--color-text-muted)]">
                          {Object.entries(entry.fields).map(([k, v]) => `${k}=${JSON.stringify(v)}`).join(' ')}
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </Card>
    </div>
  )
}
