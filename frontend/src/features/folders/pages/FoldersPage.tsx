import { Alert, Button, Card, Col, Dropdown, Empty, Row, Space, Switch, Tag, Typography, theme } from 'antd'
import { DownOutlined, EditOutlined, FolderOutlined, PlusOutlined, SyncOutlined } from '@ant-design/icons'
import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { enumLabel } from '@/shared/utils/enums'
import { formatBytes, formatNumber, formatRelative } from '@/shared/utils/format'
import type { Translate } from '@/shared/utils/serverErrors'
import { useFolders, useSetFolderEnabled, useStartScan } from '../hooks'
import { imagesIn, indexedImages, isScanActive } from '../scans'
import { SCAN_KINDS, type Folder, type Scan, type ScanKind } from '../types'

/** The dashboard: one card per folder. */
export default function FoldersPage() {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const navigate = useNavigate()
  const folders = useFolders()

  // The empty state and the header never show together, so both buttons share one test id.
  const addButton = (
    <Button data-testid="add-folder-button" type="primary" icon={<PlusOutlined />} onClick={() => navigate('/folders/new')}>
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
            {addButton}
          </Empty>
        </Card>
      )
    }
    return (
      <>
        {/* PageHeader brings its own bottom margin, and this card holds nothing else */}
        <Card styles={{ body: { paddingBottom: 0 } }}>
          <PageHeader
            icon={<FolderOutlined />}
            title={t('folders.title')}
            subtitle={t('folders.subtitle')}
            mainAction={addButton}
          />
        </Card>
        <Row gutter={[token.margin, token.margin]}>
          {folders.data.map((folder) => (
            <Col key={folder.id} xs={24} md={12} xl={8}>
              <FolderCard folder={folder} />
            </Col>
          ))}
        </Row>
      </>
    )
  }

  return <PageStack testId="dashboard-page">{content()}</PageStack>
}

function FolderCard({ folder }: { folder: Folder }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const navigate = useNavigate()
  const setEnabled = useSetFolderEnabled()
  const startScan = useStartScan()
  const pending = setEnabled.isPending ? setEnabled.variables : undefined
  const enabled = pending?.id === folder.id ? pending.enabled : folder.enabled
  const scan = folder.last_scan
  const scanning = isScanActive(scan)
  const images = indexedImages(folder)
  const optimized = imagesIn(folder, 'optimized')
  const failed = imagesIn(folder, 'failed')
  const missingTools = folder.missing_tools

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
        <Dropdown
          key="scan"
          trigger={['click']}
          disabled={!enabled || scanning}
          menu={{
            items: SCAN_KINDS.map((kind) => ({
              key: kind,
              label: <span data-testid={`${kind}-scan-item-${folder.id}`}>{t(`folders.scan.${kind}`)}</span>,
            })),
            onClick: ({ key }) => startScan.mutate({ id: folder.id, kind: key as ScanKind }),
          }}
        >
          <Button
            type="link"
            icon={<SyncOutlined spin={scanning} />}
            loading={startScan.isPending}
            disabled={!enabled || scanning}
            data-testid={`scan-folder-button-${folder.id}`}
          >
            {t('folders.scan.now')} <DownOutlined />
          </Button>
        </Dropdown>,
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
        <Typography.Text strong data-testid={`folder-image-count-${folder.id}`} data-count={images}>
          {t('folders.scan.images', { count: images, images: formatNumber(images, 0) })}
        </Typography.Text>
        <Typography.Text
          type={scan?.status === 'failed' ? 'danger' : 'secondary'}
          data-testid={`folder-scan-status-${folder.id}`}
          data-status={scan?.status ?? 'none'}
        >
          {scanLine(t, scan)}
        </Typography.Text>
        <Space size={token.marginXS} wrap>
          <Typography.Text data-testid={`folder-optimized-${folder.id}`} data-count={optimized} data-bytes={folder.bytes_saved}>
            {optimized > 0 || folder.bytes_saved > 0
              ? t('folders.optimize.summary', { count: optimized, images: formatNumber(optimized, 0), saved: formatBytes(folder.bytes_saved) })
              : t('folders.optimize.none')}
          </Typography.Text>
          {failed > 0 && (
            <Typography.Text type="danger" data-testid={`folder-failed-${folder.id}`} data-count={failed}>
              {t('folders.optimize.failed', { count: failed, images: formatNumber(failed, 0) })}
            </Typography.Text>
          )}
        </Space>
        {missingTools.length > 0 && (
          <Alert
            type="warning"
            showIcon
            data-testid={`folder-missing-tools-${folder.id}`}
            message={t('folders.optimize.missingTools', { count: missingTools.length, tools: missingTools.join(', ') })}
            action={
              <Link to="/settings" data-testid={`folder-missing-tools-link-${folder.id}`}>
                {t('folders.optimize.install')}
              </Link>
            }
          />
        )}
      </Space>
    </Card>
  )
}

/** The last scan in one line: queued, its progress, what it found, or why it failed. */
function scanLine(t: Translate, scan: Scan | null | undefined): string {
  if (!scan) return t('folders.scan.never')
  const kind = enumLabel(t, 'scanKind', scan.kind)
  const ago = formatRelative(scan.finished_at)
  const files = { count: scan.files_seen, files: formatNumber(scan.files_seen, 0) }
  switch (scan.status) {
    case 'queued':
      return t('folders.scan.queued', { kind })
    case 'running':
      return t('folders.scan.running', files)
    case 'failed':
      return t('folders.scan.failed', { kind, ago, error: scan.error })
    case 'succeeded':
      return scan.files_seen > 0 ? t('folders.scan.succeeded', { kind, ago, ...files }) : t('folders.scan.unchanged', { kind, ago })
  }
}
