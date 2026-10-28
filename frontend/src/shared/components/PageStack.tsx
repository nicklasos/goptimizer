import type { CSSProperties, ReactNode } from 'react'
import { theme } from 'antd'

interface Props {
  children: ReactNode
  testId?: string
  /** Gap between sections; defaults to the page rhythm (`token.margin`). */
  gap?: number
  style?: CSSProperties
}

/** Vertical stack of page sections with one spacing rhythm. Use it instead of `marginBottom` on cards. */
export function PageStack({ children, testId, gap, style }: Props) {
  const { token } = theme.useToken()
  return (
    <div data-testid={testId} style={{ display: 'flex', flexDirection: 'column', gap: gap ?? token.margin, minWidth: 0, ...style }}>
      {children}
    </div>
  )
}
