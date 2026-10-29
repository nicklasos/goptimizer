import { Card, Space, Tag, Typography, theme } from 'antd'
import { FolderOutlined } from '@ant-design/icons'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { PageHeader, type PageBack } from '@/shared/components/PageHeader'
import { PageError, PageLoading } from '@/shared/components/PageState'
import { PageStack } from '@/shared/components/PageStack'
import { FolderModeTag, MissingToolsAlert } from '../components/FolderSummary'
import { FolderScans } from '../components/FolderScans'
import { FolderStats } from '../components/FolderStats'
import { SavingsCard } from '../components/SavingsCard'
import { useFolderControls } from '../components/useFolderControls'
import { useFolder } from '../hooks'
import { pollInterval } from '../overview'
import { scanLine } from '../scans'
import type { Folder } from '../types'

/** `/folders/:id`: one folder's settings, figures, images and scans. */
export default function FolderPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const params = useParams()
  const id = Number(params.id)
  const valid = Number.isInteger(id) && id > 0
  const folder = useFolder(valid ? id : undefined, { live: true })
  const back: PageBack = { label: t('folders.title'), onClick: () => navigate('/'), testId: 'folder-page-back-button' }

  const content = () => {
    if (!valid) return <PageError error={null} back={back} />
    if (folder.isLoading) return <PageLoading />
    if (folder.isError || !folder.data) return <PageError error={folder.error} back={back} onRetry={() => folder.refetch()} />
    return <FolderDetails folder={folder.data} back={back} />
  }

  return <PageStack testId="folder-page">{content()}</PageStack>
}

function FolderDetails({ folder, back }: { folder: Folder; back: PageBack }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const { enabledSwitch, scanButton, editButton } = useFolderControls(folder, 'default')
  const refetchInterval = pollInterval([folder])
  const scan = folder.last_scan
  const excludes = folder.profile.excludes

  return (
    <>
      <Card>
        <PageHeader
          back={back}
          icon={<FolderOutlined />}
          title={folder.name}
          titleTestId="folder-page-title"
          tags={<FolderModeTag folder={folder} />}
          subtitle={
            <Typography.Text code data-testid="folder-page-path" style={{ overflowWrap: 'anywhere' }}>
              {folder.path}
            </Typography.Text>
          }
          extra={
            <>
              {enabledSwitch}
              {scanButton}
              {editButton}
            </>
          }
        />
        <Space direction="vertical" size={token.marginSM} style={{ width: '100%' }}>
          <Typography.Text type="secondary">
            {t('folders.scanSummary', { minutes: folder.scan_interval_minutes, hours: folder.full_scan_interval_hours })}
          </Typography.Text>
          {excludes.length > 0 && (
            <Space size={token.marginXXS} wrap data-testid="folder-page-excludes">
              <Typography.Text type="secondary">{t('folders.excluded')}</Typography.Text>
              {excludes.map((pattern) => (
                <Tag key={pattern} style={{ fontFamily: token.fontFamilyCode }}>
                  {pattern}
                </Tag>
              ))}
            </Space>
          )}
          <Typography.Text
            type={scan?.status === 'failed' ? 'danger' : undefined}
            data-testid={`folder-scan-status-${folder.id}`}
            data-status={scan?.status ?? 'none'}
          >
            {scanLine(t, scan)}
          </Typography.Text>
          <MissingToolsAlert folder={folder} />
        </Space>
      </Card>
      <FolderStats folders={[folder]} testId="folder-stats" />
      <SavingsCard folderId={folder.id} refetchInterval={refetchInterval} testId="folder-savings-chart" />
      <FolderScans folder={folder} refetchInterval={refetchInterval} />
    </>
  )
}
