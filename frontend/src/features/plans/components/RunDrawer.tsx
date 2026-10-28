import { Alert, Button, Descriptions, Drawer, Grid, List, Spin, Typography, theme } from 'antd'
import { DownloadOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { formatBytes, formatDuration, formatUtcDateTime } from '@/shared/utils/format'
import { useDownloadArchive, useRun } from '../hooks'
import { isRunActive } from '../runStatus'
import { LogBox } from './LogBox'
import { RunStatusTag } from './RunStatusTag'

/** One run: its log, followed live while it runs, what it stored, and how it failed. */
export function RunDrawer({ runId, onClose }: { runId: number | undefined; onClose: () => void }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const screens = Grid.useBreakpoint()
  const run = useRun(runId)
  const download = useDownloadArchive()
  const data = run.data

  return (
    <Drawer
      open={runId != null}
      onClose={onClose}
      width={screens.md ? 760 : '100%'}
      title={runId != null && t('runs.drawerTitle', { id: runId })}
      extra={data && <RunStatusTag status={data.status} testId="run-drawer-status" />}
      destroyOnClose
    >
      <div data-testid="run-drawer">
        {!data ? (
          <Spin />
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: token.marginMD }}>
            {data.error && (
              <div data-testid="run-error">
                <Alert type="error" showIcon message={data.error} />
              </div>
            )}
            <Descriptions size="small" column={screens.md ? 2 : 1}>
              <Descriptions.Item label={t('runs.trigger')}>{enumLabel(t, 'runTrigger', data.trigger)}</Descriptions.Item>
              {data.scheduled_for && <Descriptions.Item label={t('runs.scheduledFor')}>{formatUtcDateTime(data.scheduled_for)}</Descriptions.Item>}
              <Descriptions.Item label={t('runs.started')}>{formatUtcDateTime(data.started_at)}</Descriptions.Item>
              <Descriptions.Item label={t('runs.finished')}>{formatUtcDateTime(data.finished_at)}</Descriptions.Item>
              <Descriptions.Item label={t('runs.duration')}>{formatDuration(data.duration_ms)}</Descriptions.Item>
              <Descriptions.Item label={t('runs.size')}>{formatBytes(data.size_bytes)}</Descriptions.Item>
            </Descriptions>

            <LogBox log={data.log} active={isRunActive(data)} testId="run-log" />

            <div>
              <Typography.Title level={5}>{t('runs.archives')}</Typography.Title>
              <List
                size="small"
                bordered
                dataSource={data.archives}
                locale={{ emptyText: t('runs.noArchives') }}
                renderItem={(archive) => (
                  <List.Item
                    data-testid={`run-archive-${archive.id}`}
                    actions={[
                      archive.deleted_at ? (
                        <Typography.Text type="secondary" key="deleted">
                          {t('archives.deleted')}
                        </Typography.Text>
                      ) : (
                        <Button
                          key="download"
                          size="small"
                          icon={<DownloadOutlined />}
                          loading={download.isPending && download.variables === archive.id}
                          onClick={() => download.mutate(archive.id)}
                          data-testid={`run-archive-download-button-${archive.id}`}
                        >
                          {t('archives.download')}
                        </Button>
                      ),
                    ]}
                  >
                    <List.Item.Meta
                      title={archive.storage_name}
                      description={
                        <Typography.Text type="secondary" style={{ overflowWrap: 'anywhere' }}>
                          {archive.object_key} · {formatBytes(archive.size_bytes)}
                        </Typography.Text>
                      }
                    />
                  </List.Item>
                )}
              />
            </div>
          </div>
        )}
      </div>
    </Drawer>
  )
}
