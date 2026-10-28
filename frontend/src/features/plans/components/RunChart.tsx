import { Tooltip, Typography, theme } from 'antd'
import { useTranslation } from 'react-i18next'
import { enumLabel } from '@/shared/utils/enums'
import { formatBytes, formatDuration, formatUtcDateTime } from '@/shared/utils/format'
import { chartBars } from '../runChart'
import { RUN_STATUS_TOKEN } from '../runStatus'
import type { Run } from '../types'

const BOX = { width: 600, height: 96, slots: 30, gap: 4, minHeight: 4 }

/** The last 30 runs as bars: height is the archive size, colour the status. A bar opens its run. */
export function RunChart({ runs, onOpen }: { runs: Run[]; onOpen: (run: Run) => void }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const bars = chartBars(runs, BOX)

  if (bars.length === 0) {
    return (
      <Typography.Text type="secondary" data-testid="plan-runs-chart-empty">
        {t('runs.empty')}
      </Typography.Text>
    )
  }
  return (
    <svg
      data-testid="plan-runs-chart"
      viewBox={`0 0 ${BOX.width} ${BOX.height}`}
      preserveAspectRatio="none"
      role="img"
      aria-label={t('runs.chart')}
      style={{ display: 'block', width: '100%', height: BOX.height, borderBottom: `1px solid ${token.colorBorderSecondary}` }}
    >
      {bars.map((bar) => (
        <Tooltip
          key={bar.run.id}
          title={
            <>
              <div>{formatUtcDateTime(bar.run.started_at ?? bar.run.created_at)}</div>
              <div>
                {enumLabel(t, 'runStatus', bar.run.status)} · {formatDuration(bar.run.duration_ms)} · {formatBytes(bar.run.size_bytes)}
              </div>
            </>
          }
        >
          <rect
            data-testid={`plan-runs-chart-bar-${bar.run.id}`}
            data-status={bar.run.status}
            x={bar.x}
            y={bar.y}
            width={bar.width}
            height={bar.height}
            rx={1.5}
            fill={token[RUN_STATUS_TOKEN[bar.run.status]]}
            style={{ cursor: 'pointer' }}
            onClick={() => onOpen(bar.run)}
          />
        </Tooltip>
      ))}
    </svg>
  )
}
