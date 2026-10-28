import type { ColorToken } from './health'
import type { Dashboard, Health, RunStatus } from './types'

/** Tag colours: grey while waiting, blue while it runs, then green or red. */
export const RUN_STATUS_COLOR: Record<RunStatus, string> = {
  queued: 'default',
  running: 'processing',
  succeeded: 'success',
  failed: 'error',
}

/** Bar colours in the runs chart, the same meaning as the tags. */
export const RUN_STATUS_TOKEN: Record<RunStatus, ColorToken> = {
  queued: 'colorTextQuaternary',
  running: 'colorPrimary',
  succeeded: 'colorSuccess',
  failed: 'colorError',
}

/** How often a page asks again while a run it shows is queued or running. */
export const RUN_POLL_MS = 3000

/** A run's log is followed closer than its status. */
export const LOG_POLL_MS = 2000

/** The dashboard polls fast while a plan runs, and slowly otherwise, for scheduled runs. */
export const DASHBOARD_POLL_MS = { active: 5000, idle: 30_000 } as const

export function isRunActive(run: { status: RunStatus } | null | undefined): boolean {
  return run?.status === 'queued' || run?.status === 'running'
}

/** Whether any run on the page is in progress, which is when the page polls. */
export function anyRunActive(runs: readonly { status: RunStatus }[] | undefined): boolean {
  return (runs ?? []).some(isRunActive)
}

/** A plan with a backup or a restore in progress, which is when its pages poll fast. */
export function isPlanBusy(plan: { health: Health; last_run?: { status: RunStatus } | null } | undefined): boolean {
  return plan?.health === 'restoring' || isRunActive(plan?.last_run)
}

export function dashboardPollMs(dashboard: Dashboard | undefined): number {
  return dashboard?.plans.some((plan) => plan.health === 'running' || plan.health === 'restoring') ? DASHBOARD_POLL_MS.active : DASHBOARD_POLL_MS.idle
}
