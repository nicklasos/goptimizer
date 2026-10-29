import { Card, Space, Tag, Typography, theme, type TagProps } from 'antd'
import type { ColumnsType } from 'antd/es/table'
import { useTranslation } from 'react-i18next'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { usePageParams, useTablePagination } from '@/shared/hooks/usePageParams'
import { enumLabel } from '@/shared/utils/enums'
import { EMPTY, formatDateTime, formatDuration, formatNumber } from '@/shared/utils/format'
import { useFolderScans } from '../hooks'
import { scanDuration } from '../scans'
import type { Folder, Scan, ScanStatus } from '../types'

const STATUS_COLOR = {
  queued: 'default',
  running: 'processing',
  succeeded: 'success',
  failed: 'error',
} satisfies Record<ScanStatus, TagProps['color']>

/** The folder's scans, newest first, a page at a time. */
export function FolderScans({ folder, refetchInterval }: { folder: Folder; refetchInterval: number | false }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const paging = usePageParams(10)
  const scans = useFolderScans(folder.id, paging.params, refetchInterval)
  const pagination = useTablePagination(paging, scans.data)

  const statusTag = (scan: Scan) => (
    <Tag color={STATUS_COLOR[scan.status]} data-testid={`scan-status-${scan.id}`} data-status={scan.status}>
      {enumLabel(t, 'scanStatus', scan.status)}
    </Tag>
  )
  const started = (scan: Scan) => formatDateTime(scan.started_at ?? scan.queued_at)
  const duration = (scan: Scan) => formatDuration(scanDuration(scan))
  const count = (value: number) => formatNumber(value, 0)
  const numeric = { align: 'right' as const, render: count }

  const columns: ColumnsType<Scan> = [
    { title: t('scans.started'), key: 'started', render: (_, scan) => started(scan) },
    { title: t('scans.kind'), dataIndex: 'kind', key: 'kind', render: (kind: Scan['kind']) => enumLabel(t, 'scanKind', kind) },
    { title: t('scans.trigger'), dataIndex: 'trigger', key: 'trigger', render: (trigger: Scan['trigger']) => enumLabel(t, 'scanTrigger', trigger) },
    { title: t('scans.status'), key: 'status', render: (_, scan) => statusTag(scan) },
    { title: t('scans.duration'), key: 'duration', align: 'right', render: (_, scan) => duration(scan) },
    { title: t('scans.dirsListed'), dataIndex: 'dirs_listed', key: 'dirs_listed', ...numeric },
    { title: t('scans.filesSeen'), dataIndex: 'files_seen', key: 'files_seen', ...numeric },
    { title: t('scans.filesNew'), dataIndex: 'files_new', key: 'files_new', ...numeric },
    { title: t('scans.filesChanged'), dataIndex: 'files_changed', key: 'files_changed', ...numeric },
    { title: t('scans.filesGone'), dataIndex: 'files_gone', key: 'files_gone', ...numeric },
    {
      title: t('scans.error'),
      dataIndex: 'error',
      key: 'error',
      width: 280,
      render: (error: string | null | undefined) =>
        error ? (
          <Typography.Text type="danger" ellipsis={{ tooltip: error }} style={{ maxWidth: 280 }}>
            {error}
          </Typography.Text>
        ) : (
          EMPTY
        ),
    },
  ]

  return (
    <Card title={t('scans.title')} data-testid="folder-scans">
      <ResponsiveTable
        testId="scans-table"
        items={scans.data?.data}
        loading={scans.isLoading || (scans.isPlaceholderData && scans.isFetching)}
        columns={columns}
        emptyText={t('scans.empty')}
        pagination={pagination ?? false}
        cardTestId={(scan) => `scan-card-${scan.id}`}
        cardTitle={started}
        cardExtra={statusTag}
        renderCard={(scan) => (
          <Space direction="vertical" size={token.marginXXS} style={{ width: '100%' }}>
            <Typography.Text type="secondary">
              {[enumLabel(t, 'scanKind', scan.kind), enumLabel(t, 'scanTrigger', scan.trigger), duration(scan)].join(' · ')}
            </Typography.Text>
            <Typography.Text>
              {t('scans.filesLine', {
                dirs: count(scan.dirs_listed),
                seen: count(scan.files_seen),
                new: count(scan.files_new),
                changed: count(scan.files_changed),
                gone: count(scan.files_gone),
              })}
            </Typography.Text>
            {scan.error && <Typography.Text type="danger">{scan.error}</Typography.Text>}
          </Space>
        )}
      />
    </Card>
  )
}
