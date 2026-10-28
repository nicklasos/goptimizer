import { Typography, theme } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { usePageParams, useTablePagination } from '@/shared/hooks/usePageParams'
import { enumLabel } from '@/shared/utils/enums'
import { formatBytes, formatDuration, formatUtcDateTime } from '@/shared/utils/format'
import { usePlanRuns } from '../hooks'
import type { Run } from '../types'
import { RunChart } from './RunChart'
import { RunStatusTag } from './RunStatusTag'

const CHART_RUNS = { page: 1, page_size: 30 }

/** The chart of the last 30 runs, then every run, newest first. A run opens in the drawer. */
export function RunsTab({ planId, onOpen }: { planId: number; onOpen: (run: Run) => void }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const paging = usePageParams()
  const recent = usePlanRuns(planId, CHART_RUNS)
  const runs = usePlanRuns(planId, paging.params)
  const pagination = useTablePagination(paging, runs.data)

  const status = (run: Run) => <RunStatusTag status={run.status} testId={`run-status-${run.id}`} />
  // The table sizes columns to their content, so a long error is cut to a width here.
  const error = (run: Run, maxWidth: number | string) =>
    run.error ? (
      <Typography.Text type="danger" ellipsis={{ tooltip: run.error }} style={{ display: 'block', maxWidth }}>
        {run.error}
      </Typography.Text>
    ) : null

  const columns: ColumnsType<Run> = [
    { title: t('runs.status'), key: 'status', width: 120, render: (_, run) => status(run) },
    { title: t('runs.trigger'), key: 'trigger', width: 130, render: (_, run) => enumLabel(t, 'runTrigger', run.trigger) },
    { title: t('runs.started'), key: 'started', width: 210, render: (_, run) => formatUtcDateTime(run.started_at ?? run.created_at) },
    { title: t('runs.duration'), key: 'duration', width: 140, render: (_, run) => formatDuration(run.duration_ms) },
    { title: t('runs.size'), key: 'size', width: 110, align: 'right', render: (_, run) => formatBytes(run.size_bytes) },
    { title: t('runs.error'), key: 'error', render: (_, run) => error(run, 260) },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: token.marginLG }}>
      <div>
        <Typography.Text type="secondary" style={{ display: 'block', marginBottom: token.marginXS }}>
          {t('runs.chart')}
        </Typography.Text>
        <RunChart runs={recent.data?.data ?? []} onOpen={onOpen} />
      </div>
      <ResponsiveTable
        testId="plan-runs-table"
        items={runs.data?.data}
        loading={runs.isLoading || runs.isPlaceholderData}
        columns={columns}
        emptyText={t('runs.empty')}
        pagination={pagination}
        onItemClick={onOpen}
        cardTitle={(run) => formatUtcDateTime(run.started_at ?? run.created_at)}
        cardExtra={status}
        cardTestId={(run) => `run-card-${run.id}`}
        renderCard={(run) => (
          <>
            <Typography.Text type="secondary">
              {enumLabel(t, 'runTrigger', run.trigger)} · {formatDuration(run.duration_ms)} · {formatBytes(run.size_bytes)}
            </Typography.Text>
            <div>{error(run, '100%')}</div>
          </>
        )}
      />
    </div>
  )
}
