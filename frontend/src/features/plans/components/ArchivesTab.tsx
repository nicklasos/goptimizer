import { useState } from 'react'
import { Button, Select, Space, Tooltip, Typography, theme } from 'antd'
import { DownloadOutlined, FolderOpenOutlined, RollbackOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { Link } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { usePageParams, useTablePagination } from '@/shared/hooks/usePageParams'
import { formatBytes, formatUtcDateTime } from '@/shared/utils/format'
import { useDownloadArchive, usePlanArchives } from '../hooks'
import { browseUrl, objectFolder, objectName } from '../source'
import type { Archive, Plan } from '../types'

/** The archives retention has kept, on every storage of the plan or one. Each can be restored. */
export function ArchivesTab({ plan, onRestore }: { plan: Plan; onRestore: (archive: Archive) => void }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const paging = usePageParams()
  const [storageId, setStorageId] = useState<number>()
  const archives = usePlanArchives(plan.id, paging.params, storageId)
  const pagination = useTablePagination(paging, archives.data)
  const download = useDownloadArchive()

  const onStorage = (value: number | undefined) => {
    setStorageId(value)
    paging.setPage(1, paging.params.page_size)
  }

  const name = (archive: Archive) => (
    <span data-testid={`archive-name-${archive.id}`} title={archive.object_key} style={{ overflowWrap: 'anywhere' }}>
      {objectName(archive.object_key)}
    </span>
  )
  // The table sizes columns to their content: the name wraps instead of pushing the actions out.
  const nameCell = (archive: Archive) => <div style={{ maxWidth: 260 }}>{name(archive)}</div>
  const sha = (archive: Archive) => (
    <Typography.Text code copyable={{ text: archive.sha256 }} data-testid={`archive-sha-${archive.id}`}>
      {archive.sha256.slice(0, 12)}…
    </Typography.Text>
  )
  const actions = (archive: Archive) => (
    <Space size={token.marginXS} wrap>
      <Button
        size="small"
        icon={<DownloadOutlined />}
        loading={download.isPending && download.variables === archive.id}
        onClick={() => download.mutate(archive.id)}
        data-testid={`archive-download-button-${archive.id}`}
      >
        {t('archives.download')}
      </Button>
      <Button size="small" icon={<RollbackOutlined />} onClick={() => onRestore(archive)} data-testid={`archive-restore-button-${archive.id}`}>
        {t('restores.restore')}
      </Button>
      <Tooltip title={t('archives.browse')}>
        <Link
          to={browseUrl(archive.storage_id, objectFolder(archive.object_key))}
          aria-label={t('archives.browse')}
          data-testid={`archive-browse-link-${archive.id}`}
        >
          <FolderOpenOutlined />
        </Link>
      </Tooltip>
    </Space>
  )

  const columns: ColumnsType<Archive> = [
    { title: t('archives.file'), key: 'name', render: (_, archive) => nameCell(archive) },
    { title: t('archives.storage'), dataIndex: 'storage_name', key: 'storage', width: 140 },
    { title: t('archives.size'), key: 'size', width: 100, align: 'right', render: (_, archive) => formatBytes(archive.size_bytes) },
    { title: t('archives.created'), key: 'created', width: 200, render: (_, archive) => formatUtcDateTime(archive.created_at) },
    { title: t('archives.sha256'), key: 'sha', width: 160, render: (_, archive) => sha(archive) },
    { title: t('common.actions'), key: 'actions', width: 250, render: (_, archive) => actions(archive) },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: token.marginMD }}>
      <Select
        data-testid="archives-storage-filter"
        allowClear
        placeholder={t('archives.allStorages')}
        value={storageId}
        onChange={onStorage}
        options={plan.storages.map((storage) => ({ value: storage.id, label: storage.name }))}
        style={{ width: 260, maxWidth: '100%' }}
      />
      <ResponsiveTable
        testId="plan-archives-table"
        items={archives.data?.data}
        loading={archives.isLoading || archives.isPlaceholderData}
        columns={columns}
        emptyText={t('archives.empty')}
        pagination={pagination}
        cardTitle={name}
        cardExtra={(archive) => formatBytes(archive.size_bytes)}
        cardTestId={(archive) => `archive-card-${archive.id}`}
        renderCard={(archive) => (
          <>
            <Typography.Text type="secondary">
              {archive.storage_name} · {formatUtcDateTime(archive.created_at)}
            </Typography.Text>
            <div>{sha(archive)}</div>
          </>
        )}
        cardActions={actions}
      />
    </div>
  )
}
