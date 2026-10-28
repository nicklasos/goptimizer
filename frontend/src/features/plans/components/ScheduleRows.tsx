import { Button, Form, Space, Tag, TimePicker, Typography, theme } from 'antd'
import { ClockCircleOutlined, MinusCircleOutlined, PlusOutlined } from '@ant-design/icons'
import dayjs, { type Dayjs } from 'dayjs'
import { useTranslation } from 'react-i18next'
import { formatUtcDateTime } from '@/shared/utils/format'
import type { ScheduleRow } from '../planForm'
import { duplicateSchedules, nextRun, parseTimeOfDay } from '../schedule'
import { WEEKDAYS, type Weekday } from '../types'

const TIME_FORMAT = 'HH:mm'

/** The form keeps "HH:MM"; the picker wants a dayjs. The date part is irrelevant. */
const toPicker = (value: string | undefined) => {
  const minutes = value ? parseTimeOfDay(value) : null
  return minutes == null ? null : dayjs().startOf('day').add(minutes, 'minute')
}

interface Props {
  rows: ScheduleRow[]
}

/** The "When" section: one row per time of day, the days it runs on, and when the next run is. */
export function ScheduleRows({ rows }: Props) {
  const { t } = useTranslation()
  const { token } = theme.useToken()
  const duplicates = new Set(duplicateSchedules(rows))
  const valid = rows.filter((row) => parseTimeOfDay(row.time) != null)
  const next = nextRun(valid, new Date())

  return (
    <>
      <Typography.Paragraph type="secondary">{t('plans.schedule.hint')}</Typography.Paragraph>
      <Form.List name="schedules">
        {(fields, { add, remove }) => (
          <Space direction="vertical" size={token.marginSM} style={{ width: '100%' }} data-testid="plan-schedules">
            {fields.map((field, index) => (
              <div
                key={field.key}
                data-testid={`plan-schedule-row-${index}`}
                style={{ display: 'flex', flexWrap: 'wrap', gap: token.marginXS, alignItems: 'flex-start' }}
              >
                <Form.Item
                  name={[field.name, 'time']}
                  rules={[{ required: true, message: t('common.required') }]}
                  validateStatus={duplicates.has(index) ? 'error' : undefined}
                  help={duplicates.has(index) ? t('plans.schedule.duplicate') : undefined}
                  getValueProps={(value: string) => ({ value: toPicker(value) })}
                  normalize={(value: Dayjs | null) => (value ? value.format(TIME_FORMAT) : '')}
                  style={{ marginBottom: 0 }}
                >
                  <TimePicker
                    format={TIME_FORMAT}
                    minuteStep={5}
                    needConfirm={false}
                    allowClear={false}
                    showNow={false}
                    suffixIcon={<span>{t('plans.schedule.utc')}</span>}
                    aria-label={t('plans.schedule.time')}
                    data-testid={`plan-schedule-time-${index}`}
                    style={{ width: 120 }}
                  />
                </Form.Item>
                <Form.Item name={[field.name, 'days']} style={{ marginBottom: 0, flex: '1 1 240px', minWidth: 0 }}>
                  <DayPicker testId={`plan-schedule-${index}`} />
                </Form.Item>
                <Button
                  type="text"
                  icon={<MinusCircleOutlined />}
                  aria-label={t('plans.schedule.remove')}
                  onClick={() => remove(field.name)}
                  data-testid={`plan-schedule-remove-${index}`}
                />
              </div>
            ))}
            <Button
              type="dashed"
              size="small"
              icon={<PlusOutlined />}
              onClick={() => add({ time: suggestTime(rows), days: [] })}
              data-testid="plan-schedule-add"
            >
              {t('plans.schedule.add')}
            </Button>
          </Space>
        )}
      </Form.List>
      <Typography.Paragraph data-testid="plan-next-run" data-next={next?.toISOString() ?? ""} strong={next != null} type={next ? undefined : 'secondary'} style={{ marginTop: token.marginSM }}>
        <ClockCircleOutlined style={{ marginRight: token.marginXS }} />
        {next ? t('plans.schedule.nextRun', { when: formatUtcDateTime(next) }) : `${t('plans.schedule.manualOnly')}. ${t('plans.schedule.manualHint')}`}
      </Typography.Paragraph>
    </>
  )
}

/** A new row starts twelve hours after the last one, so it is not a duplicate straight away. */
function suggestTime(rows: ScheduleRow[]): string {
  const last = rows.length ? parseTimeOfDay(rows[rows.length - 1].time) : null
  const minutes = last == null ? 3 * 60 : (last + 12 * 60) % (24 * 60)
  return `${String(Math.floor(minutes / 60)).padStart(2, '0')}:${String(minutes % 60).padStart(2, '0')}`
}

interface DayPickerProps {
  value?: Weekday[]
  onChange?: (days: Weekday[]) => void
  testId: string
}

/** "Every day", or the weekdays picked. Unpicking the last day goes back to every day. */
function DayPicker({ value = [], onChange, testId }: DayPickerProps) {
  const { t } = useTranslation()
  const everyDay = value.length === 0 || value.length === WEEKDAYS.length
  const toggle = (day: Weekday) => {
    const chosen = everyDay ? [] : value
    const next = chosen.includes(day) ? chosen.filter((d) => d !== day) : [...chosen, day]
    onChange?.(WEEKDAYS.filter((d) => next.includes(d)))
  }
  return (
    <Space size={[0, 4]} wrap style={{ paddingTop: 4 }}>
      <Tag.CheckableTag checked={everyDay} onChange={() => onChange?.([])} data-testid={`${testId}-every-day`}>
        {t('plans.schedule.everyDay')}
      </Tag.CheckableTag>
      {WEEKDAYS.map((day) => (
        <Tag.CheckableTag key={day} checked={!everyDay && value.includes(day)} onChange={() => toggle(day)} data-testid={`${testId}-day-${day}`}>
          {t(`enums.weekdayShort.${day}`)}
        </Tag.CheckableTag>
      ))}
    </Space>
  )
}
