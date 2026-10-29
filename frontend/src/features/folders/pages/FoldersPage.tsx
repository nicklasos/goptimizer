import { Button, Card, Col, Empty, Row, Space, Typography, theme } from 'antd'
import { FolderOutlined, PlusOutlined } from '@ant-design/icons'
import { Link, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { formatBytes, formatNumber } from '@/shared/utils/format'
import { FolderModeTag, MissingToolsAlert } from '../components/FolderSummary'
import { FolderStats } from '../components/FolderStats'
import { SavingsCard } from '../components/SavingsCard'
import { useFolderControls } from '../components/useFolderControls'
import { useFolders } from '../hooks'
import { pollInterval } from '../overview'
import { imagesIn, indexedImages, scanLine } from '../scans'
import type { Folder } from '../types'

/** The dashboard: what every folder adds up to, then one card per folder. */
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
        <FolderStats folders={folders.data} testId="overview" />
        <SavingsCard refetchInterval={pollInterval(folders.data)} testId="savings-chart" />
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
  const { enabled, enabledSwitch, scanButton, editButton } = useFolderControls(folder, 'link')
  const scan = folder.last_scan
  const images = indexedImages(folder)
  const optimized = imagesIn(folder, 'optimized')
  const failed = imagesIn(folder, 'failed')

  return (
    <Card
      data-testid={`folder-card-${folder.id}`}
      title={
        <Link to={`/folders/${folder.id}`} data-testid={`folder-link-${folder.id}`}>
          {folder.name}
        </Link>
      }
      extra={enabledSwitch}
      actions={[scanButton, editButton]}
      style={{ height: '100%', opacity: enabled ? 1 : 0.7 }}
    >
      <Space direction="vertical" size={token.marginSM} style={{ width: '100%' }}>
        <Typography.Text code ellipsis={{ tooltip: folder.path }} style={{ maxWidth: '100%' }}>
          {folder.path}
        </Typography.Text>
        <Space size={token.marginXS} wrap>
          <FolderModeTag folder={folder} />
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
        <MissingToolsAlert folder={folder} />
      </Space>
    </Card>
  )
}
