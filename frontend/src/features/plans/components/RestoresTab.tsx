import { Typography } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { usePageParams, useTablePagination } from '@/shared/hooks/usePageParams'
import { EMPTY, formatDuration, formatUtcDateTime } from '@/shared/utils/format'
import { usePlanRestores } from '../hooks'
import { targetSummary } from '../restore'
import { objectName } from '../source'
import type { Restore } from '../types'
import { RunStatusTag } from './RunStatusTag'

/** Every restore of the plan, newest first. A restore opens its log, and its error, in the drawer. */
export function RestoresTab({ planId, onOpen }: { planId: number; onOpen: (restore: Restore) => void }) {
  const { t } = useTranslation()
  const paging = usePageParams()
  const restores = usePlanRestores(planId, paging.params)
  const pagination = useTablePagination(paging, restores.data)

  const status = (restore: Restore) => <RunStatusTag status={restore.status} testId={`restore-status-${restore.id}`} />
  const archive = (restore: Restore) =>
    restore.archive ? (
      <span style={{ overflowWrap: 'anywhere' }}>
        {objectName(restore.archive.object_key)}
        <Typography.Text type="secondary"> · {restore.archive.storage_name}</Typography.Text>
      </span>
    ) : (
      <Typography.Text type="secondary">{t('restores.archiveGone')}</Typography.Text>
    )
  const target = (restore: Restore) => (
    <Typography.Text code style={{ overflowWrap: 'anywhere' }}>
      {targetSummary(restore.target) ?? t('restores.originalLocationsShort')}
    </Typography.Text>
  )
  const by = (restore: Restore) => restore.created_by?.name || restore.created_by?.email || EMPTY

  const columns: ColumnsType<Restore> = [
    { title: t('runs.status'), key: 'status', width: 120, render: (_, restore) => status(restore) },
    { title: t('restores.archive'), key: 'archive', render: (_, restore) => <div style={{ maxWidth: 220 }}>{archive(restore)}</div> },
    { title: t('restores.target'), key: 'target', render: (_, restore) => <div style={{ maxWidth: 220 }}>{target(restore)}</div> },
    { title: t('runs.started'), key: 'started', width: 200, render: (_, restore) => formatUtcDateTime(restore.started_at ?? restore.created_at) },
    { title: t('runs.duration'), key: 'duration', width: 100, render: (_, restore) => formatDuration(restore.duration_ms) },
    { title: t('restores.by'), key: 'by', render: (_, restore) => by(restore) },
  ]

  return (
    <ResponsiveTable
      testId="plan-restores-table"
      items={restores.data?.data}
      loading={restores.isLoading || restores.isPlaceholderData}
      columns={columns}
      emptyText={t('restores.empty')}
      pagination={pagination}
      onItemClick={onOpen}
      cardTitle={(restore) => formatUtcDateTime(restore.started_at ?? restore.created_at)}
      cardExtra={status}
      cardTestId={(restore) => `restore-card-${restore.id}`}
      renderCard={(restore) => (
        <>
          <div>{archive(restore)}</div>
          <div>{target(restore)}</div>
          <Typography.Text type="secondary">
            {by(restore)} · {formatDuration(restore.duration_ms)}
          </Typography.Text>
        </>
      )}
    />
  )
}
