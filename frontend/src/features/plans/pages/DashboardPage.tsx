import { Button, Card, Col, Empty, Row, Space, Typography, theme } from 'antd'
import { DashboardOutlined, PlusOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useCurrentUser } from '@/auth/authStore'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { useNow } from '@/shared/hooks/useNow'
import { PlanCard } from '../components/PlanCard'
import { HEALTH, summaryCounts } from '../health'
import { useDashboard } from '../hooks'
import { canManagePlans } from '../policy'
import type { HealthSummary } from '../types'

/** The home page: every backup plan as a card coloured by its health. */
export default function DashboardPage() {
  const user = useCurrentUser()
  return <PageStack testId="dashboard-page">{canManagePlans(user) ? <PlansDashboard /> : <NoAccess />}</PageStack>
}

function PlansDashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const dashboard = useDashboard()
  const now = useNow()

  const add = (testId: string) => (
    <Button data-testid={testId} type="primary" icon={<PlusOutlined />} onClick={() => navigate('/plans/new')}>
      {t('plans.add')}
    </Button>
  )

  if (dashboard.isLoading) return <PageLoading />
  if (dashboard.isError || !dashboard.data) return <PageError error={dashboard.error} onRetry={() => dashboard.refetch()} />
  const { plans, summary } = dashboard.data

  return (
    <>
      <Card styles={{ body: plans.length > 0 ? { paddingBottom: 0 } : undefined }}>
        <PageHeader
          icon={<DashboardOutlined />}
          title={t('dashboard.title')}
          subtitle={plans.length > 0 ? <Summary summary={summary} /> : t('dashboard.subtitle')}
          mainAction={add('plans-add-button')}
        />
        {plans.length === 0 && (
          <div data-testid="dashboard-empty">
            <Empty description={t('dashboard.empty')} image={Empty.PRESENTED_IMAGE_SIMPLE}>
              <Typography.Paragraph type="secondary">{t('dashboard.emptyHint')}</Typography.Paragraph>
              {add('dashboard-empty-add-button')}
            </Empty>
          </div>
        )}
      </Card>
      {plans.length > 0 && (
        <Row gutter={[16, 16]} data-testid="dashboard-cards">
          {plans.map((plan) => (
            <Col key={plan.id} xs={24} md={12} xl={8}>
              <PlanCard plan={plan} now={now} />
            </Col>
          ))}
        </Row>
      )}
    </>
  )
}

/** "1 failed · 5 ok · 2 paused", each count in its health's colour. */
function Summary({ summary }: { summary: HealthSummary }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  return (
    <Space split="·" size={token.marginXS} wrap data-testid="dashboard-summary">
      {summaryCounts(summary).map(({ health, count }) => (
        <span key={health} data-testid={`dashboard-summary-${health}`} data-count={count} style={{ color: token[HEALTH[health].color] }}>
          {t(`dashboard.summary.${health}`, { count })}
        </span>
      ))}
    </Space>
  )
}

function NoAccess() {
  const { t } = useTranslation()
  const user = useCurrentUser()
  return (
    <Card>
      <PageHeader icon={<DashboardOutlined />} title={t('dashboard.title')} />
      <div data-testid="dashboard-no-access">
        <Typography.Paragraph style={{ marginBottom: 0 }}>{t('dashboard.welcome', { name: user?.name || user?.email || '' })}</Typography.Paragraph>
        <Typography.Text type="secondary">{t('dashboard.noAccess')}</Typography.Text>
      </div>
    </Card>
  )
}
