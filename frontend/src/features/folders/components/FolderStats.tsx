import type { ReactNode } from 'react'
import { Card, Col, Row, Typography, theme } from 'antd'
import { ExclamationCircleOutlined, SyncOutlined } from '@ant-design/icons'
import { useTranslation } from 'react-i18next'
import { formatBytes, formatCount, formatPercent } from '@/shared/utils/format'
import { folderTotals } from '../overview'
import { savedPercent } from '../savings'
import type { Folder } from '../types'

interface Tile {
  id: 'saved' | 'optimized' | 'pending' | 'failed' | 'optimizing' | 'scans'
  total: number
  value: string
  /** A line under the value. */
  caption?: string
  icon?: ReactNode
  danger?: boolean
}

/** One stat tile per total of the given folders: every folder on the dashboard, one on its page. */
export function FolderStats({ folders, testId }: { folders: readonly Folder[]; testId: string }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const totals = folderTotals(folders)
  const percent = savedPercent({ saved: totals.bytesSaved, before: totals.bytesBefore })
  const working = (count: number) => (count > 0 ? <SyncOutlined spin /> : undefined)

  const tiles: Tile[] = [
    {
      id: 'saved',
      total: totals.bytesSaved,
      value: formatBytes(totals.bytesSaved),
      caption: percent === null ? undefined : t('overview.savedOf', { percent: formatPercent(percent), before: formatBytes(totals.bytesBefore) }),
    },
    { id: 'optimized', total: totals.optimized, value: formatCount(totals.optimized) },
    { id: 'pending', total: totals.pending, value: formatCount(totals.pending) },
    {
      id: 'failed',
      total: totals.failed,
      value: formatCount(totals.failed),
      danger: totals.failed > 0,
      icon: totals.failed > 0 ? <ExclamationCircleOutlined /> : undefined,
    },
    { id: 'optimizing', total: totals.optimizing, value: formatCount(totals.optimizing), icon: working(totals.optimizing) },
    { id: 'scans', total: totals.scans, value: formatCount(totals.scans), icon: working(totals.scans) },
  ]

  return (
    <Row gutter={[token.margin, token.margin]}>
      {tiles.map((tile) => (
        <Col key={tile.id} xs={12} md={8} xl={4}>
          <Card size="small" data-testid={`${testId}-${tile.id}`} data-value={tile.total} style={{ height: '100%' }}>
            <Typography.Text type="secondary">{t(`overview.${tile.id}`)}</Typography.Text>
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: token.marginXS,
                marginTop: token.marginXXS,
                fontSize: token.fontSizeHeading3,
                fontWeight: token.fontWeightStrong,
                lineHeight: token.lineHeightHeading3,
                color: tile.danger ? token.colorError : token.colorText,
              }}
            >
              {tile.icon && <span style={{ fontSize: token.fontSizeLG, color: tile.danger ? undefined : token.colorPrimary }}>{tile.icon}</span>}
              {tile.value}
            </div>
            {tile.caption && (
              <Typography.Text type="secondary" data-testid={`${testId}-${tile.id}-caption`} style={{ fontSize: token.fontSizeSM }}>
                {tile.caption}
              </Typography.Text>
            )}
          </Card>
        </Col>
      ))}
    </Row>
  )
}
