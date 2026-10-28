import type { Schedule, Weekday } from './types'
import { WEEKDAYS } from './types'

const MINUTE = 60_000
const DAY = 24 * 60 * MINUTE

/** "HH:MM" as minutes after midnight, or null when it is not a time of day. */
export function parseTimeOfDay(value: string): number | null {
  const match = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(value)
  return match ? Number(match[1]) * 60 + Number(match[2]) : null
}

/** getUTCDay() counts from Sunday; WEEKDAYS from Monday. */
const weekdayOf = (date: Date): Weekday => WEEKDAYS[(date.getUTCDay() + 6) % 7]

/**
 * The first time after `after` that one of the schedules is due, in UTC, or null when there are
 * none (the plan only runs by hand). The API's plans.NextRun, with the same cases.
 */
export function nextRun(schedules: Schedule[], after: Date): Date | null {
  const midnight = Date.UTC(after.getUTCFullYear(), after.getUTCMonth(), after.getUTCDate())
  for (let offset = 0; offset <= 7; offset++) {
    const day = new Date(midnight + offset * DAY)
    let next: number | null = null
    for (const schedule of schedules) {
      const minutes = parseTimeOfDay(schedule.time)
      if (minutes == null) continue
      if (schedule.days.length > 0 && !schedule.days.includes(weekdayOf(day))) continue
      const at = day.getTime() + minutes * MINUTE
      if (at > after.getTime() && (next == null || at < next)) next = at
    }
    if (next != null) return new Date(next)
  }
  return null
}

/** The days of a schedule in WEEKDAYS order, with all seven as none (every day), as the API stores them. */
export function normalizeDays(days: readonly Weekday[]): Weekday[] {
  const chosen = WEEKDAYS.filter((day) => days.includes(day))
  return chosen.length === WEEKDAYS.length ? [] : chosen
}

/** Indexes of rows that repeat an earlier row, which the API refuses. */
export function duplicateSchedules(schedules: Schedule[]): number[] {
  const seen = new Set<string>()
  const duplicates: number[] = []
  schedules.forEach((schedule, index) => {
    const key = `${schedule.time} ${normalizeDays(schedule.days).join(',')}`
    if (seen.has(key)) duplicates.push(index)
    seen.add(key)
  })
  return duplicates
}

type Translate = (key: string, options?: Record<string, unknown>) => string

/** "Mon–Fri" for a run of three or more days, "Mon, Wed" otherwise. */
function daysLabel(t: Translate, days: Weekday[]): string {
  if (days.length === 0) return t('plans.schedule.daily')
  const indexes = days.map((day) => WEEKDAYS.indexOf(day))
  const consecutive = indexes.every((index, i) => i === 0 || index === indexes[i - 1] + 1)
  const name = (day: Weekday) => t(`enums.weekdayShort.${day}`)
  if (consecutive && days.length >= 3) return `${name(days[0])}–${name(days[days.length - 1])}`
  return days.map(name).join(', ')
}

/**
 * The timetable in one line: "Daily 03:00, 18:00 UTC", "Daily 03:00 · Mon–Fri 18:00 UTC".
 * Rows with the same days share a label, in the order they first appear.
 */
export function scheduleSummary(t: Translate, schedules: Schedule[]): string {
  if (schedules.length === 0) return t('plans.schedule.manualOnly')
  const groups = new Map<string, { days: Weekday[]; times: string[] }>()
  for (const schedule of schedules) {
    const days = normalizeDays(schedule.days)
    const key = days.join(',')
    const group = groups.get(key) ?? { days, times: [] }
    group.times.push(schedule.time)
    groups.set(key, group)
  }
  const parts = [...groups.values()].map(({ days, times }) => `${daysLabel(t, days)} ${[...times].sort().join(', ')}`)
  return `${parts.join(' · ')} UTC`
}
