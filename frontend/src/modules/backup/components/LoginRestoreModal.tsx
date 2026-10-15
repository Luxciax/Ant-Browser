import { useEffect, useState } from 'react'
import { Button, Modal, toast } from '../../../shared/components'
import { restoreBackupWithLoginPassword, type BackupActionResult } from '../api'

interface Props {
  request: BackupActionResult | null
  onClose: () => void
  onRestored: () => void
}

export function LoginRestoreModal({ request, onClose, onRestored }: Props) {
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  useEffect(() => { setPassword(''); setError('') }, [request])

  const restore = async () => {
    if (!request?.zipPath || busy) return
    if (password.trim().length < 8) { setError('请输入导出时的迁移密码（至少 8 个字符）'); return }
    setBusy(true)
    setError('')
    try {
      const result = await restoreBackupWithLoginPassword(request.zipPath, password)
      if (result.partial) toast.warning('数据已恢复，但部分模块失败，请查看导入日志')
      else toast.success('备份已恢复；首次启动实例时写入并核验 Cookie，网站可能仍要求验证')
      setPassword('')
      onRestored()
      onClose()
    } catch (failure) {
      setError(failure instanceof Error ? failure.message : String(failure))
    } finally { setBusy(false) }
  }

  return <Modal open={Boolean(request)} title="恢复加密登录态" width="560px" onClose={busy ? () => undefined : onClose} footer={<>
    <Button variant="secondary" disabled={busy} onClick={onClose}>取消</Button>
    <Button loading={busy} onClick={() => void restore()}>解密并恢复备份</Button>
  </>}>
    <div className="space-y-3 text-sm">
      <p>{request?.encryptedArchive ? '此全量备份已整包加密，包含网站存储与登录态。' : `此全量备份包含 ${request?.portableLoginCount || 0} 个实例的加密登录态。`}密码和完整性验证前不会修改本机数据。</p>
      <p className="text-[var(--color-text-secondary)]">恢复会停止运行中的实例并合并备份数据。密码不会保存；错误密码可重试。网站撤销的会话、硬件绑定凭据仍需重新登录。</p>
      <label className="block">
        <span className="mb-1 block">迁移密码</span>
        <input type="password" autoComplete="off" value={password} disabled={busy} onChange={event => { setPassword(event.target.value); setError('') }}
          className="w-full rounded-md border border-[var(--color-border-default)] bg-[var(--color-bg-elevated)] px-3 py-2 text-sm text-[var(--color-text-primary)] outline-none focus:border-[var(--color-primary)]" />
      </label>
      {error && <p role="alert" className="text-[var(--color-error)]">{error}</p>}
    </div>
  </Modal>
}
