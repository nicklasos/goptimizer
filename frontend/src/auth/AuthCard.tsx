import type { ReactNode } from 'react'
import { Card, Grid, Typography, theme } from 'antd'
import { useTranslation } from 'react-i18next'

interface Props {
  subtitle: ReactNode
  children: ReactNode
  testId?: string
}

/** The centred card every signed-out page sits in. */
export function AuthCard({ subtitle, children, testId }: Props) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const isMobile = !Grid.useBreakpoint().md

  return (
    <div
      data-testid={testId}
      style={{
        minHeight: '100vh',
        background: token.colorBgLayout,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: token.paddingLG,
      }}
    >
      <Card style={{ width: '100%', maxWidth: isMobile ? 360 : 400 }}>
        <div style={{ textAlign: 'center', marginBottom: token.marginLG }}>
          <img src="/favicon.svg" alt="" width={48} height={48} style={{ marginBottom: token.marginSM }} />
          <Typography.Title level={isMobile ? 3 : 2} style={{ marginBottom: token.marginXS }}>
            {t('common.appName')}
          </Typography.Title>
          <Typography.Text type="secondary">{subtitle}</Typography.Text>
        </div>
        {children}
      </Card>
    </div>
  )
}
