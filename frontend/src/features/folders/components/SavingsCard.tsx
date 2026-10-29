import { useRef, useState, type KeyboardEvent } from 'react'
import { Card, Typography, theme } from 'antd'
import { useTranslation } from 'react-i18next'
import type { ID } from '@/api/types'
import { useElementWidth } from '@/shared/hooks/useElementWidth'
import { formatBytes, formatDay, formatNumber } from '@/shared/utils/format'
import { useDailySavings } from '../hooks'
import { byteTicks, totalSaved } from '../savings'
import type { DaySavings } from '../types'

const PLOT_TOP = 8
const PLOT_HEIGHT = 160
const AXIS_HEIGHT = 24
const HEIGHT = PLOT_TOP + PLOT_HEIGHT + AXIS_HEIGHT
const MAX_BAR = 24
const RADIUS = 4
// A day with any saving stays visible next to a day that saved a thousand times more.
const MIN_BAR = 2
// Roughly the width of one character of a tick label at the small font size, to size the y-axis gutter.
const LABEL_CHAR = 7

interface Props {
  /** One folder's days; all folders together without it. */
  folderId?: ID
  refetchInterval: number | false
  testId: string
}

/** Bytes saved on each of the last 30 days, as one series of columns. */
export function SavingsCard({ folderId, refetchInterval, testId }: Props) {
  const { t } = useTranslation()
  const savings = useDailySavings(folderId, refetchInterval)
  const total = savings.data ? totalSaved(savings.data) : undefined

  return (
    <Card
      data-testid={testId}
      data-total={total}
      title={t('savings.title')}
      extra={
        total !== undefined && (
          <Typography.Text type="secondary">{total > 0 ? t('savings.total', { saved: formatBytes(total) }) : t('savings.none')}</Typography.Text>
        )
      }
    >
      <SavingsChart days={savings.data ?? []} />
    </Card>
  )
}

