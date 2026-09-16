import { useEffect, useState } from 'react'
import { Button, Modal } from '../../../shared/components'
import type { BrowserProfilePackageExportOptions } from '../types'

interface ProfilePackageExportModalProps {
  open: boolean
  profileCount: number
  busy?: boolean
  onClose: () => void
  onConfirm: (options: BrowserProfilePackageExportOptions) => void
}

export function ProfilePackageExportModal({
  open,
  profileCount,
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
      if (migrationPassword.length < 8) {
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
      migrationPassword: portableLogin ? migrationPassword : '',
    })
  }

  return (
    <Modal
      open={open}
      onClose={busy ? () => undefined : onClose}
      title="导出实例"
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
          将导出 {profileCount} 个实例的配置和完整用户数据。
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
              额外导出密码保护的浏览器加密主密钥，使 Cookie 和已保存登录信息可在另一台 Windows 电脑恢复。不会把 Cookie 或密码明文写入实例包。
            </span>
          </span>
        </label>

        {portableLogin && (
          <div className="space-y-3 rounded-lg border border-[var(--color-border-default)] p-3">
            <div className="text-xs leading-5 text-[var(--color-warning)]">
              目标电脑导入时必须输入同一个迁移密码。Ant Browser 不会保存这个密码，忘记后无法恢复跨电脑登录态。
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
