import type { ReactNode } from 'react'
import { Button, Grid, Space, Typography, theme } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { StickyActionBar } from './StickyActionBar'

export interface PageBack {
  label: ReactNode
  onClick: () => void
  testId: string
}

interface Props {
  title: ReactNode
  icon?: ReactNode
  /** Small items next to the title: state tags, version picker. */
  tags?: ReactNode
  /** One line under the title: counts, source of the numbers, dates. */
  subtitle?: ReactNode
  /** Secondary page actions. They wrap under the title on narrow screens. */
  extra?: ReactNode
  /** The page's primary action: last in the header on desktop, a sticky full-width bar at the bottom on phones. */
  mainAction?: ReactNode
  back?: PageBack
  titleTestId?: string
}

/** The top of every page: title, optional back link, secondary actions and one primary action. */
export function PageHeader({ title, icon, tags, subtitle, extra, mainAction, back, titleTestId }: Props) {
  const { token } = theme.useToken()
  const screens = Grid.useBreakpoint()
  const isMobile = !screens.md

  return (
    <div style={{ marginBottom: token.marginLG }}>
      {back && (
        <Button
          data-testid={back.testId}
          type="link"
          size="small"
          icon={<ArrowLeftOutlined />}
          onClick={back.onClick}
          style={{ paddingInline: 0, marginBottom: token.marginSM }}
        >
          {back.label}
        </Button>
      )}
      <div
        style={{
          display: 'flex',
          flexDirection: isMobile ? 'column' : 'row',
          alignItems: isMobile ? 'stretch' : 'flex-start',
          justifyContent: 'space-between',
          gap: token.marginMD,
        }}
      >
        <div style={{ minWidth: 0 }}>
          <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: token.marginSM }}>
            <Typography.Title
              level={isMobile ? 4 : 3}
              data-testid={titleTestId}
              style={{ margin: 0, display: 'flex', alignItems: 'center', gap: token.marginXS }}
            >
              {icon}
              {title}
            </Typography.Title>
            {tags}
          </div>
          {subtitle && (
            <Typography.Text type="secondary" style={{ display: 'block', marginTop: token.marginXS }}>
              {subtitle}
            </Typography.Text>
          )}
        </div>
        {(extra || (mainAction && !isMobile)) && (
          <Space wrap size={token.marginSM} style={{ flexShrink: 0 }}>
            {extra}
            {!isMobile && mainAction}
          </Space>
        )}
      </div>
      {isMobile && mainAction && <StickyActionBar>{mainAction}</StickyActionBar>}
    </div>
  )
}
