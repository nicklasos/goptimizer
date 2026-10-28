import { useState } from 'react'
import { Alert, Button, Card, Tag, Tooltip, Typography, theme } from 'antd'
import { CloudServerOutlined, FolderOpenOutlined, PlusOutlined } from '@ant-design/icons'
import type { ColumnsType } from 'antd/es/table'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageStack } from '@/shared/components/PageStack'
import { ResponsiveTable } from '@/shared/components/ResponsiveTable'
import { RowActions } from '@/shared/components/RowActions'
import { usePageParams, useTablePagination } from '@/shared/hooks/usePageParams'
import { enumLabel } from '@/shared/utils/enums'
import message from '@/shared/utils/message'
import { errorMessage } from '@/shared/utils/serverErrors'
import { useDeleteStorage, useStorages } from '../hooks'
import { plansUsingStorage, type PlanUsingStorage } from '../inUse'
import { storageLocation } from '../storageForm'
import type { Storage } from '../types'

export default function StoragesPage() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const navigate = useNavigate()
  const paging = usePageParams()
  const storages = useStorages(paging.params)
  const remove = useDeleteStorage()
  const pagination = useTablePagination(paging, storages.data)
  const [inUse, setInUse] = useState<PlanUsingStorage[] | null>(null)

  const onDelete = (storage: Storage) => {
    setInUse(null)
    remove.mutate(storage.id, {
      onSuccess: () => message.success(t('storages.deleted')),
      onError: (error) => {
        const plans = plansUsingStorage(error)
        if (plans) setInUse(plans)
        else message.error(errorMessage(t, error))
      },
    })
  }

  const kindTag = (storage: Storage) => (
    <Tag bordered={false} color="processing" data-testid={`storage-kind-${storage.id}`}>
      {enumLabel(t, 'storageKind', storage.kind)}
    </Tag>
  )

  const secretTag = (storage: Storage) => {
    if (storage.kind === 'local') return <Typography.Text type="secondary">{t('storages.secretNotNeeded')}</Typography.Text>
    return storage.secret_set ? (
      <Tag bordered={false} color="success">
        {t('storages.secretStored')}
      </Tag>
    ) : (
      <Tag bordered={false} color="warning">
        {t('storages.secretMissing')}
      </Tag>
    )
  }

  const location = (storage: Storage) => (
    <Typography.Text code style={{ overflowWrap: 'anywhere' }}>
      {storageLocation(storage)}
    </Typography.Text>
  )

  const actions = (storage: Storage) => (
    <RowActions
      entity="storage"
      id={storage.id}
      onEdit={() => navigate(`/storages/${storage.id}/edit`)}
      onDelete={() => onDelete(storage)}
      deleteTitle={t('storages.deleteConfirm')}
      deleteDescription={t('storages.deleteHint')}
    >
      <Tooltip title={t('storages.browse')}>
        <Button
          type="text"
          icon={<FolderOpenOutlined />}
          size="small"
          onClick={() => navigate(`/storages/${storage.id}/browse`)}
          data-testid={`browse-storage-button-${storage.id}`}
        />
      </Tooltip>
    </RowActions>
  )

  const columns: ColumnsType<Storage> = [
    { title: t('storages.nameField'), dataIndex: 'name', key: 'name' },
    { title: t('storages.kindField'), key: 'kind', render: (_, storage) => kindTag(storage) },
    {
      title: t('storages.location'),
      key: 'location',
      render: (_, storage) => (
        <Typography.Text
          ellipsis={{ tooltip: storageLocation(storage) }}
          style={{ maxWidth: 260, display: 'inline-block', fontFamily: token.fontFamilyCode, fontSize: token.fontSizeSM }}
        >
          {storageLocation(storage)}
        </Typography.Text>
      ),
    },
    { title: t('storages.credentials'), key: 'secret', render: (_, storage) => secretTag(storage) },
    { title: t('common.actions'), key: 'actions', width: 140, fixed: 'right', render: (_, storage) => actions(storage) },
  ]

  return (
    <PageStack testId="storages-page">
      <Card>
        <PageHeader
          icon={<CloudServerOutlined />}
          title={t('storages.title')}
          subtitle={t('storages.subtitle')}
          mainAction={
            <Button data-testid="storages-add-button" type="primary" icon={<PlusOutlined />} onClick={() => navigate('/storages/new')}>
              {t('storages.add')}
            </Button>
          }
        />
        {inUse && (
          <Alert
            data-testid="storage-in-use-alert"
            type="warning"
            showIcon
            closable
            onClose={() => setInUse(null)}
            message={t('storages.inUse', { plans: inUse.map((plan) => plan.name).join(', ') })}
            style={{ marginBottom: token.marginMD }}
          />
        )}
        <ResponsiveTable
          testId="storages-table"
          items={storages.data?.data}
          loading={storages.isFetching || remove.isPending}
          columns={columns}
          emptyText={t('storages.empty')}
          pagination={pagination}
          cardTitle={(storage) => storage.name}
          cardExtra={kindTag}
          cardTestId={(storage) => `storage-card-${storage.id}`}
          renderCard={(storage) => (
            <>
              <div style={{ minWidth: 0 }}>{location(storage)}</div>
              <div>{secretTag(storage)}</div>
            </>
          )}
          cardActions={actions}
        />
      </Card>
    </PageStack>
  )
}
