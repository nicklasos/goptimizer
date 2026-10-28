import { useEffect, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { theme } from 'antd'
import './StickyActionBar.css'

/** Phone-only bar pinned to the bottom of the screen with the page's main action. While it is shown, the content gets bottom padding (`has-sticky-bar`) so it never covers the page. */
export function StickyActionBar({ children }: { children: ReactNode }) {
  const { token } = theme.useToken()
  useEffect(() => {
    document.body.classList.add('has-sticky-bar')
    return () => document.body.classList.remove('has-sticky-bar')
  }, [])
  return createPortal(
    <div
      className="sticky-action-bar"
      data-testid="sticky-action-bar"
      style={{ background: token.colorBgContainer, borderTop: `1px solid ${token.colorBorderSecondary}` }}
    >
      {children}
    </div>,
    document.body
  )
}
