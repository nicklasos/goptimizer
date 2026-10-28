import { Tooltip, theme } from 'antd'
import { useTranslation } from 'react-i18next'
import { formatDay } from '@/shared/utils/format'
import { DAY_COLOR, dayKind } from '../days'
import type { Day } from '../types'

/** The last 30 UTC days of a plan, one segment each: red with a failure, green with only successes, grey with no runs. */
export function DayStrip({ planId, days }: { planId: number; days: Day[] }) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  return (
    <div data-testid={`plan-days-${planId}`} style={{ display: 'flex', gap: 2, height: 14 }}>
      {days.map((day) => {
        const kind = dayKind(day)
        const date = formatDay(day.date)
        const label = kind === 'none' ? t('dashboard.dayNone', { date }) : t('dashboard.dayRuns', { date, succeeded: day.succeeded, failed: day.failed })
        return (
          <Tooltip key={day.date} title={label}>
            <div
              data-testid={`plan-day-${planId}-${day.date}`}
              data-kind={kind}
              style={{ flex: 1, minWidth: 0, borderRadius: 2, background: token[DAY_COLOR[kind]] }}
            />
          </Tooltip>
        )
      })}
    </div>
  )
}
