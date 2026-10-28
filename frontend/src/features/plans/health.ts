import type { ComponentType, CSSProperties } from 'react'
import {
  CheckCircleFilled,
  ClockCircleFilled,
  CloseCircleFilled,
  MinusCircleFilled,
  PauseCircleFilled,
  RollbackOutlined,
  SyncOutlined,
} from '@ant-design/icons'
import type { Dashboard, Health, HealthSummary, LastRun, Run } from './types'

/** A theme token for a colour, read with `theme.useToken()`. */
export type ColorToken =
  | 'colorSuccess'
  | 'colorError'
  | 'colorWarning'
  | 'colorPrimary'
  | 'purple'
  | 'colorTextTertiary'
  | 'colorTextQuaternary'
  | 'colorFillSecondary'

export interface HealthLook {
  /** The Ant Design preset colour of its tag. */
  tag: 'success' | 'error' | 'warning' | 'processing' | 'purple' | 'default'
  /** The colour of a card's strip. */
  color: ColorToken
  Icon: ComponentType<{ spin?: boolean; style?: CSSProperties }>
  spin: boolean
}

/** How each health looks. The API decides the health (plans.PlanHealth); this only colours it. */
export const HEALTH: Record<Health, HealthLook> = {
  ok: { tag: 'success', color: 'colorSuccess', Icon: CheckCircleFilled, spin: false },
  failed: { tag: 'error', color: 'colorError', Icon: CloseCircleFilled, spin: false },
  overdue: { tag: 'warning', color: 'colorWarning', Icon: ClockCircleFilled, spin: false },
  running: { tag: 'processing', color: 'colorPrimary', Icon: SyncOutlined, spin: true },
  restoring: { tag: 'purple', color: 'purple', Icon: RollbackOutlined, spin: false },
  paused: { tag: 'default', color: 'colorTextTertiary', Icon: PauseCircleFilled, spin: false },
  never: { tag: 'default', color: 'colorTextTertiary', Icon: MinusCircleFilled, spin: false },
}

/** Problems first, in the summary. */
export const HEALTH_ORDER: readonly Health[] = ['failed', 'overdue', 'restoring', 'running', 'ok', 'paused', 'never']

/** The healths that some plan has, with how many, in HEALTH_ORDER. */
export function summaryCounts(summary: HealthSummary): { health: Health; count: number }[] {
  return HEALTH_ORDER.map((health) => ({ health, count: summary[health] })).filter(({ count }) => count > 0)
}

export function lastRunOf(run: Run): LastRun {
  return { id: run.id, status: run.status, trigger: run.trigger, started_at: run.started_at, finished_at: run.finished_at, size_bytes: run.size_bytes }
}

/**
 * The dashboard right after a run of the plan was queued: the plan is running, which is what the
 * API answers too until the run ends. The card turns at once, and the dashboard polls fast
 * while a plan runs, so the next answer shows how it ended.
 */
export function withRunQueued(dashboard: Dashboard, planId: number, run: Run): Dashboard {
  const plan = dashboard.plans.find((p) => p.id === planId)
  if (!plan) return dashboard
  const summary = { ...dashboard.summary, [plan.health]: dashboard.summary[plan.health] - 1 }
  summary.running += 1
  return {
    summary,
    plans: dashboard.plans.map((p) => (p.id === planId ? { ...p, health: 'running', last_run: lastRunOf(run) } : p)),
  }
}
