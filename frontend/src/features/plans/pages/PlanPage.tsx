import { Button, Card, Descriptions, Grid, Switch, Tabs, Tooltip, Typography } from 'antd'
import { EditOutlined } from '@ant-design/icons'
import { useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { enumLabel } from '@/shared/utils/enums'
import { formatBytes, formatRelative, formatUtcDateTime } from '@/shared/utils/format'
import message from '@/shared/utils/message'
import { ArchivesTab } from '../components/ArchivesTab'
import { HealthTag } from '../components/HealthTag'
import { RestoreDrawer } from '../components/RestoreDrawer'
import { RestoresTab } from '../components/RestoresTab'
import { RunDrawer } from '../components/RunDrawer'
import { RunNowButton } from '../components/RunNowButton'
import { RunsTab } from '../components/RunsTab'
import { StoragesTab } from '../components/StoragesTab'
import { usePlan, useSetPlanEnabled } from '../hooks'
import { retentionSummary } from '../planForm'
import { scheduleSummary } from '../schedule'
import { SOURCE_ICON, sourceSummary } from '../source'
import type { Archive, Plan } from '../types'

const TABS = ['runs', 'archives', 'restores', 'storages'] as const
type Tab = (typeof TABS)[number]

/** A plan: how it is doing, what it backs up, its runs with their logs, its archives to restore, its restores and its storages. */
export default function PlanPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const id = Number(useParams().id)
  const [search, setSearch] = useSearchParams()
  const plan = usePlan(id)
  const [restoring, setRestoring] = useState<Archive>()

  const back = { label: t('navigation.dashboard'), onClick: () => navigate('/'), testId: 'plan-back-button' }
  if (plan.isLoading) return <PageLoading />
  if (plan.isError || !plan.data) return <PageError error={plan.error} back={back} onRetry={() => plan.refetch()} />

  const tab: Tab = TABS.find((key) => key === search.get('tab')) ?? 'runs'
  const idParam = (name: string) => {
    const value = Number(search.get(name))
    return Number.isInteger(value) && value > 0 ? value : undefined
  }
  const openRun = idParam('run')
  const openRestore = idParam('restore')
  const update = (change: Record<string, string | null>) =>
    setSearch((current) => {
      const next = new URLSearchParams(current)
      for (const [key, value] of Object.entries(change)) {
        if (value == null) next.delete(key)
        else next.set(key, value)
      }
      return next
    })

  return (
    <PageStack testId="plan-page">
      <Card>
        <PlanHeader plan={plan.data} back={back} />
        <PlanFacts plan={plan.data} />
      </Card>
      <Card>
        <Tabs
          activeKey={tab}
          onChange={(key) => setSearch({ tab: key })}
          items={[
            {
              key: 'runs',
              label: <span data-testid="plan-tab-runs">{t('plans.page.runs')}</span>,
              children: <RunsTab planId={plan.data.id} onOpen={(run) => update({ run: String(run.id) })} />,
            },
            {
              key: 'archives',
              label: <span data-testid="plan-tab-archives">{t('plans.page.archives')}</span>,
              children: <ArchivesTab plan={plan.data} onRestore={setRestoring} />,
            },
            {
              key: 'restores',
              label: <span data-testid="plan-tab-restores">{t('plans.page.restores')}</span>,
              children: <RestoresTab planId={plan.data.id} onOpen={(restore) => update({ restore: String(restore.id) })} />,
            },
            {
              key: 'storages',
              label: <span data-testid="plan-tab-storages">{t('plans.page.storages')}</span>,
              children: <StoragesTab plan={plan.data} />,
            },
          ]}
        />
      </Card>
      <RunDrawer runId={openRun} onClose={() => update({ run: null })} />
      <RestoreDrawer
        plan={plan.data}
        archive={restoring}
        restoreId={openRestore}
        onStarted={(restore) => {
          setRestoring(undefined)
          update({ tab: 'restores', restore: String(restore.id) })
        }}
        onClose={() => {
          setRestoring(undefined)
          update({ restore: null })
        }}
      />
    </PageStack>
  )
}

function PlanHeader({ plan, back }: { plan: Plan; back: { label: string; onClick: () => void; testId: string } }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const setEnabled = useSetPlanEnabled()
  const onToggle = (enabled: boolean) =>
    setEnabled.mutate({ id: plan.id, enabled }, { onSuccess: () => message.success(t(enabled ? 'plans.page.resumed' : 'plans.page.paused')) })

  return (
    <PageHeader
      title={plan.name}
      titleTestId="plan-title"
      tags={<HealthTag health={plan.health} testId="plan-health" />}
      back={back}
      extra={
        <>
          <Tooltip title={t('plans.enabledHint')}>
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: 8 }}>
              <Switch checked={plan.enabled} loading={setEnabled.isPending} onChange={onToggle} data-testid="plan-enabled-toggle" />
              {t('plans.enabled')}
            </span>
          </Tooltip>
          <Button icon={<EditOutlined />} onClick={() => navigate(`/plans/${plan.id}/edit`)} data-testid="plan-edit-button">
            {t('common.edit')}
          </Button>
        </>
      }
      mainAction={<RunNowButton plan={plan} testId="plan-run-button" />}
    />
  )
}

function PlanFacts({ plan }: { plan: Plan }) {
  const { t } = useTranslation()
  const screens = Grid.useBreakpoint()
  const SourceIcon = SOURCE_ICON[plan.source_kind]
  const next = plan.next_run_at ? formatUtcDateTime(plan.next_run_at) : t(plan.enabled ? 'plans.manual' : 'plans.paused')
  return (
    <Descriptions size="small" column={screens.lg ? 2 : 1} data-testid="plan-facts">
      <Descriptions.Item label={t('plans.source')}>
        <span data-testid="plan-source-summary" style={{ overflowWrap: 'anywhere' }}>
          <SourceIcon /> {enumLabel(t, 'sourceKind', plan.source_kind)}{' '}
          <Typography.Text code>{sourceSummary(plan)}</Typography.Text>
        </span>
      </Descriptions.Item>
      <Descriptions.Item label={t('plans.schedules')}>
        <span data-testid="plan-schedule-summary">{scheduleSummary(t, plan.schedules)}</span>
      </Descriptions.Item>
      <Descriptions.Item label={t('plans.page.nextRun')}>
        <span data-testid="plan-next-run">{next}</span>
      </Descriptions.Item>
      <Descriptions.Item label={t('plans.page.lastBackup')}>
        <span data-testid="plan-last-backup">
          {plan.last_success_at ? (
            <Tooltip title={formatUtcDateTime(plan.last_success_at)}>
              <span>
                {formatRelative(plan.last_success_at)}
                {plan.last_size_bytes != null && `, ${formatBytes(plan.last_size_bytes)}`}
              </span>
            </Tooltip>
          ) : (
            t('dashboard.noBackup')
          )}
        </span>
      </Descriptions.Item>
      <Descriptions.Item label={t('plans.page.keep')} span={screens.lg ? 2 : 1}>
        <span data-testid="plan-retention-summary">{retentionSummary(t, plan)}</span>
      </Descriptions.Item>
    </Descriptions>
  )
}
