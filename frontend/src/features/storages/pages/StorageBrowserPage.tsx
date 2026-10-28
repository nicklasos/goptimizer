import { Alert, Breadcrumb, Button, Card, Typography, theme } from 'antd'
import { FileOutlined, FolderFilled, ReloadOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { isApiError } from '@/api/errors'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { EMPTY, formatBytes, formatDateTime } from '@/shared/utils/format'
import { errorMessage } from '@/shared/utils/serverErrors'
import { childPrefix, normalizePrefix, prefixCrumbs } from '../browsePath'
import { useStorage, useStorageObjects } from '../hooks'
import { storageLocation } from '../storageForm'
import type { StorageObject } from '../types'

type Row = StorageObject & { id: number }

export default function StorageBrowserPage() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const navigate = useNavigate()
  const id = Number(useParams().id)
  const [search, setSearch] = useSearchParams()
  const prefix = normalizePrefix(search.get('prefix'))
  const storage = useStorage(id)
  const objects = useStorageObjects(id, prefix)

  const back = { label: t('storages.title'), onClick: () => navigate('/storages'), testId: 'storage-browser-back-button' }

  if (storage.isLoading) return <PageLoading />
  if (storage.isError || !storage.data) return <PageError error={storage.error} back={back} onRetry={() => storage.refetch()} />

  const open = (next: string) => setSearch(next ? { prefix: next } : {})
  const rows: Row[] = (objects.data ?? []).map((object, index) => ({ ...object, id: index }))

  const name = (row: Row) =>
    row.is_dir ? (
      <Button
        type="link"
        icon={<FolderFilled />}
        onClick={() => open(childPrefix(prefix, row.name))}
        data-testid={`storage-folder-${row.name}`}
        style={{ paddingInline: 0, height: 'auto', whiteSpace: 'normal', textAlign: 'left' }}
      >
        {row.name}
      </Button>
    ) : (
      <span data-testid={`storage-file-${row.name}`} style={{ overflowWrap: 'anywhere' }}>
        <FileOutlined style={{ color: token.colorTextSecondary, marginRight: token.marginXS }} />
        {row.name}
      </span>
    )

  const columns: ColumnsType<Row> = [
    { title: t('storages.objectName'), key: 'name', render: (_, row) => name(row) },
    { title: t('storages.objectSize'), key: 'size', width: 140, align: 'right', render: (_, row) => (row.is_dir ? EMPTY : formatBytes(row.size)) },
    { title: t('storages.objectModified'), key: 'modified', width: 180, render: (_, row) => formatDateTime(row.modified_at) },
  ]

  const reason = isApiError(objects.error) && typeof objects.error.details?.reason === 'string' ? objects.error.details.reason : undefined

  return (
    <PageStack testId="storage-browser-page">
      <Card>
        <PageHeader
          title={storage.data.name}
          subtitle={<Typography.Text code style={{ overflowWrap: 'anywhere' }}>{storageLocation(storage.data)}</Typography.Text>}
          back={back}
          extra={
            <Button icon={<ReloadOutlined />} onClick={() => objects.refetch()} loading={objects.isFetching} data-testid="storage-browser-refresh-button">
              {t('storages.refresh')}
            </Button>
          }
        />

        <Breadcrumb
          data-testid="storage-browser-breadcrumb"
          style={{ marginBottom: token.marginMD, overflowWrap: 'anywhere' }}
          items={[
            { key: 'root', title: <a onClick={() => open('')} data-testid="storage-crumb-root">{t('storages.root')}</a> },
            ...prefixCrumbs(prefix).map((crumb) => ({
              key: crumb.prefix,
              title: <a onClick={() => open(crumb.prefix)} data-testid={`storage-crumb-${crumb.name}`}>{crumb.name}</a>,
            })),
          ]}
        />

        {objects.isError ? (
          <div data-testid="storage-browser-error">
            <Alert
              type="error"
              showIcon
              message={errorMessage(t, objects.error)}
              description={reason}
            />
          </div>
        ) : (
          <ResponsiveTable
            testId="storage-objects-table"
            items={rows}
            loading={objects.isFetching}
            columns={columns}
            emptyText={t('storages.folderEmpty')}
            pagination={false}
            cardTitle={name}
            cardExtra={(row) => (row.is_dir ? null : formatBytes(row.size))}
            renderCard={(row) => (row.modified_at ? formatDateTime(row.modified_at) : null)}
          />
        )}
      </Card>
    </PageStack>
  )
}
