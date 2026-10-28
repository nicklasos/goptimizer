import { Card, Space, Tag, Tooltip, Typography, theme } from 'antd'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { formatBytes, formatNextRun, formatRelative, formatUtcDateTime } from '@/shared/utils/format'
import { HEALTH } from '../health'
import { SOURCE_ICON } from '../source'
import type { DashboardPlan } from '../types'
import { DayStrip } from './DayStrip'
import { HealthTag } from './HealthTag'
import { RunNowButton } from './RunNowButton'

/** A plan on the dashboard. The whole card opens the plan; Run now does not. */
export function PlanCard({ plan, now }: { plan: DashboardPlan; now: Date }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const navigate = useNavigate()
  const SourceIcon = SOURCE_ICON[plan.source_kind]

  const lastBackup = plan.last_success_at ? (
    <Tooltip title={formatUtcDateTime(plan.last_success_at)}>
      <span>
        {t('dashboard.lastBackup', { when: formatRelative(plan.last_success_at, now) })}
        {plan.last_size_bytes != null && `, ${formatBytes(plan.last_size_bytes)}`}
      </span>
    </Tooltip>
  ) : (
    t('dashboard.noBackup')
  )

  const next = plan.next_run_at ? (
    <Tooltip title={formatUtcDateTime(plan.next_run_at)}>
      <span>{t('dashboard.next', { when: formatNextRun(plan.next_run_at, now) })}</span>
    </Tooltip>
  ) : (
    t(plan.enabled ? 'plans.manual' : 'plans.paused')
  )

  return (
    <Card
      hoverable
      onClick={() => navigate(`/plans/${plan.id}`)}
      data-testid={`plan-card-${plan.id}`}
      data-health={plan.health}
      style={{ height: '100%', borderInlineStart: `4px solid ${token[HEALTH[plan.health].color]}` }}
      styles={{ body: { display: 'flex', flexDirection: 'column', gap: token.marginSM, height: '100%' } }}
    >
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: token.marginSM }}>
        <div style={{ minWidth: 0 }}>
          <Typography.Title level={4} ellipsis={{ tooltip: plan.name }} style={{ margin: 0 }} data-testid={`plan-card-name-${plan.id}`}>
            {plan.name}
          </Typography.Title>
          <Typography.Text type="secondary">
            <SourceIcon /> {enumLabel(t, 'sourceKind', plan.source_kind)}
          </Typography.Text>
        </div>
        <HealthTag health={plan.health} testId={`plan-health-${plan.id}`} />
      </div>

      <div>
        <div data-testid={`plan-last-backup-${plan.id}`}>{lastBackup}</div>
        <Typography.Text type="secondary" data-testid={`plan-next-run-${plan.id}`}>
          {next}
        </Typography.Text>
      </div>

      <Space size={[4, 4]} wrap>
        {plan.storages.map((storage) => (
          <Tag key={storage.id} bordered={false} style={{ marginInlineEnd: 0 }}>
            {storage.name}
          </Tag>
        ))}
      </Space>

      <div style={{ marginTop: 'auto', display: 'flex', alignItems: 'center', gap: token.marginSM }}>
        <div style={{ flex: 1, minWidth: 0 }}>
          <DayStrip planId={plan.id} days={plan.days} />
        </div>
        <div onClick={(event) => event.stopPropagation()}>
          <RunNowButton plan={plan} size="small" testId={`run-plan-button-${plan.id}`} />
        </div>
      </div>
    </Card>
  )
}
