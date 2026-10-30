/**
 * Display formatting for numbers and dates.
 * The locale follows the app language (`setFormatLanguage`, called by i18n), never the browser.
 * Every formatter returns `—` for a missing value, never 0.
 */

export const EMPTY = '—'
type Lang = 'uk' | 'en'

const LOCALES: Record<Lang, string> = { uk: 'uk-UA', en: 'en-GB' }

let lang: Lang = 'en'

export function setFormatLanguage(language: string | undefined): void {
  lang = language?.startsWith('uk') ? 'uk' : 'en'
}

const missing = (value: number | null | undefined): value is null | undefined => value == null || !Number.isFinite(value)

export function formatNumber(value: number | null | undefined, digits = 2): string {
  if (missing(value)) return EMPTY
  return new Intl.NumberFormat(LOCALES[lang], { maximumFractionDigits: digits }).format(value)
}

/** A count for a headline figure: in full below 10,000, then compact by the language: "12.9k", "1.3m". */
export function formatCount(value: number | null | undefined): string {
  if (missing(value)) return EMPTY
  if (Math.abs(value) < 10_000) return formatNumber(value, 0)
  return new Intl.NumberFormat(LOCALES[lang], { notation: 'compact', maximumFractionDigits: 1 }).format(value)
}

/** A timestamp in local time: "11.09.2026 14:05". */
export function formatDateTime(value: string | null | undefined): string {
  if (!value) return EMPTY
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(date.getDate())}.${pad(date.getMonth() + 1)}.${date.getFullYear()} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

/** A UTC calendar day ("2026-10-29"): "29 Oct" short, "Thu, 29 Oct 2026" long. */
export function formatDay(day: string, style: 'short' | 'long' = 'short'): string {
  const date = new Date(`${day}T00:00:00Z`)
  if (Number.isNaN(date.getTime())) return day
  const options: Intl.DateTimeFormatOptions =
    style === 'short' ? { day: 'numeric', month: 'short' } : { weekday: 'short', day: 'numeric', month: 'short', year: 'numeric' }
  return new Intl.DateTimeFormat(LOCALES[lang], { ...options, timeZone: 'UTC' }).format(date)
}

/** A share out of 100, whole: "24%". */
export function formatPercent(value: number | null | undefined): string {
  if (missing(value)) return EMPTY
  return `${formatNumber(value, 0)}%`
}

/** A byte count in binary units: "512 B", "1.5 KB", "3.2 MB". */
export function formatBytes(bytes: number | null | undefined): string {
  if (missing(bytes)) return EMPTY
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let value = bytes
  let unit = 0
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024
    unit += 1
  }
  return `${formatNumber(value, unit === 0 ? 0 : 1)} ${units[unit]}`
}

const SECOND = 1000
const MINUTE = 60 * SECOND
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

const relativeSteps: { unit: Intl.RelativeTimeFormatUnit; ms: number; below: number }[] = [
  { unit: 'second', ms: SECOND, below: 45 * SECOND },
  { unit: 'minute', ms: MINUTE, below: 45 * MINUTE },
  { unit: 'hour', ms: HOUR, below: 22 * HOUR },
  { unit: 'day', ms: DAY, below: 26 * DAY },
  { unit: 'month', ms: 30 * DAY, below: 320 * DAY },
  { unit: 'year', ms: 365 * DAY, below: Infinity },
]

/** How long ago (or how far ahead) a moment is: "3 hours ago", "yesterday", "in 5 minutes". */
export function formatRelative(value: string | null | undefined, now: Date = new Date()): string {
  if (!value) return EMPTY
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const diff = date.getTime() - now.getTime()
  const step = relativeSteps.find(({ below }) => Math.abs(diff) < below) ?? relativeSteps[relativeSteps.length - 1]
  const amount = step.unit === 'second' ? 0 : Math.round(diff / step.ms)
  return new Intl.RelativeTimeFormat(LOCALES[lang], { numeric: 'auto' }).format(amount, step.unit)
}

/** How long something took, in its two largest units: "850 ms", "12 sec", "3 min, 5 sec", "1 hr, 2 min". */
export function formatDuration(ms: number | null | undefined): string {
  if (missing(ms)) return EMPTY
  const unit = (amount: number, name: string) =>
    new Intl.NumberFormat(LOCALES[lang], { style: 'unit', unit: name, unitDisplay: 'short' }).format(amount)
  if (ms < SECOND) return unit(Math.round(ms), 'millisecond')
  const seconds = Math.round(ms / SECOND)
  const parts =
    seconds < 60
      ? [unit(seconds, 'second')]
      : seconds < 3600
        ? [unit(Math.floor(seconds / 60), 'minute'), unit(seconds % 60, 'second')]
        : [unit(Math.floor(seconds / 3600), 'hour'), unit(Math.floor((seconds % 3600) / 60), 'minute')]
  return parts.join(', ')
}
