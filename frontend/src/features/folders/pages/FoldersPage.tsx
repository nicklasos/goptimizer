import { Button, Card, Col, Empty, Row, Space, Switch, Tag, Typography, theme } from 'antd'
import { EditOutlined, FolderOutlined, PlusOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { enumLabel } from '@/shared/utils/enums'
import { useFolders, useSetFolderEnabled } from '../hooks'
import type { Folder } from '../types'

/** The dashboard: one card per folder. */
export default function FoldersPage() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const navigate = useNavigate()
  const folders = useFolders()

  const addButton = (testId: string) => (
    <Button data-testid={testId} type="primary" icon={<PlusOutlined />} onClick={() => navigate('/folders/new')}>
      {t('folders.add')}
    </Button>
  )

  const content = () => {
    if (folders.isLoading) return <PageLoading />
    if (folders.isError) return <PageError error={folders.error} onRetry={() => folders.refetch()} />
    if (!folders.data?.length) {
      return (
        <Card>
          <Empty data-testid="folders-empty" description={t('folders.empty')} image={Empty.PRESENTED_IMAGE_SIMPLE}>
            {addButton('folders-empty-add-button')}
          </Empty>
        </Card>
      )
    }
    return (
      <Row gutter={[token.margin, token.margin]}>
        {folders.data.map((folder) => (
          <Col key={folder.id} xs={24} md={12} xl={8}>
            <FolderCard folder={folder} />
          </Col>
        ))}
      </Row>
    )
  }

  return (
    <PageStack testId="dashboard-page">
      {/* PageHeader brings its own bottom margin, and this card holds nothing else */}
      <Card styles={{ body: { paddingBottom: 0 } }}>
        <PageHeader
          icon={<FolderOutlined />}
          title={t('folders.title')}
          subtitle={t('folders.subtitle')}
          mainAction={addButton('add-folder-button')}
        />
      </Card>
      {content()}
    </PageStack>
  )
}

function FolderCard({ folder }: { folder: Folder }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const navigate = useNavigate()
  const setEnabled = useSetFolderEnabled()
  const pending = setEnabled.isPending ? setEnabled.variables : undefined
  const enabled = pending?.id === folder.id ? pending.enabled : folder.enabled

  return (
    <Card
      data-testid={`folder-card-${folder.id}`}
      title={folder.name}
      extra={
        <Space size={token.marginXS}>
          <Typography.Text type="secondary">{enabled ? t('folders.active') : t('folders.paused')}</Typography.Text>
          <Switch
            data-testid={`folder-enabled-switch-${folder.id}`}
            checked={enabled}
            loading={setEnabled.isPending}
            onChange={(checked) => setEnabled.mutate({ id: folder.id, enabled: checked })}
          />
        </Space>
      }
      actions={[
        <Button
          key="edit"
          type="link"
          icon={<EditOutlined />}
          data-testid={`edit-folder-button-${folder.id}`}
          onClick={() => navigate(`/folders/${folder.id}/edit`)}
        >
          {t('common.edit')}
        </Button>,
      ]}
      style={{ height: '100%', opacity: enabled ? 1 : 0.7 }}
    >
      <Space direction="vertical" size={token.marginSM} style={{ width: '100%' }}>
        <Typography.Text code ellipsis={{ tooltip: folder.path }} style={{ maxWidth: '100%' }}>
          {folder.path}
        </Typography.Text>
        <Space size={token.marginXS} wrap>
          <Tag
            data-testid={`folder-mode-tag-${folder.id}`}
            data-mode={folder.profile.mode}
            color={folder.profile.mode === 'lossy' ? 'blue' : 'green'}
          >
            {enumLabel(t, 'mode', folder.profile.mode)}
          </Tag>
          <Typography.Text type="secondary">
            {t('folders.scanSummary', { minutes: folder.scan_interval_minutes, hours: folder.full_scan_interval_hours })}
          </Typography.Text>
        </Space>
      </Space>
    </Card>
  )
}
