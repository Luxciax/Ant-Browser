import { useEffect, useState } from 'react'
import { Button, Modal } from '../../../shared/components'
import type { BrowserProfilePackageExportOptions } from '../types'

interface ProfilePackageExportModalProps {
  open: boolean
  profileCount: number
  fullBackup?: boolean
  busy?: boolean
  onClose: () => void
  onConfirm: (options: BrowserProfilePackageExportOptions) => void
}

export function ProfilePackageExportModal({
  open,
  profileCount,
  fullBackup = false,
  busy = false,
  onClose,
  onConfirm,
}: ProfilePackageExportModalProps) {
  const [portableLogin, setPortableLogin] = useState(false)
  const [migrationPassword, setMigrationPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    if (!open) {
      setPortableLogin(false)
      setMigrationPassword('')
      setConfirmPassword('')
      setError('')
    }
  }, [open])

  const confirm = () => {
    if (busy) return
    if (portableLogin) {
      if (migrationPassword.trim().length < 8) {
        setError('迁移密码至少需要 8 个字符')
        return
      }
      if (migrationPassword !== confirmPassword) {
        setError('两次输入的迁移密码不一致')
        return
      }
    }
    onConfirm({
      portableLogin,
      protectArchive: fullBackup && portableLogin,
      migrationPassword: portableLogin ? migrationPassword : '',
    })
  }

  return (
    <Modal
      open={open}
      onClose={busy ? () => undefined : onClose}
      title={fullBackup ? '全量备份与登录态' : '导出实例'}
      width="560px"
      footer={(
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>取消</Button>
          <Button onClick={confirm} loading={busy}>导出</Button>
        </>
      )}
    >
      <div className="space-y-4 text-sm">
        <div className="text-[var(--color-text-secondary)]">
          {fullBackup ? '将备份应用配置、全部实例及完整用户数据。' : `将导出 ${profileCount} 个实例的配置和完整用户数据。`}
        </div>
        <label className="flex cursor-pointer items-start gap-3 rounded-lg border border-[var(--color-border-default)] bg-[var(--color-bg-muted)] p-3">
          <input
            type="checkbox"
            checked={portableLogin}
            onChange={(event) => {
              setPortableLogin(event.target.checked)
              setError('')
            }}
            disabled={busy}
            className="mt-0.5"
          />
          <span>
            <strong className="block font-medium text-[var(--color-text-primary)]">可迁移登录态（跨电脑）</strong>
            <span className="mt-1 block text-xs leading-5 text-[var(--color-text-secondary)]">
              Windows 支持密码加密的 Cookie 快照迁移，包括原浏览器可读取的 App-Bound Cookie；其他系统暂不支持。请先在新版中打开实例，再从实例列表点击“停止”，生成最新快照后导出。网站存储随目录备份；已保存密码、通行密钥不保证跨设备可用。
            </span>
          </span>
        </label>

        {portableLogin && (
          <div className="space-y-3 rounded-lg border border-[var(--color-border-default)] p-3">
            <div className="text-xs leading-5 text-[var(--color-warning)]">
              换电脑或重装系统后恢复，必须输入同一个迁移密码。本次密码不保存。网站过期或设备绑定仍可能要求重新登录。{fullBackup ? '此全量备份将整包加密，包括网站存储、Service Workers 和扩展数据。' : '实例包只加密登录态载荷，其他文件未整体加密；敏感数据请使用全量加密备份。'}
            </div>
            <label className="block">
              <span className="mb-1 block text-xs text-[var(--color-text-secondary)]">迁移密码</span>
              <input
                type="password"
                value={migrationPassword}
                onChange={(event) => {
                  setMigrationPassword(event.target.value)
                  setError('')
                }}
                disabled={busy}
                autoComplete="new-password"
                className="w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-3 py-2 text-sm text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]"
              />
            </label>
            <label className="block">
              <span className="mb-1 block text-xs text-[var(--color-text-secondary)]">确认迁移密码</span>
              <input
                type="password"
                value={confirmPassword}
                onChange={(event) => {
                  setConfirmPassword(event.target.value)
                  setError('')
                }}
                disabled={busy}
                autoComplete="new-password"
                className="w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-3 py-2 text-sm text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]"
              />
            </label>
          </div>
        )}

        {error && <div className="text-xs text-[var(--color-error)]">{error}</div>}
      </div>
    </Modal>
  )
}