function SavingsChart({ days }: { days: DaySavings[] }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const [box, width] = useElementWidth<HTMLDivElement>()
  const [active, setActive] = useState<number | null>(null)
  const [tabStop, setTabStop] = useState<number | null>(null)
  const targets = useRef<(SVGRectElement | null)[]>([])

  const ticks = byteTicks(Math.max(0, ...days.map((day) => day.bytes_saved)))
  const top = ticks[ticks.length - 1] || 1
  const gutter = Math.max(...ticks.map((tick) => formatBytes(tick).length)) * LABEL_CHAR + token.marginSM
  const slot = days.length ? Math.max(0, width - gutter) / days.length : 0
  const barWidth = Math.min(MAX_BAR, Math.max(2, slot * 0.6))
  const y = (bytes: number) => PLOT_TOP + PLOT_HEIGHT * (1 - bytes / top)
  const baseline = y(0)
  const slotLeft = (index: number) => gutter + index * slot
  const barLeft = (index: number) => slotLeft(index) + (slot - barWidth) / 2
  const focusable = tabStop ?? days.length - 1

  const saved = (day: DaySavings) => t('savings.saved', { saved: formatBytes(day.bytes_saved) })
  const optimized = (day: DaySavings) => t('savings.images', { count: day.optimized, images: formatNumber(day.optimized, 0) })

  const onKeyDown = (event: KeyboardEvent, index: number) => {
    const next = { ArrowLeft: index - 1, ArrowRight: index + 1, Home: 0, End: days.length - 1 }[event.key]
    if (next === undefined) return
    event.preventDefault()
    const target = Math.min(days.length - 1, Math.max(0, next))
    setTabStop(target)
    targets.current[target]?.focus()
  }

  // First, middle and last day, kept inside the plot by anchoring the outer two at their bar's edge.
  const xLabels: { index: number; x: number; anchor: 'start' | 'middle' | 'end' }[] =
    days.length > 2
      ? [
          { index: 0, x: barLeft(0), anchor: 'start' },
          { index: Math.floor((days.length - 1) / 2), x: slotLeft(Math.floor((days.length - 1) / 2)) + slot / 2, anchor: 'middle' },
          { index: days.length - 1, x: barLeft(days.length - 1) + barWidth, anchor: 'end' },
        ]
      : []

  const day = active === null ? undefined : days[active]
  const onLeftHalf = active !== null && active < days.length / 2

  return (
    <div ref={box} style={{ position: 'relative', height: HEIGHT }}>
      {width > 0 && days.length > 0 && (
        <svg width={width} height={HEIGHT} role="group" aria-label={t('savings.chartLabel')} style={{ display: 'block', overflow: 'visible' }}>
          {active !== null && (
            <rect x={slotLeft(active)} y={PLOT_TOP} width={slot} height={PLOT_HEIGHT} rx={token.borderRadiusSM} fill={token.colorFillTertiary} />
          )}
          {ticks.map((tick) => (
            <g key={tick}>
              <line x1={gutter} x2={width} y1={y(tick)} y2={y(tick)} stroke={token.colorBorderSecondary} shapeRendering="crispEdges" />
              <text
                x={gutter - token.marginXS}
                y={y(tick)}
                textAnchor="end"
                dominantBaseline="middle"
                fontSize={token.fontSizeSM}
                fill={token.colorTextSecondary}
                style={{ fontVariantNumeric: 'tabular-nums' }}
              >
                {formatBytes(tick)}
              </text>
            </g>
          ))}
          {days.map((entry, index) => (
            <path
              key={entry.day}
              d={columnPath(barLeft(index), entry.bytes_saved > 0 ? Math.min(y(entry.bytes_saved), baseline - MIN_BAR) : baseline, barWidth, baseline)}
              fill={index === active ? token.colorPrimaryHover : token.colorPrimary}
            />
          ))}
          {xLabels.map(({ index, x, anchor }) => (
            <text key={index} x={x} y={baseline + AXIS_HEIGHT - token.marginXS} textAnchor={anchor} fontSize={token.fontSizeSM} fill={token.colorTextSecondary}>
              {formatDay(days[index].day)}
            </text>
          ))}
          {days.map((entry, index) => (
            <rect
              key={entry.day}
              ref={(element) => {
                targets.current[index] = element
              }}
              data-testid={`savings-day-${entry.day}`}
              data-bytes={entry.bytes_saved}
              data-count={entry.optimized}
              role="img"
              aria-label={`${formatDay(entry.day, 'long')}: ${saved(entry)}, ${optimized(entry)}`}
              tabIndex={index === focusable ? 0 : -1}
              x={slotLeft(index)}
              y={PLOT_TOP}
              width={slot}
              height={PLOT_HEIGHT}
              fill="transparent"
              onPointerEnter={() => setActive(index)}
              onPointerLeave={() => setActive(null)}
              onFocus={() => {
                setActive(index)
                setTabStop(index)
              }}
              onBlur={() => setActive(null)}
              onKeyDown={(event) => onKeyDown(event, index)}
            />
          ))}
        </svg>
      )}
      {day && active !== null && (
        <div
          role="tooltip"
          data-testid="savings-tooltip"
          style={{
            position: 'absolute',
            top: PLOT_TOP,
            left: onLeftHalf ? slotLeft(active) + slot + token.marginXXS : slotLeft(active) - token.marginXXS,
            transform: onLeftHalf ? undefined : 'translateX(-100%)',
            pointerEvents: 'none',
            zIndex: 1,
            whiteSpace: 'nowrap',
            padding: `${token.paddingXS}px ${token.paddingSM}px`,
            background: token.colorBgElevated,
            borderRadius: token.borderRadius,
            boxShadow: token.boxShadowSecondary,
            fontSize: token.fontSizeSM,
            lineHeight: token.lineHeightSM,
          }}
        >
          <div style={{ fontSize: token.fontSize, fontWeight: token.fontWeightStrong, color: token.colorText }}>{saved(day)}</div>
          <div style={{ color: token.colorText }}>{optimized(day)}</div>
          <div style={{ color: token.colorTextSecondary }}>{formatDay(day.day, 'long')}</div>
        </div>
      )}
    </div>
  )
}

/** A column standing on the baseline, its top corners rounded and its foot square. Nothing for zero height. */
function columnPath(x: number, top: number, width: number, base: number): string {
  const height = base - top
  if (height <= 0) return ''
  const r = Math.min(RADIUS, width / 2, height)
  return `M${x},${base}V${top + r}A${r},${r} 0 0 1 ${x + r},${top}H${x + width - r}A${r},${r} 0 0 1 ${x + width},${top + r}V${base}Z`
}
