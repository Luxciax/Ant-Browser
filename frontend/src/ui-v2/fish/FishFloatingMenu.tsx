import { useLayoutEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'

interface Props {
  anchor: HTMLButtonElement
  onClose: () => void
  children: ReactNode
}

// Keep the theme ancestor, but escape the scrollable/clipping row container.
export function FishFloatingMenu({ anchor, onClose, children }: Props) {
  const menuRef = useRef<HTMLDivElement>(null)
  const [position, setPosition] = useState<{ left: number; top: number; maxHeight: number }>()
  const positioned = Boolean(position)

  useLayoutEffect(() => {
    if (positioned) menuRef.current?.querySelector<HTMLButtonElement>('button:not(:disabled)')?.focus({ preventScroll: true })
  }, [anchor, positioned])

  useLayoutEffect(() => {
    const menu = menuRef.current
    if (!menu) return
    let frame = 0
    const place = () => {
      if (!anchor.isConnected) { onClose(); return }
      const rect = anchor.getBoundingClientRect()
      const width = window.innerWidth, height = window.innerHeight
      if (rect.bottom <= 0 || rect.top >= height || rect.right <= 0 || rect.left >= width) { onClose(); return }
      const margin = 8, gap = 6
      const below = Math.max(0, height - rect.bottom - gap - margin)
      const above = Math.max(0, rect.top - gap - margin)
      const naturalHeight = menu.scrollHeight + 2
      const upwards = naturalHeight > below && above > below
      const maxHeight = Math.min(height - margin * 2, upwards ? above : below)
      const renderedHeight = Math.min(naturalHeight, maxHeight)
      const left = Math.max(margin, Math.min(rect.right - menu.offsetWidth, width - menu.offsetWidth - margin))
      const top = Math.max(margin, Math.min(upwards ? rect.top - gap - renderedHeight : rect.bottom + gap, height - renderedHeight - margin))
      setPosition(previous => previous?.left === left && previous.top === top && previous.maxHeight === maxHeight ? previous : { left, top, maxHeight })
    }
    const schedule = (event?: Event) => {
      if (event?.target instanceof Node && menu.contains(event.target)) return
      if (!frame) frame = requestAnimationFrame(() => { frame = 0; place() })
    }
    const outside = (event: PointerEvent) => {
      if (event.target instanceof Node && !menu.contains(event.target) && !anchor.contains(event.target)) onClose()
    }
    const escape = (event: globalThis.KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
        anchor.focus({ preventScroll: true })
      }
    }
    place()
    const observer = new ResizeObserver(() => schedule())
    observer.observe(anchor)
    observer.observe(menu)
    window.addEventListener('resize', schedule)
    window.addEventListener('scroll', schedule, true)
    document.addEventListener('pointerdown', outside)
    document.addEventListener('keydown', escape)
    return () => {
      cancelAnimationFrame(frame)
      observer.disconnect()
      window.removeEventListener('resize', schedule)
      window.removeEventListener('scroll', schedule, true)
      document.removeEventListener('pointerdown', outside)
      document.removeEventListener('keydown', escape)
    }
  }, [anchor, onClose])

  const navigate = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key === 'Tab') { onClose(); return }
    if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) return
    event.preventDefault()
    const items = Array.from(menuRef.current?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)') || [])
    if (!items.length) return
    const current = items.indexOf(document.activeElement as HTMLButtonElement)
    const next = event.key === 'Home' ? 0 : event.key === 'End' ? items.length - 1 : (current + (event.key === 'ArrowDown' ? 1 : -1) + items.length) % items.length
    items[next].focus()
  }

  return createPortal(
    <div ref={menuRef} className="fish-menu fixed" role="menu" aria-label="扩展操作" onKeyDown={navigate}
      style={{ ...position, visibility: position ? 'visible' : 'hidden', minWidth: 'min(190px, calc(100vw - 16px))', maxWidth: 'calc(100vw - 16px)', overflowY: 'auto', boxSizing: 'border-box' }}>
      {children}
    </div>,
    anchor.closest('.fish-ui') || document.body,
  )
}
